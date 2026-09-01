package news

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StockSessions (stockssessions.com) is a Next.js aggregator whose browser
// bundle calls a JSON API at api.stockssessions.com. The API requires the
// same public x-api-key the site's own JS ships, plus a referer. Two feeds:
//
//   - news: aggregated headlines (mostly Nepali titles) with links to the
//     original articles (beemapost, arthasarokar, bizmandu, nepalipaisa).
//   - pulse: company disclosures with full English analysis inline
//     (ProseMirror JSON: paragraphs + financial tables) — quarterly reports,
//     rights/bonus actions, etc. This is the highest-value feed: it is the
//     company-filings source, filterable by company name.
//
// Rate limit: 60 requests/min (x-ratelimit headers). Lists cache 10 min.
const (
	sssAPIBase  = "https://api.stockssessions.com/api/v1"
	sssAPIKey   = "RCTxTU2o48q0aQ9AqcaFDdI620xvuyHhRm9k8ffH6Js"
	sssReferer  = "https://www.stockssessions.com/"
	sssListTTL  = 10 * time.Minute
	sssMaxLimit = 50
)

// ssNewsItem is one aggregated headline from /news/.
type ssNewsItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	ImageURL  string `json:"image_url"`
	FetchedAt string `json:"fetched_at"`
	SourceID  int    `json:"news_source_id"`
	CatID     int    `json:"news_category_id"`
}

// ssPulseItem is one company disclosure from /pulse/. Content is ProseMirror
// JSON (paragraphs + tables); the list carries the FULL body, no detail call.
type ssPulseItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Content     string `json:"content"`
	CompanyName string `json:"company_name"`
	CreatedAt   string `json:"created_at"`
	DocType     struct {
		Name string `json:"name"`
	} `json:"document_type"`
	Sector struct {
		Name string `json:"name"`
	} `json:"sector"`
}

type ssResponse struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// StocksSessionsClient talks to the stockssessions JSON API.
type StocksSessionsClient struct {
	http *http.Client
}

func newStocksSessionsClient(hc *http.Client) *StocksSessionsClient {
	return &StocksSessionsClient{http: hc}
}

func (s *StocksSessionsClient) getJSON(path string, params url.Values, out *ssResponse) error {
	u := sssAPIBase + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("x-api-key", sssAPIKey)
	req.Header.Set("referer", sssReferer)
	req.Header.Set("origin", "https://www.stockssessions.com")
	req.Header.Set("accept", "application/json")
	req.Header.Set("user-agent", userAgent)

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d", u, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", u, err)
	}
	if out.Status != 200 {
		return fmt.Errorf("%s: API error %d: %s", u, out.Status, out.Message)
	}
	return nil
}

// GetNewsSSS returns aggregated headlines (Nepali titles, external links).
// The API has no company filter; titles are matched client-side when company
// is non-empty.
func (c *Client) GetNewsSSS(company string, limit, page int) ([]NewsItem, error) {
	if limit <= 0 || limit > sssMaxLimit {
		limit = sssMaxLimit
	}
	if page < 1 {
		page = 1
	}
	key := fmt.Sprintf("sss:news:%d:%d:%s", limit, page, strings.ToUpper(company))
	if items, _, ok := c.getListCached(key, sssListTTL); ok {
		return items, nil
	}

	params := url.Values{}
	params.Set("skip", fmt.Sprint((page-1)*limit))
	params.Set("limit", fmt.Sprint(limit))
	var res ssResponse
	if err := c.ss.getJSON("/news/", params, &res); err != nil {
		return nil, err
	}
	var raw []ssNewsItem
	if err := json.Unmarshal(res.Data, &raw); err != nil {
		return nil, fmt.Errorf("parsing news list: %w", err)
	}

	cats := c.ssCategories()
	company = strings.ToUpper(strings.TrimSpace(company))
	items := make([]NewsItem, 0, len(raw))
	for _, n := range raw {
		if company != "" && !strings.Contains(strings.ToUpper(n.Title), company) {
			continue
		}
		items = append(items, NewsItem{
			Source:   SourceStocksSessions,
			Category: cats[n.CatID],
			Title:    strings.TrimSpace(n.Title),
			URL:      n.Link,
			Date:     n.FetchedAt,
			Language: detectLanguage(n.Title),
		})
	}
	c.putListCache(key, items, "")
	return items, nil
}

// GetPulse returns company disclosures (Q4 reports, rights/bonus actions)
// with English analysis. Filtered by company name substring when set.
func (c *Client) GetPulse(company string, limit int) ([]NewsItem, error) {
	if limit <= 0 || limit > sssMaxLimit {
		limit = sssMaxLimit
	}
	key := fmt.Sprintf("sss:pulse:%d:%s", limit, strings.ToUpper(company))
	if items, _, ok := c.getListCached(key, sssListTTL); ok {
		return items, nil
	}

	params := url.Values{}
	params.Set("limit", fmt.Sprint(limit))
	var res ssResponse
	if err := c.ss.getJSON("/pulse/", params, &res); err != nil {
		return nil, err
	}
	var raw []ssPulseItem
	if err := json.Unmarshal(res.Data, &raw); err != nil {
		return nil, fmt.Errorf("parsing pulse list: %w", err)
	}

	company = strings.ToUpper(strings.TrimSpace(company))
	items := make([]NewsItem, 0, len(raw))
	for _, p := range raw {
		if company != "" && !strings.Contains(strings.ToUpper(p.CompanyName), company) {
			continue
		}
		cat := p.DocType.Name
		if p.Sector.Name != "" {
			cat = p.DocType.Name + " · " + p.Sector.Name
		}
		items = append(items, NewsItem{
			Source:   SourcePulse,
			Category: cat,
			Title:    strings.TrimSpace(p.Title),
			URL:      "https://www.stockssessions.com/pulse/" + p.Slug,
			Date:     p.CreatedAt,
			Language: "EN",
			Preview:  strings.TrimSpace(p.Description),
		})
	}
	c.putListCache(key, items, "")
	return items, nil
}

// GetArticleSSS resolves a stockssessions URL. Pulse slugs return the rich
// ProseMirror body (paragraphs + financial tables) extracted to text; any
// other URL (aggregated external article) is fetched and stripped generically.
func (c *Client) GetArticleSSS(articleURL string) (*Article, error) {
	if i := strings.Index(articleURL, "/pulse/"); i >= 0 {
		slug := strings.Trim(strings.TrimSpace(articleURL[i+len("/pulse/"):]), "/")
		items, err := c.GetPulse("", sssMaxLimit)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if strings.HasSuffix(it.URL, "/"+slug) {
				pulse, err := c.pulseBySlug(slug)
				if err != nil {
					return nil, err
				}
				return &Article{
					Source:   SourcePulse,
					Title:    it.Title,
					Date:     it.Date,
					URL:      it.URL,
					Language: "EN",
					Body:     extractProseMirror(pulse.Content),
				}, nil
			}
		}
		return nil, fmt.Errorf("pulse item %q not found", slug)
	}

	body, err := c.fetchRaw(articleURL)
	if err != nil {
		return nil, err
	}
	return &Article{
		Source:   SourceStocksSessions,
		Title:    "",
		Date:     "",
		URL:      articleURL,
		Language: detectLanguage(string(body)),
		Body:     collapseText(string(body)),
	}, nil
}

func (c *Client) pulseBySlug(slug string) (*ssPulseItem, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprint(sssMaxLimit))
	var res ssResponse
	if err := c.ss.getJSON("/pulse/", params, &res); err != nil {
		return nil, err
	}
	var raw []ssPulseItem
	if err := json.Unmarshal(res.Data, &raw); err != nil {
		return nil, fmt.Errorf("parsing pulse list: %w", err)
	}
	for _, p := range raw {
		if p.Slug == slug {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("pulse item %q not found", slug)
}

// ssCategories lazily loads the news-category id→name map (cached in-process).
func (c *Client) ssCategories() map[int]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ssCatMap != nil {
		return c.ssCatMap
	}
	m := map[int]string{}
	params := url.Values{}
	var res ssResponse
	if err := c.ss.getJSON("/news-category/", params, &res); err != nil {
		m[0] = ""
		c.ssCatMap = m
		return m
	}
	var raw []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(res.Data, &raw); err == nil {
		for _, c := range raw {
			m[c.ID] = c.Name
		}
	}
	c.ssCatMap = m
	return m
}

// extractProseMirror flattens a ProseMirror JSON document to readable text:
// paragraphs on their own lines, table rows as pipe-joined cells.
func extractProseMirror(raw string) string {
	var doc struct {
		Content []pmNode `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return strings.TrimSpace(raw)
	}
	var sb strings.Builder
	walkPM(doc.Content, &sb, 0)
	return strings.TrimSpace(sb.String())
}

type pmNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []pmNode  `json:"content"`
	Attrs   pmAttrs   `json:"attrs"`
	Marks   []pmMark  `json:"marks"`
}

type pmAttrs struct {
	TextAlign string `json:"textAlign"`
	Level     int    `json:"level"`
}

type pmMark struct {
	Type string `json:"type"`
}

// walkPM writes node text; table rows become single lines of pipe-joined
// cells so the model sees the numbers in one glance.
func walkPM(nodes []pmNode, sb *strings.Builder, depth int) {
	for _, n := range nodes {
		switch n.Type {
		case "text":
			sb.WriteString(n.Text)
		case "paragraph", "heading":
			if depth > 0 {
				sb.WriteString("\n")
			}
			walkPM(n.Content, sb, depth+1)
			sb.WriteString("\n")
		case "tableRow":
			var cells []string
			for _, cell := range n.Content {
				var cs strings.Builder
				walkPM(cell.Content, &cs, 0)
				cells = append(cells, strings.TrimSpace(cs.String()))
			}
			sb.WriteString(strings.Join(cells, " | ") + "\n")
		default:
			walkPM(n.Content, sb, depth+1)
		}
	}
}
