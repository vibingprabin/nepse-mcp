package news

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	dir := os.Getenv("NEWS_FIXTURE_DIR")
	if dir == "" {
		dir = "testdata"
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return b
}

func TestParseSSCards(t *testing.T) {
	body := fixture(t, "ss_latest.html")
	items := parseSSCards(body)
	if len(items) == 0 {
		t.Fatal("expected headline cards, got none")
	}
	// ShareSansar /category/latest holds 10 headline cards.
	if len(items) != 10 {
		t.Fatalf("expected 10 cards, got %d", len(items))
	}
	first := items[0]
	if first.Source != SourceShareSansar {
		t.Errorf("source = %q, want sharesansar", first.Source)
	}
	if first.Language != "en" {
		t.Errorf("language = %q, want en", first.Language)
	}
	if !strings.HasPrefix(first.Title, "NEPSE Index Rebounds") {
		t.Errorf("first title = %q, want the NEPSE index article", first.Title)
	}
	if !strings.Contains(first.URL, "newsdetail/") {
		t.Errorf("first url = %q, want a newsdetail link", first.URL)
	}
	if first.Date == "" {
		t.Error("first card missing date")
	}
	for _, it := range items {
		if it.Title == "" {
			t.Error("empty title in card")
		}
		if it.URL == "" {
			t.Error("empty url in card")
		}
	}
}

func TestParseSSAnnouncements(t *testing.T) {
	body := fixture(t, "ss_ann.html")
	items := parseSSCards(body)
	if len(items) == 0 {
		t.Fatal("expected announcement cards, got none")
	}
	ann := false
	for _, it := range items {
		if strings.Contains(it.URL, "announcementdetail/") {
			ann = true
		}
	}
	if !ann {
		t.Error("expected at least one announcementdetail link")
	}
}

func TestParseMLCards(t *testing.T) {
	body := fixture(t, "ml_list.json")
	var records []mlNewsRecord
	if err := json.Unmarshal(body, &records); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(records) != 8 {
		t.Fatalf("expected 8 records, got %d", len(records))
	}

	items := make([]NewsItem, 0, len(records))
	for _, r := range records {
		if r.NewsTitle == "" {
			continue
		}
		items = append(items, NewsItem{
			Source:   SourceMerolagani,
			Category: "corporate",
			Title:    collapseText(r.NewsTitle),
			URL:      fmt.Sprintf("https://merolagani.com/NewsDetail.aspx?newsID=%d", r.NewsID),
			Date:     formatMLDate(r.NewsDateAD),
			Language: detectLanguage(r.NewsTitle),
		})
	}

	if len(items) != 8 {
		t.Fatalf("expected 8 items, got %d", len(items))
	}
	first := items[0]
	if first.Source != SourceMerolagani {
		t.Errorf("source = %q, want merolagani", first.Source)
	}
	if first.Language != "ne" {
		t.Errorf("language = %q, want ne (Devanagari)", first.Language)
	}
	if !strings.Contains(first.URL, "newsID=129167") {
		t.Errorf("first url = %q, want newsID=129167", first.URL)
	}
	if first.Date != "Jul 31, 2026 04:49 PM" {
		t.Errorf("first date = %q, want Jul 31, 2026 04:49 PM", first.Date)
	}
	// mixed-language detection: the English headline should be "en"
	for _, it := range items {
		if it.Title == "NEPSE Index Rebounds By 0.48%" && it.Language != "en" {
			t.Errorf("english headline language = %q, want en", it.Language)
		}
	}
}

func TestParseSSArticle(t *testing.T) {
	body := fixture(t, "ss_detail.html")
	art := parseSSArticle(body, "https://www.sharesansar.com/newsdetail/test")
	if !strings.Contains(art.Title, "NEPSE Index Rebounds") {
		t.Errorf("title = %q", art.Title)
	}
	if art.Date == "" {
		t.Error("missing date")
	}
	if !strings.Contains(art.Body, "2,685.54") {
		t.Errorf("body missing index close: %q", art.Body[:min(200, len(art.Body))])
	}
	if len(art.ImageURLs) == 0 {
		t.Error("expected content images (charts)")
	}
}

func TestParseMLArticle(t *testing.T) {
	body := fixture(t, "ml_detail.html")
	art := parseMLArticle(body, "https://merolagani.com/NewsDetail.aspx?newsID=129167")
	if art.Title == "" {
		t.Error("missing title")
	}
	if art.Language != "ne" {
		t.Errorf("language = %q, want ne", art.Language)
	}
	if art.Date == "" {
		t.Error("missing date")
	}
	if art.Body == "" {
		t.Error("missing body")
	}
	// the inline ad block must be stripped from the body
	if strings.Contains(art.Body, "laxmisunrise") {
		t.Error("inline advertisement not stripped from body")
	}
	if strings.Contains(art.Body, "bigyapan") {
		t.Error("ad image not stripped from body")
	}
	if len(art.ImageURLs) == 0 {
		t.Error("expected an article image")
	}
}

func TestDedupeByTitle(t *testing.T) {
	in := []NewsItem{
		{Title: "Nepal  Stock  Exchange"},
		{Title: "nepal stock exchange"},
		{Title: "Another Headline"},
		{Title: ""},
	}
	out := dedupeByTitle(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 deduped items, got %d", len(out))
	}
}

func TestParseDateWeight(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"Friday, July 31, 2026", 202607310000},
		{"Jul 31, 2026 04:49 PM", 202607310449},
		{"Friday, July 31, 2026 3:47 PM", 202607310347},
		{"garbage", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := parseDateWeight(tc.in); got != tc.want {
			t.Errorf("parseDateWeight(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSortNewsByDate(t *testing.T) {
	items := []NewsItem{
		{Title: "old", Date: "Jul 29, 2026"},
		{Title: "new", Date: "Jul 31, 2026"},
		{Title: "mid", Date: "Jul 30, 2026"},
	}
	sortNewsByDate(items)
	if items[0].Title != "new" || items[2].Title != "old" {
		t.Errorf("sort order wrong: %v", items)
	}
}
