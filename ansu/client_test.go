package ansu

import (
	"strings"
	"testing"
)

func TestGetValuationSBL(t *testing.T) {
	c := NewClient()
	d, err := c.GetValuation("SBL")
	if err != nil {
		t.Fatalf("GetValuation failed: %v", err)
	}
	if d.IntrinsicValue <= 0 {
		t.Fatalf("expected positive intrinsic value, got %v", d.IntrinsicValue)
	}
	if d.Valuation == "" {
		t.Fatal("valuation verdict missing")
	}
	t.Logf("SBL: verdict=%s intrinsic=%.2f ltp=%.2f eps=%v date=%s",
		d.Valuation, d.IntrinsicValue, d.MarketPrice, d.EPS.Val, d.IntrinsicDate)
}

func TestGetValuationInvalid(t *testing.T) {
	c := NewClient()
	_, err := c.GetValuation("INVALIDXYZ123")
	if err == nil {
		t.Fatal("expected error for unknown symbol")
	}
	t.Logf("Got expected error: %v", err)
}

func TestListValuationsAllSectors(t *testing.T) {
	c := NewClient()
	items, err := c.ListValuations(0, 10000)
	if err != nil {
		t.Fatalf("ListValuations failed: %v", err)
	}
	if len(items) < 100 {
		t.Fatalf("expected >100 covered stocks, got %d", len(items))
	}
	t.Logf("covered stocks: %d (sample: %s %s intrinsic=%.2f)",
		len(items), items[0].CompanySymbol, items[0].Valuation, items[0].IntrinsicValue)
}

func TestListValuationsBySector(t *testing.T) {
	c := NewClient()
	sectors, err := c.GetSectors()
	if err != nil {
		t.Fatalf("GetSectors failed: %v", err)
	}
	if len(sectors) == 0 {
		t.Fatal("no sectors returned")
	}
	items, err := c.ListValuations(sectors[0].SectorID, 10000)
	if err != nil {
		t.Fatalf("ListValuations(sector) failed: %v", err)
	}
	t.Logf("sector %s: %d valuations", sectors[0].SectorName, len(items))
}

func TestFindCompany(t *testing.T) {
	c := NewClient()
	co, err := c.FindCompany("NABIL")
	if err != nil {
		t.Fatalf("FindCompany failed: %v", err)
	}
	if co.CompanyID == 0 {
		t.Fatal("expected non-zero company_id")
	}
	t.Logf("NABIL: company_id=%d name=%s", co.CompanyID, co.CompanyName)
}

func TestListArticles(t *testing.T) {
	c := NewClient()
	arts, err := c.ListArticles("", 1, 5)
	if err != nil {
		t.Fatalf("ListArticles failed: %v", err)
	}
	if len(arts) == 0 {
		t.Fatal("no articles returned")
	}
	first := arts[0]
	if first.Slug == "" {
		t.Fatal("article slug missing")
	}
	img := c.HeroImageURL(first.Image)
	if !strings.HasPrefix(img, imageBaseURL) {
		t.Fatalf("unexpected hero image URL: %s", img)
	}
	t.Logf("articles=%d first=%q slug=%s image=%s", len(arts), first.Title, first.Slug, img)
}

func TestListArticlesByCompany(t *testing.T) {
	c := NewClient()
	arts, err := c.ListArticles("SBL", 1, 5)
	if err != nil {
		t.Fatalf("ListArticles(SBL) failed: %v", err)
	}
	if len(arts) == 0 {
		t.Fatal("expected SBL articles")
	}
	t.Logf("SBL articles: %d", len(arts))
}

func TestGetArticle(t *testing.T) {
	c := NewClient()
	art, related, err := c.GetArticle("siddhartha-bank-is-undervalued169")
	if err != nil {
		t.Fatalf("GetArticle failed: %v", err)
	}
	body := HTMLToText(art.Description)
	if len(body) < 200 {
		t.Fatalf("expected substantial article body, got %d chars", len(body))
	}
	t.Logf("title=%q body=%d chars related=%d", art.Title, len(body), len(related))
}

func TestHTMLToText(t *testing.T) {
	in := `<p dir="ltr">Hello &amp; welcome&nbsp;to&rsquo;test.</p><p>Second <strong>para</strong>.</p><img src="data:image/png;base64,AAAA">`
	out := HTMLToText(in)
	if !strings.Contains(out, "Hello & welcome") {
		t.Fatalf("entity decode failed: %q", out)
	}
	if strings.Contains(out, "<") {
		t.Fatalf("tags not stripped: %q", out)
	}
	imgs := ExtractInlineImages(in)
	if len(imgs) != 1 || imgs[0][0] != "image/png" || imgs[0][1] != "AAAA" {
		t.Fatalf("inline image extraction failed: %v", imgs)
	}
	t.Logf("HTMLToText OK: %q", out)
}

func TestFetchImageBytes(t *testing.T) {
	c := NewClient()
	arts, err := c.ListArticles("", 1, 1)
	if err != nil || len(arts) == 0 || arts[0].Image == "" {
		t.Skip("no article image available to test")
	}
	b64, mime, err := c.FetchImageBytes(c.HeroImageURL(arts[0].Image))
	if err != nil {
		t.Fatalf("FetchImageBytes failed: %v", err)
	}
	if len(b64) < 100 {
		t.Fatal("image payload suspiciously small")
	}
	t.Logf("hero image fetched: mime=%s base64len=%d", mime, len(b64))
}
