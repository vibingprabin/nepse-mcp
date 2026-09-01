package news

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Merolagani category ids (from the NewsList dropdown; 0 = All).
var mlCategories = map[string]string{
	"latest":              "0",
	"budget-2077-78":      "22",
	"budget-2078-79":      "24",
	"budget-2081-82":      "30",
	"budget-2082-83":      "32",
	"budget-2083-84":      "34",
	"budget-fy-2076-077":  "11",
	"company-news":        "7",
	"corporate":           "17",
	"covid-19-updates":    "19",
	"current-affairs":     "25",
	"economy":             "8",
	"election":            "33",
	"federal-economy":     "16",
	"from-other-sources":  "5",
	"hydropower":          "9",
	"insurance":           "13",
	"international":       "12",
	"interview":           "31",
	"it-auto":             "23",
	"local-election-2079": "29",
	"market-analysis":     "14",
	"monetary-policy":     "18",
	"opinion":             "10",
	"others":              "20",
	"stock-market":        "6",
	"technical":           "15",
	"tourism":             "28",
	"video":               "26",
	"video-category":      "21",
}

// mlListAPI is Merolagani's JSON news list endpoint (the "Load More" button
// on NewsList.aspx calls exactly this). Format verbs in order: category id,
// symbol (company filter), keyword (news= free-text), page, pageSize.
const mlListAPI = "https://merolagani.com/handlers/webrequesthandler.ashx?type=get_news&newsID=0&newsCategoryID=%s&symbol=%s&page=%d&pageSize=%d&popular=false&includeFeatured=true&news=%s&languageType=NP"

type mlNewsRecord struct {
	NewsID      int    `json:"newsID"`
	NewsTitle   string `json:"newsTitle"`
	NewsDateAD  string `json:"newsDateAD"`
	ImagePath   string `json:"imagePath"`
	ViewCount   int    `json:"viewCount"`
	NewsOverview string `json:"newsOverview"`
}

// GetNewsML fetches the Merolagani list for a category via the JSON API.
// symbol filters to one company (exact ticker); query is a free-text
// keyword search (news= param, matches headline text). Both are optional and
// AND-combined. Each page is 8 records and a single round-trip (~300ms cold).
func (c *Client) GetNewsML(category, symbol, query string, limit, page int) ([]NewsItem, error) {
	id, ok := mlCategories[category]
	if !ok {
		return nil, fmt.Errorf("Merolagani category %q not supported — use one of: %s", category, strings.Join(sortedKeys(mlCategories), ", "))
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 30 {
		limit = 30
	}
	pageSize := min(limit, 8)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	query = strings.TrimSpace(query)

	rawURL := fmt.Sprintf(mlListAPI, id, url.QueryEscape(symbol), page, pageSize, url.QueryEscape(query))
	cacheKey := "ml|" + id + "|" + symbol + "|" + query + "|" + strconv.Itoa(page) + "|" + strconv.Itoa(pageSize)
	if items, _, ok := c.getListCached(cacheKey, listCacheTTL); ok {
		return items, nil
	}

	body, err := c.fetchRaw(rawURL)
	if err != nil {
		return nil, err
	}

	var records []mlNewsRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("parsing Merolagani news JSON: %w", err)
	}

	items := make([]NewsItem, 0, len(records))
	for _, r := range records {
		if r.NewsTitle == "" {
			continue
		}
		items = append(items, NewsItem{
			Source:   SourceMerolagani,
			Category: category,
			Title:    collapseText(r.NewsTitle),
			URL:      fmt.Sprintf("https://merolagani.com/NewsDetail.aspx?newsID=%d", r.NewsID),
			Date:     formatMLDate(r.NewsDateAD),
			Language: detectLanguage(r.NewsTitle),
		})
	}
	c.putListCache(cacheKey, items, "")
	return items, nil
}

// formatMLDate converts "2026-07-31T21:32:13.473" into a display date
// "Jul 31, 2026 09:32 PM" (also parseable by parseDateWeight).
func formatMLDate(iso string) string {
	t, err := time.Parse("2006-01-02T15:04:05.999", iso)
	if err != nil {
		return iso
	}
	return t.Format("Jan 2, 2006 03:04 PM")
}

func detectLanguage(s string) string {
	if devanagariRe.MatchString(s) {
		return "ne"
	}
	return "en"
}

// GetArticleML fetches and parses a Merolagani NewsDetail page. Accepts a
// full URL or a bare /NewsDetail.aspx?newsID=N path.
func (c *Client) GetArticleML(slugURL string) (*Article, error) {
	if !strings.HasPrefix(slugURL, "http") {
		slugURL = "https://merolagani.com" + slugURL
	}

	cacheKey := "mlart|" + slugURL
	if art, ok := c.getArticleCached(cacheKey); ok {
		return art, nil
	}

	body, err := c.fetchRaw(slugURL)
	if err != nil {
		return nil, err
	}

	art := parseMLArticle(body, slugURL)
	c.putArticleCache(cacheKey, art)
	return art, nil
}

var (
	mlTitleRe    = regexp.MustCompile(`(?is)<h4 id="ctl00_ContentPlaceHolder1_newsTitle"[^>]*>(.*?)</h4>`)
	mlDateRe     = regexp.MustCompile(`(?is)<span id="ctl00_ContentPlaceHolder1_newsDate"[^>]*>(.*?)</span>`)
	mlOverviewRe = regexp.MustCompile(`(?is)<div id="ctl00_ContentPlaceHolder1_newsOverview"[^>]*>(.*?)</div>`)
	mlDetailRe   = regexp.MustCompile(`(?is)<div id="ctl00_ContentPlaceHolder1_newsDetail"[^>]*>(.*?)</div>`)
	// mlAdRe strips the inline advertisement blocks inside article bodies.
	mlAdRe = regexp.MustCompile(`(?is)<div class=['"]news-inner-ads['"].*?</div>\s*</div>`)
)

func parseMLArticle(body []byte, rawURL string) *Article {
	art := &Article{Source: SourceMerolagani, URL: rawURL}

	if m := mlTitleRe.FindSubmatch(body); len(m) > 1 {
		art.Title = collapseText(string(m[1]))
	}
	if m := mlDateRe.FindSubmatch(body); len(m) > 1 {
		art.Date = collapseText(string(m[1]))
	}
	art.Language = detectLanguage(art.Title)

	overview := ""
	if m := mlOverviewRe.FindSubmatch(body); len(m) > 1 {
		overview = string(m[1])
	}
	detail := ""
	if m := mlDetailRe.FindSubmatch(body); len(m) > 1 {
		detail = mlAdRe.ReplaceAllString(string(m[1]), "")
	}
	combined := overview + "\n" + detail
	art.ImageURLs = extractImages(combined)
	art.Body = blockToText(imgTagRe.ReplaceAllString(combined, " "))
	return art
}
