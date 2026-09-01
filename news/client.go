// Package news provides clients for the two NEPSE news sources that serve
// server-rendered HTML without a public JSON API:
//
//   - ShareSansar (sharessansar.com): English-language market news. List pages
//     use cursor-based pagination (?cursor=<base64>); article bodies live in
//     div.detail-content.
//   - Merolagani (merolagani.com): Nepali (Devanagari) news. List pages use
//     plain ?page=N pagination; article bodies live in #newsDetail.
//
// Both sites are scraped with hoisted regexes (no DOM dependency). List pages
// are cached 10 minutes (headlines change through the day), article bodies
// 6 hours (immutable after publication).
package news

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// ShareSansar list page holds 16 articles; Merolagani holds 8.
	ssListURL    = "https://www.sharesansar.com"
	mlListURL    = "https://merolagani.com"
	userAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	listCacheTTL = 10 * time.Minute
	artCacheTTL  = 6 * time.Hour
)

// Source identifiers accepted by GetNews/GetArticle.
const (
	SourceShareSansar     = "sharesansar"
	SourceMerolagani      = "merolagani"
	SourceBoth            = "both"
	SourceStocksSessions  = "stockssessions"
	SourcePulse           = "pulse"
)

// NewsItem is one headline in a news list.
type NewsItem struct {
	Source   string `json:"source"`
	Category string `json:"category"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Date     string `json:"date"`
	Language string `json:"language"`
	Preview  string `json:"preview,omitempty"`
}

// Article is a full news article body.
type Article struct {
	Source    string   `json:"source"`
	Title     string   `json:"title"`
	Date      string   `json:"date"`
	URL       string   `json:"url"`
	Language  string   `json:"language"`
	Body      string   `json:"body"`
	ImageURLs []string `json:"image_urls,omitempty"`
}

// Client fetches and caches news from ShareSansar, Merolagani and
// StockSessions (news + pulse).
type Client struct {
	HTTPClient *http.Client
	ss         *StocksSessionsClient
	ssCatMap   map[int]string

	mu       sync.Mutex
	listCache   map[string]cachedList
	articleCache map[string]cachedArticle
}

type cachedList struct {
	items []NewsItem
	next  string
	at    time.Time
}

type cachedArticle struct {
	art *Article
	at  time.Time
}

// NewClient creates a news client with the same transport tuning as the rest
// of the server (shared idle connections, no per-request dial costs).
func NewClient() *Client {
	c := &Client{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		listCache:      make(map[string]cachedList),
		articleCache:   make(map[string]cachedArticle),
	}
	c.ss = newStocksSessionsClient(c.HTTPClient)
	return c
}

// canonicalSource maps aliases to full names and rejects unknown sources
// with a helpful error.
func canonicalSource(source string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SourceShareSansar, "ss", "sharesansar.com":
		return SourceShareSansar, nil
	case SourceMerolagani, "ml", "merolagani.com":
		return SourceMerolagani, nil
	case SourceStocksSessions, "sts", "sessions", "stockssessions.com":
		return SourceStocksSessions, nil
	case SourcePulse, "pl":
		return SourcePulse, nil
	case SourceBoth, "":
		return SourceBoth, nil
	default:
		return "", fmt.Errorf("unknown source %q — use %q, %q, %q, %q or %q", source, SourceShareSansar, SourceMerolagani, SourceStocksSessions, SourcePulse, SourceBoth)
	}
}

func (c *Client) getListCached(key string, ttl time.Duration) ([]NewsItem, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.listCache[key]
	if !ok || time.Since(e.at) > ttl {
		return nil, "", false
	}
	return e.items, e.next, true
}

func (c *Client) putListCache(key string, items []NewsItem, next string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listCache[key] = cachedList{items: items, next: next, at: time.Now()}
}

func (c *Client) getArticleCached(key string) (*Article, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.articleCache[key]
	if !ok || time.Since(e.at) > artCacheTTL {
		return nil, false
	}
	return e.art, true
}

func (c *Client) putArticleCache(key string, art *Article) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.articleCache[key] = cachedArticle{art: art, at: time.Now()}
}

// ---------------------------------------------------------------------------
// Shared HTML helpers (hoisted regexes, compiled once)
// ---------------------------------------------------------------------------

var (
	// whitespaceRe collapses runs of whitespace into a single space.
	whitespaceRe = regexp.MustCompile(`\s+`)
	// tagRe strips any single HTML tag.
	tagRe = regexp.MustCompile(`(?s)<[^>]+>`)
	// imgSrcRe captures <img src="..."> URLs.
	imgSrcRe = regexp.MustCompile(`(?is)<img[^>]+src=["']([^"']+)["']`)
	// imgTagRe strips an ENTIRE <img ...> tag. imgSrcRe alone is insufficient
	// for body cleanup: it stops at the closing quote of src, orphaning trailing
	// attributes like class='...' /> (which tagRe can no longer catch).
	imgTagRe = regexp.MustCompile(`(?is)<img[^>]*>`)
	// devanagariRe detects Devanagari (Nepali) script.
	devanagariRe = regexp.MustCompile(`[\p{Devanagari}]`)
)

// collapseText normalises extracted raw text: entity-decode, strip tags,
// collapse whitespace.
func collapseText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = decodeEntities(s)
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// extractImages returns every <img src> URL in the HTML fragment.
func extractImages(h string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range imgSrcRe.FindAllStringSubmatch(h, -1) {
		if len(m) < 2 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// fetchRaw GETs a URL with our User-Agent and returns the raw body.
func (c *Client) fetchRaw(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	body, err := readAllLimited(resp.Body, 8<<20)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP %d", rawURL, resp.StatusCode)
	}
	return body, nil
}
