package ansu

import (
	"fmt"
	"testing"
	"time"
)

func timeIt(label string, fn func()) {
	start := time.Now()
	fn()
	fmt.Printf("  %-38s %8s\n", label, time.Since(start).Round(time.Millisecond))
}

func TestTiming(t *testing.T) {
	c := NewClient()

	fmt.Println("== COLD (first call, no cache) ==")
	var vd *ValuationDetail
	var vi []ValuationItem
	var sec []Sector
	var arts []Article
	var det *ResearchDetail

	timeIt("GetValuation(SBL)", func() { vd, _ = c.GetValuation("SBL") })
	timeIt("GetSectors()", func() { sec, _ = c.GetSectors() })
	timeIt("ListValuations(all, 10000)", func() { vi, _ = c.ListValuations(0, 10000) })
	timeIt("FindCompany(SBL)", func() { _, _ = c.FindCompany("SBL") })
	timeIt("ListArticles('')", func() { arts, _ = c.ListArticles("", 1, 15) })
	if len(arts) > 0 {
		timeIt("GetArticle(slug) [cold]", func() { det, _, _ = c.GetArticle(arts[0].Slug) })
		timeIt("GetArticle(slug) [cached]", func() { det, _, _ = c.GetArticle(arts[0].Slug) })
	} else {
		t.Log("no articles")
	}

	fmt.Println("== WARM (cached) ==")
	timeIt("GetValuation(SBL) [cache? no, uncached path]", func() { vd, _ = c.GetValuation("SBL") })
	timeIt("GetSectors() [cached]", func() { sec, _ = c.GetSectors() })
	timeIt("ListValuations(all) [cached]", func() { vi, _ = c.ListValuations(0, 10000) })
	timeIt("FindCompany(SBL) [cached]", func() { _, _ = c.FindCompany("SBL") })
	timeIt("ListArticles('') [uncached]", func() { arts, _ = c.ListArticles("", 1, 15) })
	if det != nil {
		timeIt("HTMLToText(description)", func() { _ = HTMLToText(det.Description) })
		timeIt("ExtractInlineImages(description)", func() { _ = ExtractInlineImages(det.Description) })
	}

	if vd != nil {
		fmt.Printf("  valuation verdict: %s (intrinsic %.2f vs ltp %.2f)\n", vd.Valuation, vd.IntrinsicValue, vd.MarketPrice)
	}
	fmt.Printf("  sectors: %d | screener rows: %d | articles: %d\n", len(sec), len(vi), len(arts))
}
