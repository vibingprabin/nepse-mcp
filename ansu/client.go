// Package ansu provides a client for the ansuinvest.com JSON API
// (backend.ansuinvest.com). No authentication is required; the backend serves
// open CORS JSON to the Angular SPA at ansuinvest.com.
package ansu

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	baseAPI      = "https://backend.ansuinvest.com/api/web/v1"
	imageBaseURL = "https://backend.ansuinvest.com/public/images"
	userAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

	// Caches are cheap to hold: the screener and sector list change at most
	// daily, so a long TTL avoids hammering the backend on repeated calls.
	// Articles are immutable after publishing, so a slug cache makes repeat
	// reads of the same report ~0ms instead of 0.5-1.5s.
	valuationCacheTTL = 6 * time.Hour
	sectorCacheTTL    = 24 * time.Hour
	companyCacheTTL   = 24 * time.Hour
	articleCacheTTL   = 6 * time.Hour
)

// Client is a thin HTTP client for the ansuinvest.com backend.
type Client struct {
	HTTPClient *http.Client

	// manual TTL caches (same pattern as broker/client.go)
	valuationCache   map[string]cachedValuations
	valuationCacheAt time.Time
	detailCache      map[string]cachedValuationDetail
	detailCacheAt    time.Time
	sectorCache      []Sector
	sectorCacheAt    time.Time
	companyCache     map[string]cachedCompany
	companyCacheAt   time.Time
	articleCache     map[string]cachedArticle
	articleCacheAt   time.Time
}

type cachedValuations struct {
	items []ValuationItem
	at    time.Time
}

type cachedValuationDetail struct {
	item *ValuationDetail
	at   time.Time
}

type cachedCompany struct {
	item ValuationItem
	at   time.Time
}

type cachedArticle struct {
	detail  *ResearchDetail
	related []Article
	at      time.Time
}

// NewClient creates an ansuinvest.com API client.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		valuationCache: make(map[string]cachedValuations),
		detailCache:    make(map[string]cachedValuationDetail),
		companyCache:   make(map[string]cachedCompany),
		articleCache:   make(map[string]cachedArticle),
	}
}

// ---------------------------------------------------------------------------
// Response envelope (the backend wraps everything in {success, statusCode,
// statusMessage, message, data, count}).
// ---------------------------------------------------------------------------

type apiEnvelope struct {
	Success string          `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshalling request: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseAPI+path, reader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://ansuinvest.com")
	req.Header.Set("Referer", "https://ansuinvest.com/")
	req.Header.Set("Authorization", "Bearer null")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", path, resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("parsing envelope from %s: %w", path, err)
	}
	if env.Success != "true" {
		return fmt.Errorf("%s: api error: %s", path, env.Message)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("parsing data from %s: %w", path, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Valuation
// ---------------------------------------------------------------------------

// ValuationDetail is the single-stock valuation verdict from
// /company/valuation-details.
type ValuationDetail struct {
	Sector            string    `json:"sector"`
	SharesOutstanding *float64  `json:"shares_outstanding"`
	MarketPrice       float64   `json:"market_price"`
	PerChange         float64   `json:"per_change"`
	EPS               FlexFloat `json:"eps"`
	PERatio           FlexFloat `json:"pe_ratio"`
	BookValue         FlexFloat `json:"book_value"`
	PBV               FlexFloat `json:"pbv"`
	Valuation         string    `json:"valuation"`
	IntrinsicValue    float64   `json:"intrinsic_value"`
	IntrinsicDate     string    `json:"intrisic_date"` // typo is in the API
}

// FlexFloat tolerates Ansu's mixed payloads: book_value/pbv/eps/pe_ratio arrive
// as JSON numbers for most symbols but as the string "-" for others (e.g. NABIL,
// JOSHI). Valid=false means the source sent a non-numeric placeholder; renderers
// should show "—" rather than a fake 0.00.
type FlexFloat struct {
	Val   float64
	Valid bool
}

func (f *FlexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		f.Val, f.Valid = 0, false
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		str = strings.ReplaceAll(strings.TrimSpace(str), ",", "")
		if str == "" || str == "-" {
			f.Val, f.Valid = 0, false
			return nil
		}
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			f.Val, f.Valid = 0, false
			return nil
		}
		f.Val, f.Valid = v, true
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		f.Val, f.Valid = 0, false
		return nil
	}
	f.Val, f.Valid = v, true
	return nil
}

// ValuationItem is one row of the /company/valuation screener.
type ValuationItem struct {
	Valuation      string  `json:"valuation"`
	CompanyID      int     `json:"company_id"`
	CompanyName    string  `json:"company_name"`
	CompanySymbol  string  `json:"company_sort_code"`
	IntrinsicValue float64 `json:"intrinsic_value"`
	IntrinsicDate  string  `json:"intrisic_date"` // typo is in the API
	SectorName     string  `json:"sector_name"`
	LTP            float64 `json:"ltp"`
	ValuationPer   float64 `json:"valuation_per"` // % LTP vs intrinsic value (negative = below intrinsic)
	Blurred        bool    `json:"blurred"`
}

type valuationListRequest struct {
	Page      int    `json:"page"`
	Limit     int    `json:"limit"`
	Sort      string `json:"sort"`
	SortField string `json:"sort_field"`
	Status    string `json:"status"`
	Fields    []any  `json:"fields"`
	SectorID  int    `json:"sector_id,omitempty"`
}

// GetValuation fetches the Ansu Invest valuation verdict for one symbol.
func (c *Client) GetValuation(symbol string) (*ValuationDetail, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if cv, ok := c.detailCache[symbol]; ok && time.Since(cv.at) < valuationCacheTTL {
		return cv.item, nil
	}
	path := "/company/valuation-details?company_short_code=" + url.QueryEscape(symbol)
	var d ValuationDetail
	if err := c.doJSON(http.MethodGet, path, nil, &d); err != nil {
		if isEmptyArrayErr(err) {
			return nil, fmt.Errorf("%s: symbol not found in Ansu Invest valuation (check symbol name)", symbol)
		}
		return nil, err
	}
	c.detailCache[symbol] = cachedValuationDetail{item: &d, at: time.Now()}
	return &d, nil
}

// isEmptyArrayErr reports whether err came from unmarshalling an empty JSON
// array (the backend's "no record found" shape for unknown symbols).
func isEmptyArrayErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "cannot unmarshal array")
}

// GetSectors returns the sector list (cached 24h) for screener filtering.
func (c *Client) GetSectors() ([]Sector, error) {
	if c.sectorCache != nil && time.Since(c.sectorCacheAt) < sectorCacheTTL {
		return c.sectorCache, nil
	}
	var items []Sector
	err := c.doJSON(http.MethodPost, "/company/list-company-sector", map[string]any{"lang": "eng"}, &items)
	if err != nil {
		return nil, err
	}
	c.sectorCache, c.sectorCacheAt = items, time.Now()
	return items, nil
}

// ListValuations returns the valuation screener (cached 6h), optionally
// filtered to one sector. sectorID 0 = all sectors.
func (c *Client) ListValuations(sectorID, limit int) ([]ValuationItem, error) {
	key := fmt.Sprintf("%d|%d", sectorID, limit)
	if c.valuationCache != nil {
		if cv, ok := c.valuationCache[key]; ok && time.Since(cv.at) < valuationCacheTTL {
			return cv.items, nil
		}
	}
	if limit <= 0 || limit > 10000 {
		limit = 10000 // backend caps around here; filters apply client-side
	}
	req := valuationListRequest{
		Page:      1,
		Limit:     limit,
		Sort:      "DESC",
		SortField: "valuation_date",
		Status:    "active",
		Fields:    []any{map[string]string{"field": "", "operator": "", "value": ""}},
	}
	if sectorID > 0 {
		req.SectorID = sectorID
	}
	var items []ValuationItem
	if err := c.doJSON(http.MethodPost, "/company/valuation", req, &items); err != nil {
		return nil, err
	}
	if c.valuationCache == nil {
		c.valuationCache = make(map[string]cachedValuations)
	}
	c.valuationCache[key] = cachedValuations{items: items, at: time.Now()}
	return items, nil
}

// FindCompany looks up a company by symbol in the screener (cached).
// Used to resolve company_id for per-company research feeds.
func (c *Client) FindCompany(symbol string) (*ValuationItem, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if cc, ok := c.companyCache[symbol]; ok && time.Since(cc.at) < companyCacheTTL {
		return &cc.item, nil
	}
	items, err := c.ListValuations(0, 10000)
	if err != nil {
		return nil, fmt.Errorf("resolving %s in screener: %w", symbol, err)
	}
	for i := range items {
		if strings.EqualFold(items[i].CompanySymbol, symbol) {
			c.companyCache[symbol] = cachedCompany{item: items[i], at: time.Now()}
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("symbol %s not found in Ansu Invest screener", symbol)
}

// ---------------------------------------------------------------------------
// Research / articles
// ---------------------------------------------------------------------------

// Article is one research-opinion article (list shape).
type Article struct {
	ExpertID    int    `json:"expert_id"`
	Title       string `json:"title"`
	SubTitle    string `json:"sub_title"`
	Slug        string `json:"slug"`
	Summary     string `json:"summary"` // HTML
	Image       string `json:"image"`   // hero image filename
	ImageSource string `json:"image_source"`
	PostedAt    string `json:"posted_at"`
	IsPremium   int    `json:"is_premium"`
}

// ResearchDetail is the full article (view shape: same fields + description).
type ResearchDetail struct {
	Article
	Disclaimer  string `json:"disclaimer"`
	Description string `json:"description"` // full HTML; may embed base64 chart images
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

type researchViewData struct {
	Model   ResearchDetail `json:"model"`
	Related []Article      `json:"except_model"`
}

type researchListRequest struct {
	Lang      string `json:"lang"`
	Page      int    `json:"page"`
	Limit     int    `json:"limit"`
	Sort      string `json:"sort"`
	SortField string `json:"sort_field"`
	Status    string `json:"status"`
	Fields    []any  `json:"fields"`
}

// ListArticles returns the research-opinion feed. If symbol is non-empty the
// feed is filtered to that company (resolved via the screener's company_id).
func (c *Client) ListArticles(symbol string, page, limit int) ([]Article, error) {
	if limit <= 0 || limit > 100 {
		limit = 15
	}
	if page <= 0 {
		page = 1
	}

	base := researchListRequest{
		Lang:      "eng",
		Page:      page,
		Limit:     limit,
		Sort:      "DESC",
		SortField: "ordering",
		Status:    "active",
		Fields:    []any{map[string]string{"field": "", "operator": "", "value": ""}},
	}

	path := "/research/list-expert-research"
	body := any(base)
	if symbol != "" {
		co, err := c.FindCompany(symbol)
		if err != nil {
			return nil, err
		}
		path = "/research/company-expert-research"
		body = map[string]any{
			"company_id": co.CompanyID,
			"page":       fmt.Sprintf("%d", page),
			"limit":      fmt.Sprintf("%d", limit),
			"sort":       "DESC",
			"sort_field": "ordering",
			"status":     "active",
			"fields":     []any{map[string]string{"field": "string", "operator": "contains or matches", "value": "string"}},
		}
	}

	var items []Article
	if err := c.doJSON(http.MethodPost, path, body, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// GetArticle fetches the full article body by slug, plus related articles.
// Results are cached per slug (6h); articles are immutable after publishing.
func (c *Client) GetArticle(slug string) (*ResearchDetail, []Article, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, nil, fmt.Errorf("slug is required")
	}
	if ca, ok := c.articleCache[slug]; ok && time.Since(ca.at) < articleCacheTTL {
		return ca.detail, ca.related, nil
	}
	var d researchViewData
	if err := c.doJSON(http.MethodPost, "/research/view", map[string]any{"slug": slug, "limit": 5}, &d); err != nil {
		if isEmptyArrayErr(err) {
			return nil, nil, fmt.Errorf("article %q not found in Ansu Invest research (check the slug)", slug)
		}
		return nil, nil, err
	}
	c.articleCache[slug] = cachedArticle{detail: &d.Model, related: d.Related, at: time.Now()}
	return &d.Model, d.Related, nil
}

// HeroImageURL resolves an article's hero image filename to a full URL.
func (c *Client) HeroImageURL(filename string) string {
	if filename == "" {
		return ""
	}
	return imageBaseURL + "/" + url.PathEscape(filename)
}

// FetchImageBytes downloads an image and returns its base64 payload + MIME.
func (c *Client) FetchImageBytes(u string) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", "", fmt.Errorf("creating image request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetching image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("image %s returned status %d", u, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("reading image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), mimeFromURL(u), nil
}

// ---------------------------------------------------------------------------
// HTML helpers
// ---------------------------------------------------------------------------

// stripTagsRe removes any remaining tag-like fragments.
var stripTagsRe = regexp.MustCompile(`(?s)<[^>]*>`)

// scriptStyleRe strips <script>/<style> blocks (RE2 has no backreferences,
// hence the two explicit alternates).
var scriptStyleRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)

// inlineImgRe captures inline base64 chart images embedded in description HTML.
var inlineImgRe = regexp.MustCompile(`(?is)<img[^>]*src=["']data:image/([a-z0-9+.-]+);base64,([^"']+)["']`)

// HTMLToText regexes are hoisted to package level: they are compiled once
// instead of on every call (HTMLToText runs 3x per article read).
var (
	blockEndRe   = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|ul|ol|table|blockquote)>`)
	brHrRe       = regexp.MustCompile(`(?i)<(br|hr)[^>]*>`)
	liStartRe    = regexp.MustCompile(`(?i)<li[^>]*>`)
	headingRe    = regexp.MustCompile(`(?i)<h([1-6])[^>]*>`)
	tdEndRe      = regexp.MustCompile(`(?i)</td>`)
	thEndRe      = regexp.MustCompile(`(?i)</th>`)
	trStartRe    = regexp.MustCompile(`(?i)<tr[^>]*>`)
	newlineRunRe = regexp.MustCompile(`\n{3,}`)
)

// HTMLToText converts CMS article HTML into readable plain text.
func HTMLToText(h string) string {
	if h == "" {
		return ""
	}
	s := h
	// drop script/style blocks
	s = scriptStyleRe.ReplaceAllString(s, " ")
	// block elements become line breaks
	s = blockEndRe.ReplaceAllString(s, "\n")
	s = brHrRe.ReplaceAllString(s, "\n")
	// list items get a bullet
	s = liStartRe.ReplaceAllString(s, "\n- ")
	// headings get a marker
	s = headingRe.ReplaceAllString(s, "\n\n### ")
	// tables: separate cells with pipes
	s = tdEndRe.ReplaceAllString(s, " | ")
	s = thEndRe.ReplaceAllString(s, " | ")
	s = trStartRe.ReplaceAllString(s, "\n| ")
	// strip every remaining tag
	s = stripTagsRe.ReplaceAllString(s, " ")
	// decode entities (&nbsp; &amp; &#39; &rsquo; &ldquo; ...)
	s = html.UnescapeString(s)
	// normalise whitespace: collapse 3+ newlines to 2, trim lines
	lines := strings.Split(s, "\n")
	var out []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		out = append(out, t)
	}
	s = strings.Join(out, "\n")
	s = newlineRunRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// StripDisclaimer removes Ansu's standard legal boilerplate from an article's
// rendered text. The block always starts with "Important information:" and runs
// to the end of the article; it carries no analytical content.
func StripDisclaimer(text string) string {
	i := strings.Index(text, "Important information:")
	if i < 0 {
		return text
	}
	return strings.TrimSpace(text[:i])
}

// ExtractInlineImages returns (mimeType, base64Data) pairs for every inline
// chart image embedded in the description HTML. Used by vision-capable models
// via include_images=true.
func ExtractInlineImages(description string) [][2]string {
	var out [][2]string
	seen := map[string]bool{}
	for _, m := range inlineImgRe.FindAllStringSubmatch(description, -1) {
		if len(m) < 3 || m[2] == "" {
			continue
		}
		if seen[m[2]] {
			continue
		}
		seen[m[2]] = true
		out = append(out, [2]string{"image/" + strings.ToLower(m[1]), m[2]})
	}
	return out
}

func mimeFromURL(u string) string {
	lu := strings.ToLower(u)
	switch {
	case strings.Contains(lu, ".png"):
		return "image/png"
	case strings.Contains(lu, ".webp"):
		return "image/webp"
	case strings.Contains(lu, ".gif"):
		return "image/gif"
	case strings.Contains(lu, ".jpg"), strings.Contains(lu, ".jpeg"):
		return "image/jpeg"
	default:
		return "image/jpeg"
	}
}

// FetchedImage is a successfully downloaded image (or the error from it).
type FetchedImage struct {
	URL  string
	Data string
	MIME string
	Err  error
}

// FetchImages downloads URLs concurrently (4 in flight) and returns results
// in input order. A failed download yields a FetchedImage with a non-nil Err
// instead of aborting the batch.
func (c *Client) FetchImages(urls []string) []FetchedImage {
	out := make([]FetchedImage, len(urls))
	if len(urls) == 0 {
		return out
	}
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i].URL = u
			out[i].Data, out[i].MIME, out[i].Err = c.FetchImageBytes(u)
		}(i, u)
	}
	wg.Wait()
	return out
}

// Sector is one entry of the /company/list-company-sector response.
type Sector struct {
	SectorID   int    `json:"sector_id"`
	SectorName string `json:"sector_name"`
	NepseCode  string `json:"nepse_code"`
	Status     string `json:"status"`
}
