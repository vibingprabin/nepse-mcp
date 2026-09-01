package news

import (
	"os"
	"testing"
	"time"
)

func TestLiveGetNewsBoth(t *testing.T) {
	if os.Getenv("NEWS_LIVE") == "" {
		t.Skip("NEWS_LIVE not set")
	}
	c := NewClient()

	start := time.Now()
	items, err := c.GetNews(SourceBoth, "latest", "", "", 15, 1)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetNews both: %v", err)
	}
	t.Logf("get_news(both, latest, 15, 1) — %d items in %s", len(items), elapsed)
	if len(items) == 0 {
		t.Fatal("no items returned")
	}
	if items[0].Date == "" {
		t.Error("first item missing date")
	}

	start = time.Now()
	_, _ = c.GetNews(SourceBoth, "latest", "", "", 15, 1)
	t.Logf("get_news cached — %s", time.Since(start))

	foundSS, foundML := false, false
	for _, it := range items {
		if it.Source == SourceShareSansar {
			foundSS = true
		}
		if it.Source == SourceMerolagani {
			foundML = true
			if it.Language != "ne" {
				t.Errorf("merolagani item language = %q, want ne", it.Language)
			}
		}
	}
	if !foundSS || !foundML {
		t.Errorf("expected both sources in merged output (ss=%v ml=%v)", foundSS, foundML)
	}
}

func TestLiveGetArticleBoth(t *testing.T) {
	if os.Getenv("NEWS_LIVE") == "" {
		t.Skip("NEWS_LIVE not set")
	}
	c := NewClient()

	ssItems, err := c.GetNewsSS("latest", "", 5, 1)
	if err != nil {
		t.Fatalf("GetNewsSS: %v", err)
	}
	if len(ssItems) == 0 {
		t.Fatal("no SS items")
	}
	start := time.Now()
	ssArt, err := c.GetArticleSS(ssItems[0].URL)
	if err != nil {
		t.Fatalf("GetArticleSS: %v", err)
	}
	t.Logf("SS article %q — %d body chars in %s", ssArt.Title, len(ssArt.Body), time.Since(start))
	if ssArt.Body == "" {
		t.Error("SS body empty")
	}

	mlItems, err := c.GetNewsML("latest", "", "", 5, 1)
	if err != nil {
		t.Fatalf("GetNewsML: %v", err)
	}
	if len(mlItems) == 0 {
		t.Fatal("no ML items")
	}
	start = time.Now()
	mlArt, err := c.GetArticleML(mlItems[0].URL)
	if err != nil {
		t.Fatalf("GetArticleML: %v", err)
	}
	t.Logf("ML article %q — %d body chars in %s", mlArt.Title, len(mlArt.Body), time.Since(start))
	if mlArt.Body == "" {
		t.Error("ML body empty")
	}
	if mlArt.Language != "ne" {
		t.Errorf("ML language = %q, want ne", mlArt.Language)
	}

	start = time.Now()
	_, _ = c.GetArticleSS(ssItems[0].URL)
	t.Logf("SS article cached — %s", time.Since(start))
}

func TestLiveSSPage2(t *testing.T) {
	if os.Getenv("NEWS_LIVE") == "" {
		t.Skip("NEWS_LIVE not set")
	}
	c := NewClient()
	items, err := c.GetNewsSS("latest", "", 10, 2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("page 2 returned no items")
	}
	t.Logf("SS page 2 — %d items, first: %s", len(items), items[0].Title)
}

func TestLiveMLPage2(t *testing.T) {
	if os.Getenv("NEWS_LIVE") == "" {
		t.Skip("NEWS_LIVE not set")
	}
	c := NewClient()
	items, err := c.GetNewsML("latest", "", "", 10, 2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("page 2 returned no items")
	}
	page1, _ := c.GetNewsML("latest", "", "", 10, 1)
	titles := map[string]bool{}
	for _, it := range page1 {
		titles[it.Title] = true
	}
	different := 0
	for _, it := range items {
		if !titles[it.Title] {
			different++
		}
	}
	t.Logf("ML page 2 — %d items, %d new vs page 1", len(items), different)
	if different == 0 {
		t.Error("page 2 identical to page 1 — pagination broken")
	}
}

// TestLiveMLSearch verifies the Merolagani free-text keyword search (news=
// param) and company symbol filter (symbol= param) both narrow the feed and
// differ from the unfiltered baseline.
func TestLiveMLSearch(t *testing.T) {
	if os.Getenv("NEWS_LIVE") == "" {
		t.Skip("NEWS_LIVE not set")
	}
	c := NewClient()

	baseline, err := c.GetNewsML("latest", "", "", 15, 1)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if len(baseline) == 0 {
		t.Fatal("baseline empty")
	}

	byKeyword, err := c.GetNewsML("latest", "", "nepse", 15, 1)
	if err != nil {
		t.Fatalf("keyword search: %v", err)
	}
	t.Logf("keyword 'nepse' — %d items, baseline %d", len(byKeyword), len(baseline))
	if len(byKeyword) == 0 {
		t.Fatal("keyword search returned no items")
	}
	same := 0
	for _, a := range byKeyword {
		for _, b := range baseline {
			if a.Title == b.Title {
				same++
			}
		}
	}
	if same == len(byKeyword) {
		t.Error("keyword search identical to baseline — news= param ignored")
	}

	byCompany, err := c.GetNewsML("latest", "NABIL", "", 15, 1)
	if err != nil {
		t.Fatalf("company filter: %v", err)
	}
	t.Logf("company NABIL — %d items", len(byCompany))
	if len(byCompany) == 0 {
		t.Fatal("company filter returned no items")
	}
}
