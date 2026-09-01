package news

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ShareSansar canonical category names → list paths. Announcements use a
// dedicated page with a different card class (featured-announcement-title).
var ssCategories = map[string]string{
	"latest":           "/category/latest",
	"announcement":     "/announcement",
	"exclusive":        "/category/exclusive",
	"company-analysis": "/category/financial-analysis",
	"ipo-fpo":          "/category/ipo-fpo-news",
	"dividend":         "/category/proposed-dividend",
	"allotment":        "/category/share-allotment",
	"listing":          "/category/share-listed",
	"technical":        "/category/technical-analysis",
	"weekly":           "/category/weekly-analysis",
	"video":            "/category/video-tutorials",
}

// ssCardRe captures one headline card: an <a> whose href ends in
// newsdetail/ or announcementdetail/, its title attribute, the <h4> title
// text, and the date in the following span.text-org.
var ssCardRe = regexp.MustCompile(`(?is)<a href="(https?://[^"]*?(?:newsdetail|announcementdetail)/[^"]+)"[^>]*title="([^"]*)"[^>]*>\s*<h4 class="featured-[^"]*-title">(.*?)</h4>\s*</a>\s*<p>\s*<span class="text-org">(.*?)</span>`)

// ssCursorRe captures the ?cursor=... link marked rel="next".
var ssCursorRe = regexp.MustCompile(`(?i)href="\?cursor=([^"]+)"[^>]*rel="next"`)

// GetNewsSS fetches the ShareSansar list for a category. Pagination is
// cursor-based and sequential: page N needs the cursor from page N-1, so it
// costs N round-trips. Page 1 is the common case (~2s cold, cached 10min).
// company (optional) is a server-side symbol filter — the list pages accept
// ?company=X and return only that company's headlines.
func (c *Client) GetNewsSS(category, company string, limit, page int) ([]NewsItem, error) {
	path, ok := ssCategories[category]
	if !ok {
		return nil, fmt.Errorf("ShareSansar category %q not supported — use one of: %s", category, strings.Join(sortedKeys(ssCategories), ", "))
	}
	if page < 1 {
		page = 1
	}
	company = strings.ToUpper(strings.TrimSpace(company))

	cursor := ""
	var items []NewsItem
	for p := 1; p <= page; p++ {
		rawURL := ssListURL + path
		params := url.Values{}
		if company != "" {
			params.Set("company", company)
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		if enc := params.Encode(); enc != "" {
			rawURL += "?" + enc
		}

		cacheKey := "ss|" + path + "|" + company + "|" + cursor
		pageItems, nextCursor, cached := c.getListCached(cacheKey, listCacheTTL)
		if !cached {
			body, err := c.fetchRaw(rawURL)
			if err != nil {
				return nil, err
			}
			pageItems = parseSSCards(body)
			nextCursor = nextSSCursor(body)
			c.putListCache(cacheKey, pageItems, nextCursor)
		}

		if p == page {
			items = pageItems
			break
		}

		cursor = nextCursor
		if cursor == "" {
			return nil, fmt.Errorf("ShareSansar page %d has no next-cursor", p)
		}
		if len(pageItems) == 0 {
			break
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// nextSSCursor extracts the ?cursor=... link marked rel="next".
func nextSSCursor(body []byte) string {
	m := ssCursorRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func parseSSCards(body []byte) []NewsItem {
	matches := ssCardRe.FindAllSubmatch(body, -1)
	items := make([]NewsItem, 0, len(matches))
	for _, m := range matches {
		if len(m) < 5 {
			continue
		}
		title := collapseText(string(m[3]))
		if title == "" {
			title = collapseText(string(m[2]))
		}
		if title == "" {
			continue
		}
		items = append(items, NewsItem{
			Source:   SourceShareSansar,
			Category: detectSSCategory(string(m[1])),
			Title:    title,
			URL:      string(m[1]),
			Date:     collapseText(string(m[4])),
			Language: "en",
		})
	}
	return items
}

func detectSSCategory(rawURL string) string {
	if strings.Contains(rawURL, "/announcementdetail/") {
		return "announcement"
	}
	return "news"
}

// GetArticleSS fetches and parses a ShareSansar detail page. Accepts a full
// URL or a bare path (/newsdetail/slug).
func (c *Client) GetArticleSS(slugURL string) (*Article, error) {
	if !strings.HasPrefix(slugURL, "http") {
		slugURL = ssListURL + slugURL
	}

	cacheKey := "ssart|" + slugURL
	if art, ok := c.getArticleCached(cacheKey); ok {
		return art, nil
	}

	body, err := c.fetchRaw(slugURL)
	if err != nil {
		return nil, err
	}

	art := parseSSArticle(body, slugURL)
	c.putArticleCache(cacheKey, art)
	return art, nil
}

var (
	ssTitleRe   = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	ssDateRe    = regexp.MustCompile(`(?is)<h5><i[^>]*></i>\s*(.*?)</i>`)
	ssContentRe = regexp.MustCompile(`(?is)<div id="newsdetail-content">(.*?)</div>`)
)

func parseSSArticle(body []byte, rawURL string) *Article {
	art := &Article{Source: SourceShareSansar, URL: rawURL}

	if m := ssTitleRe.FindSubmatch(body); len(m) > 1 {
		art.Title = collapseText(string(m[1]))
	}
	if m := ssDateRe.FindSubmatch(body); len(m) > 1 {
		art.Date = collapseText(string(m[1]))
	}
	if m := ssContentRe.FindSubmatch(body); len(m) > 1 {
		content := string(m[1])
		art.ImageURLs = extractImages(content)
		art.Body = blockToText(imgTagRe.ReplaceAllString(content, " "))
	}
	return art
}

// ---------------------------------------------------------------------------
// Shared rich-HTML → text conversion (mirrors ansu.HTMLToText so both news
// sources render paragraphs, lists, headings and tables readably).
// ---------------------------------------------------------------------------

var (
	blockEndRe   = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|ul|ol|table|blockquote)>`)
	brHrRe       = regexp.MustCompile(`(?i)<(br|hr)[^>]*>`)
	liStartRe    = regexp.MustCompile(`(?i)<li[^>]*>`)
	headingRe    = regexp.MustCompile(`(?i)<h([1-6])[^>]*>`)
	tdEndRe      = regexp.MustCompile(`(?i)</td>`)
	thEndRe      = regexp.MustCompile(`(?i)</th>`)
	trStartRe    = regexp.MustCompile(`(?i)<tr[^>]*>`)
	newlineRunRe = regexp.MustCompile(`\n{3,}`)
	stripTagsRe  = regexp.MustCompile(`(?s)<[^>]+>`)
)

func blockToText(h string) string {
	if h == "" {
		return ""
	}
	s := scriptStyleRe.ReplaceAllString(h, " ")
	s = blockEndRe.ReplaceAllString(s, "\n")
	s = brHrRe.ReplaceAllString(s, "\n")
	s = liStartRe.ReplaceAllString(s, "\n- ")
	s = headingRe.ReplaceAllString(s, "\n\n### ")
	s = tdEndRe.ReplaceAllString(s, " | ")
	s = thEndRe.ReplaceAllString(s, " | ")
	s = trStartRe.ReplaceAllString(s, "\n| ")
	s = stripTagsRe.ReplaceAllString(s, " ")
	s = decodeEntities(s)
	lines := strings.Split(s, "\n")
	var out []string
	for _, ln := range lines {
		out = append(out, strings.TrimSpace(ln))
	}
	s = newlineRunRe.ReplaceAllString(strings.Join(out, "\n"), "\n\n")
	return strings.TrimSpace(s)
}

var scriptStyleRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
