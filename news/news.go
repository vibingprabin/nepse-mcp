package news

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// GetNews returns headlines for the given source(s). source may be
// "sharesansar", "merolagani" or "both" (merged and sorted newest-first).
// Category names are canonicalised across sources where they overlap; each
// source returns a helpful list of its supported categories on mismatch.
// company (optional) filters to one ticker server-side where supported;
// query (optional) is a free-text keyword search (Merolagani news= param).
func (c *Client) GetNews(source, category, company, query string, limit, page int) ([]NewsItem, error) {
	src, err := canonicalSource(source)
	if err != nil {
		return nil, err
	}
	if category == "" {
		category = "latest"
	}
	if limit <= 0 || limit > 30 {
		limit = 30
	}
	if page < 1 {
		page = 1
	}
	company = strings.ToUpper(strings.TrimSpace(company))
	query = strings.TrimSpace(query)

	var wg sync.WaitGroup
	var ssItems, mlItems, stsItems, plItems []NewsItem
	var ssErr, mlErr, stsErr, plErr error

	fetch := func(fn func() ([]NewsItem, error), out *[]NewsItem, outErr *error) {
		wg.Go(func() {
			items, err := fn()
			*out, *outErr = items, err
		})
	}

	switch src {
	case SourceShareSansar:
		fetch(func() ([]NewsItem, error) { return c.GetNewsSS(category, company, limit, page) }, &ssItems, &ssErr)
	case SourceMerolagani:
		fetch(func() ([]NewsItem, error) { return c.GetNewsML(category, company, query, limit, page) }, &mlItems, &mlErr)
	case SourceStocksSessions:
		fetch(func() ([]NewsItem, error) { return c.GetNewsSSS(company, limit, page) }, &stsItems, &stsErr)
	case SourcePulse:
		fetch(func() ([]NewsItem, error) { return c.GetPulse(company, limit) }, &plItems, &plErr)
	default:
		fetch(func() ([]NewsItem, error) { return c.GetNewsSS(category, company, limit, page) }, &ssItems, &ssErr)
		fetch(func() ([]NewsItem, error) { return c.GetNewsML(category, company, query, limit, page) }, &mlItems, &mlErr)
	}
	wg.Wait()

	if ssErr != nil && mlErr != nil && stsErr != nil && plErr != nil {
		return nil, fmt.Errorf("sharesansar: %v; merolagani: %v; stockssessions: %v; pulse: %v", ssErr, mlErr, stsErr, plErr)
	}
	if ssErr != nil && src == SourceShareSansar {
		return nil, ssErr
	}
	if mlErr != nil && src == SourceMerolagani {
		return nil, mlErr
	}
	if stsErr != nil && src == SourceStocksSessions {
		return nil, stsErr
	}
	if plErr != nil && src == SourcePulse {
		return nil, plErr
	}

	merged := append(append(append(append([]NewsItem{}, ssItems...), mlItems...), stsItems...), plItems...)
	merged = dedupeByTitle(merged)
	sortNewsByDate(merged)
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// GetArticle fetches one article. source is required; url may be a full URL
// or the bare path returned by GetNews (e.g. /newsdetail/slug).
func (c *Client) GetArticle(source, articleURL string) (*Article, error) {
	src, err := canonicalSource(source)
	if err != nil {
		return nil, err
	}
	if src == SourceBoth {
		// disambiguate from the URL shape
		if strings.Contains(articleURL, "NewsDetail.aspx") {
			src = SourceMerolagani
		} else {
			src = SourceShareSansar
		}
	}
	switch src {
	case SourceShareSansar:
		return c.GetArticleSS(articleURL)
	case SourceMerolagani:
		return c.GetArticleML(articleURL)
	case SourceStocksSessions, SourcePulse:
		return c.GetArticleSSS(articleURL)
	default:
		return nil, fmt.Errorf("GetArticle needs an explicit source for %q", articleURL)
	}
}

// sortedKeys returns the map keys sorted — used for stable error messages.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dedupeByTitle removes items with the same normalized title (case- and
// space-insensitive), preferring the first occurrence.
func dedupeByTitle(items []NewsItem) []NewsItem {
	seen := make(map[string]bool, len(items))
	out := make([]NewsItem, 0, len(items))
	for _, it := range items {
		key := strings.ToLower(strings.Join(strings.Fields(it.Title), " "))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, it)
	}
	return out
}

// sortNewsByDate sorts newest-first using the human-readable dates both
// sites emit (e.g. "Friday, July 31, 2026" / "Jul 31, 2026 04:49 PM").
func sortNewsByDate(items []NewsItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return parseDateWeight(items[i].Date) > parseDateWeight(items[j].Date)
	})
}

var months = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
	"july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "jun": 6, "jul": 7,
	"aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

// parseDateWeight converts "Friday, July 31, 2026" / "Jul 31, 2026 04:49 PM"
// into a sortable integer YYYYMMDDHHMM. Unparseable dates sort last.
func parseDateWeight(s string) int {
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	day, month, year := 0, 0, 0
	hour, minute := 0, 0
	for _, f := range fields {
		f = strings.TrimSuffix(f, ",")
		lower := strings.ToLower(f)
		if v, ok := months[lower]; ok {
			month = v
			continue
		}
		if strings.Contains(f, ":") {
			parts := strings.Split(f, ":")
			hour, minute = atoi(parts[0]), atoi(parts[1])
			continue
		}
		v := atoi(f)
		switch {
		case v >= 2000 && v <= 2100:
			year = v
		case v >= 1 && v <= 31 && day == 0:
			day = v
		}
	}
	if year == 0 {
		return 0
	}
	return year*100000000 + month*1000000 + day*10000 + hour*100 + minute
}

func atoi(s string) int {
	v := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return v
		}
		v = v*10 + int(r-'0')
	}
	return v
}
