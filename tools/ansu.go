package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vibinprabin/nepse-mcp/ansu"
	"vibinprabin/nepse-mcp/utils"
)

// verdictAge returns "~Nmo" (months since the valuation date) plus "(stale)"
// when the model is older than 6 months. Empty string if the date can't be parsed.
func verdictAge(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return ""
	}
	months := int(time.Since(t).Hours() / 24 / 30.4)
	if months < 0 {
		months = 0
	}
	if months > 6 {
		return fmt.Sprintf("~%dmo (stale)", months)
	}
	return fmt.Sprintf("~%dmo", months)
}

const (
	includeImagesDesc = "include_images: set true ONLY if your model has vision (consume MCP image blocks). Text-only models: omit."
	valuationVerdicts = "Highly Undervalued | Undervalued | Fairly Valued | Overvalued | Highly Overvalued"
)

func RegisterAnsuTools(s *server.MCPServer) {
	ac := ansu.NewClient()

	// 1. get_stock_valuation
	s.AddTool(mcp.NewTool("get_stock_valuation",
		mcp.WithDescription("Ansu valuation verdict. TERMINAL-LAYER ONLY: do not call this to start a query — run the pipeline (structure, hands, catalyst) first and use this LAST as a counterweight ('the market is neither blind nor deaf'). Params: symbol (required). verdict ("+valuationVerdicts+"), intrinsic, LTP, P/E, P/B, EPS, upside. Assumptions in its research report."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol (e.g., NABIL)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
		if symbol == "" {
			return mcp.NewToolResultError("Symbol is required"), nil
		}

		d, err := ac.GetValuation(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch valuation for %s: %v", symbol, err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s — %s\n", symbol, d.Valuation)
		if age := verdictAge(d.IntrinsicDate); age != "" {
			fmt.Fprintf(&sb, "_Valuation date: %s (%s) | Sector: %s_\n\n", d.IntrinsicDate, age, d.Sector)
		} else {
			fmt.Fprintf(&sb, "_Valuation date: %s | Sector: %s_\n\n", d.IntrinsicDate, d.Sector)
		}
		sb.WriteString("| Metric | Value |\n|---|---|\n")
		fmt.Fprintf(&sb, "| Intrinsic value | %s |\n", utils.FormatCurrency(d.IntrinsicValue))
		fmt.Fprintf(&sb, "| Market price (LTP) | %s |\n", utils.FormatCurrency(d.MarketPrice))
		if d.IntrinsicValue > 0 {
			upside := (d.MarketPrice - d.IntrinsicValue) / d.IntrinsicValue * 100
			fmt.Fprintf(&sb, "| Upside / (downside) | %s |\n", utils.FormatPercentage(upside))
		}
		if d.EPS.Valid {
			fmt.Fprintf(&sb, "| EPS | %s |\n", utils.FormatNumber(d.EPS.Val))
		} else {
			sb.WriteString("| EPS | — |\n")
		}
		if d.PERatio.Valid {
			fmt.Fprintf(&sb, "| P/E | %.2f |\n", d.PERatio.Val)
		} else {
			sb.WriteString("| P/E | — |\n")
		}
		if d.BookValue.Valid {
			fmt.Fprintf(&sb, "| Book value | %s |\n", utils.FormatCurrency(d.BookValue.Val))
		} else {
			sb.WriteString("| Book value | — |\n")
		}
		if d.PBV.Valid {
			fmt.Fprintf(&sb, "| P/B | %.2f |\n", d.PBV.Val)
			if d.BookValue.Valid && d.BookValue.Val == d.PBV.Val {
				sb.WriteString("_⚠ Ansu reports P/B identical to book value — likely an upstream data error. Cross-check with get_company_profile._\n")
			}
		} else {
			sb.WriteString("| P/B | — |\n")
		}
		if d.SharesOutstanding != nil && *d.SharesOutstanding > 0 {
			fmt.Fprintf(&sb, "| Shares outstanding | %s |\n", utils.FormatVolume(int64(*d.SharesOutstanding)))
		}
		sb.WriteString("\n")
		sb.WriteString("_Source: ansuinvest.com valuation-details. Verdict scale: Highly Undervalued < Undervalued < Fairly Valued < Overvalued < Highly Overvalued._\n")
		fmt.Fprintf(&sb, "_The assumptions behind this intrinsic value (loan growth, credit quality, cost of equity) are in Ansu's research report for %s — get_research_articles symbol=%s to read them._", symbol, symbol)
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_valuation_screener
	s.AddTool(mcp.NewTool("get_valuation_screener",
		mcp.WithDescription("Ansu valuation screener: every covered stock (or one sector), most-undervalued first. TERMINAL-LAYER ONLY: never the entry point for a query — it ranks a model's intrinsic guess, not what the hands are doing; run the pipeline first and use this LAST. Params: sector (omit=all), limit (default 20, max 100). verdict ("+valuationVerdicts+"), intrinsic, LTP, upside. Use get_stock_valuation(symbol) for one stock. Assumptions in each report."),
		mcp.WithString("sector", mcp.Description("Sector filter (e.g., 'Commercial Banks'). Omit = all.")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 20, max 100)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sectorFilter := strings.TrimSpace(request.GetString("sector", ""))
		limit := request.GetInt("limit", 20)
		if limit <= 0 || limit > 100 {
			limit = 100
		}

		sectorID := 0
		sectorName := sectorFilter
		if sectorFilter != "" {
			sectors, err := ac.GetSectors()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to load sectors: %v", err)), nil
			}
			for _, sec := range sectors {
				if strings.EqualFold(sec.SectorName, sectorFilter) || strings.EqualFold(sec.NepseCode, sectorFilter) {
					sectorID = sec.SectorID
					sectorName = sec.SectorName
					break
				}
			}
			if sectorID == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("Unknown sector %q. Call with sector omitted or one of: %s", sectorFilter, sectorList(sectors))), nil
			}
		}

		items, err := ac.ListValuations(sectorID, 10000)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch valuation screener: %v", err)), nil
		}

		// drop junk rows (Ansu sometimes emits blank symbols or LTP 0.00)
		clean := items[:0]
		for _, it := range items {
			if strings.TrimSpace(it.CompanySymbol) == "" || it.LTP <= 0 {
				continue
			}
			clean = append(clean, it)
		}
		items = clean

		// rank most-undervalued first (largest positive intrinsic-to-LTP gap)
		sort.Slice(items, func(i, j int) bool {
			gi, gj := items[i].IntrinsicValue-items[i].LTP, items[j].IntrinsicValue-items[j].LTP
			if gi == gj {
				return items[i].CompanySymbol < items[j].CompanySymbol
			}
			return gi > gj
		})

		if len(items) > limit {
			items = items[:limit]
		}

		if len(items) == 0 {
			return mcp.NewToolResultText("No valuations found for that sector."), nil
		}

		title := "Ansu Invest Valuation Screener"
		if sectorName != "" {
			title += " — " + sectorName
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s (%d stocks)\n\n", title, len(items))
		sb.WriteString("| # | Symbol | Company | Verdict | LTP | Intrinsic | Upside | Date |\n")
		sb.WriteString("|---|---|---|---|---|---|---|---|\n")
		for i, it := range items {
			upside := ""
			if it.IntrinsicValue > 0 {
				upside = utils.FormatPercentage((it.LTP - it.IntrinsicValue) / it.IntrinsicValue * 100)
			}
			dateCell := it.IntrinsicDate
			if age := verdictAge(it.IntrinsicDate); age != "" {
				dateCell = fmt.Sprintf("%s %s", it.IntrinsicDate, age)
			}
			fmt.Fprintf(&sb, "| %d | %s | %s | %s | %s | %s | %s | %s |\n",
				i+1, it.CompanySymbol, it.CompanyName, it.Valuation,
				utils.FormatCurrency(it.LTP), utils.FormatCurrency(it.IntrinsicValue), upside, dateCell)
		}
		sb.WriteString("\n")
		sb.WriteString("_Upside = (LTP − intrinsic) / intrinsic. Negative = trading below intrinsic (potential upside). Source: ansuinvest.com valuation._\n")
		sb.WriteString("_Rows are ranked by raw discount — freshness matters: a discount against a months-old model is partly an artifact ('stale' tag). Weight recent verdicts, read the report, and ask why the gap exists before acting on any name._")
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 3. get_research_articles
	s.AddTool(mcp.NewTool("get_research_articles",
		mcp.WithDescription("Ansu research articles (expert valuation reports). Params: symbol (ticker filter, omit=all), limit (default 10, max 50), include_images (bool, vision-only). Returns title, slug, date, premium, summary. Full text: get_research_article(slug). "+includeImagesDesc),
		mcp.WithString("symbol", mcp.Description("Ticker filter. Omit = all articles.")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 10, max 50)")),
		mcp.WithBoolean("include_images", mcp.Description("true ONLY if model has vision")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
		limit := request.GetInt("limit", 10)
		if limit <= 0 || limit > 50 {
			limit = 50
		}
		includeImages := request.GetBool("include_images", false)

		articles, err := ac.ListArticles(symbol, 1, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch articles: %v", err)), nil
		}

		if len(articles) == 0 {
			msg := "No research articles found."
			if symbol != "" {
				msg = fmt.Sprintf("No research articles found for %s.", symbol)
			}
			return mcp.NewToolResultText(msg), nil
		}

		var sb strings.Builder
		if symbol != "" {
			fmt.Fprintf(&sb, "# Research & Opinion — %s (%d articles)\n\n", symbol, len(articles))
		} else {
			fmt.Fprintf(&sb, "# Research & Opinion — latest %d articles\n\n", len(articles))
		}
		for i, a := range articles {
			premium := ""
			if a.IsPremium == 1 {
				premium = " 🔒 premium"
			}
			fmt.Fprintf(&sb, "**%d. %s**%s\n", i+1, a.Title, premium)
			if a.SubTitle != "" {
				fmt.Fprintf(&sb, "_%s_\n", a.SubTitle)
			}
			fmt.Fprintf(&sb, "- **Date:** %s | **Slug:** `%s`\n", a.PostedAt, a.Slug)
			summary := ansu.StripDisclaimer(ansu.HTMLToText(a.Summary))
			if len(summary) > 400 {
				summary = summary[:400] + "…"
			}
			if summary != "" {
				fmt.Fprintf(&sb, "- **Summary:** %s\n", summary)
			}
			if includeImages && a.Image != "" {
				fmt.Fprintf(&sb, "- **Image:** %s\n", ac.HeroImageURL(a.Image))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("_Source: ansuinvest.com research. Use get_research_article with the slug for the full report._")

		if !includeImages {
			return mcp.NewToolResultText(sb.String()), nil
		}

		// vision mode: append hero photos as image content blocks (fetched in parallel)
		var contents []mcp.Content
		contents = append(contents, mcp.TextContent{Type: mcp.ContentTypeText, Text: sb.String()})
		var urls []string
		for _, a := range articles {
			if a.Image != "" {
				urls = append(urls, ac.HeroImageURL(a.Image))
			}
		}
		for _, img := range ac.FetchImages(urls) {
			if img.Err == nil {
				contents = append(contents, mcp.ImageContent{Type: mcp.ContentTypeImage, Data: img.Data, MIMEType: img.MIME})
			}
		}
		return &mcp.CallToolResult{Content: contents}, nil
	})

	// 4. get_research_article
	s.AddTool(mcp.NewTool("get_research_article",
		mcp.WithDescription("Full Ansu research article. Params: slug (required, from get_research_articles), include_images (bool, vision-only). Text, hero image, valuation charts. "+includeImagesDesc),
		mcp.WithString("slug", mcp.Required(), mcp.Description("Article slug from get_research_articles")),
		mcp.WithBoolean("include_images", mcp.Description("true ONLY if model has vision")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug := strings.TrimSpace(request.GetString("slug", ""))
		if slug == "" {
			return mcp.NewToolResultError("Slug is required"), nil
		}
		includeImages := request.GetBool("include_images", false)

		article, related, err := ac.GetArticle(slug)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch article: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s\n", article.Title)
		if article.SubTitle != "" {
			fmt.Fprintf(&sb, "_%s_\n", article.SubTitle)
		}
		premium := ""
		if article.IsPremium == 1 {
			premium = " 🔒 premium"
		}
		fmt.Fprintf(&sb, "**Posted:** %s%s\n\n", article.PostedAt, premium)

		body := ansu.StripDisclaimer(ansu.HTMLToText(article.Description))
		if body == "" {
			body = ansu.StripDisclaimer(ansu.HTMLToText(article.Summary))
		}
		if body != "" {
			sb.WriteString(body)
			sb.WriteString("\n\n")
		} else if !includeImages {
			return mcp.NewToolResultError("Article has no readable content."), nil
		}

		if len(related) > 0 {
			sb.WriteString("## Related\n")
			for i, r := range related {
				if i >= 5 {
					break
				}
				fmt.Fprintf(&sb, "- %s — slug: `%s`\n", r.Title, r.Slug)
			}
			sb.WriteString("\n")
		}

		if !includeImages {
			sb.WriteString("_Source: ansuinvest.com research. For embedded charts, re-call with include_images=true if your model has vision._")
			return mcp.NewToolResultText(sb.String()), nil
		}

		// vision mode: hero photo + inline base64 charts as image content
		var contents []mcp.Content
		var imageNotes []string
		if article.Image != "" {
			b64, mime, err := ac.FetchImageBytes(ac.HeroImageURL(article.Image))
			if err == nil {
				contents = append(contents, mcp.ImageContent{Type: mcp.ContentTypeImage, Data: b64, MIMEType: mime})
				imageNotes = append(imageNotes, "[hero image]")
			}
		}
		charts := ansu.ExtractInlineImages(article.Description)
		for i, ch := range charts {
			if len(charts) > 8 && i >= 7 {
				imageNotes = append(imageNotes, fmt.Sprintf("[+%d more charts omitted]", len(charts)-7))
				break
			}
			contents = append(contents, mcp.ImageContent{Type: mcp.ContentTypeImage, Data: ch[1], MIMEType: ch[0]})
		}
		if len(contents) > 1 {
			sb.WriteString("## Embedded images (in order)\n")
			for _, n := range imageNotes {
				sb.WriteString("- " + n + "\n")
			}
			sb.WriteString("\n")
		}
		contents = append([]mcp.Content{mcp.TextContent{Type: mcp.ContentTypeText, Text: sb.String()}}, contents...)
		return &mcp.CallToolResult{Content: contents}, nil
	})
}

func sectorList(sectors []ansu.Sector) string {
	names := make([]string, 0, len(sectors))
	for _, sec := range sectors {
		names = append(names, sec.SectorName)
	}
	return strings.Join(names, ", ")
}
