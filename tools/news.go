package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vibinprabin/nepse-mcp/news"
)

const newsSourcesDesc = "sharesansar (English market news) | merolagani (Nepali/Devanagari market news) | stockssessions (Nepali headline aggregator) | pulse (company disclosures: Q4 reports, rights/bonus, EN) | both (merged, newest-first, de-duplicated)"

// Kept in sync with the news package maps. Full lists live in the 'news'
// guide topic (on-demand); these stay short for the always-on schema.
const ssCategoriesDesc = "latest, announcement, exclusive, company-analysis, ipo-fpo, dividend, allotment, listing, technical, weekly, video"
const mlCategoriesDesc = "latest, corporate, company-news, stock-market, hydropower, insurance, international, interview, it-auto, market-analysis, opinion, technical, tourism, economy, monetary-policy, current-affairs, others, budget, covid, election, video"

func RegisterNewsTools(s *server.MCPServer) {
	nc := news.NewClient()

	// 1. get_news — headline scanner
	s.AddTool(mcp.NewTool("get_news",
		mcp.WithDescription("NEPSE news headlines (ShareSansar EN + Merolagani NE), ~30 tokens each. Params: source ("+newsSourcesDesc+"), category (default latest), company (ticker filter), query (keyword, ML-only), limit (default 15, max 30), page (default 1). Full category lists: get_usage_guide('news'). Lists cache 10min."),
		mcp.WithString("source", mcp.Description("sharesansar|merolagani|both (default: both)")),
		mcp.WithString("category", mcp.Description("Category (default: latest). See guide 'news' for lists.")),
		mcp.WithString("company", mcp.Description("Ticker filter (server-side, both sources)")),
		mcp.WithString("query", mcp.Description("Headline keyword (Merolagani only)")),
		mcp.WithNumber("limit", mcp.Description("Max headlines (default: 15, max 30)")),
		mcp.WithNumber("page", mcp.Description("Page (default: 1). SS page N costs N fetches.")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		source := strings.TrimSpace(request.GetString("source", "both"))
		category := strings.TrimSpace(request.GetString("category", "latest"))
		company := strings.TrimSpace(request.GetString("company", ""))
		query := strings.TrimSpace(request.GetString("query", ""))
		limit := request.GetInt("limit", 15)
		page := request.GetInt("page", 1)

		items, err := nc.GetNews(source, category, company, query, limit, page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch news: %v", err)), nil
		}
		if len(items) == 0 {
			return mcp.NewToolResultText("No news found for source=" + source + " category=" + category + " company=" + company + " query=" + query + " page=" + strconv.Itoa(page) + "."), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# %d headlines — source=%s category=%s", len(items), source, category)
		if company != "" {
			fmt.Fprintf(&sb, " company=%s", company)
		}
		if query != "" {
			fmt.Fprintf(&sb, " query=%q", query)
		}
		sb.WriteString("\n")
		for i, it := range items {
			lang := it.Language
			if lang == "ne" {
				lang = "Nepali"
			} else {
				lang = "English"
			}
			fmt.Fprintf(&sb, "%d. [%s] %s (%s, %s)\n   %s\n", i+1, lang, it.Title, it.Date, it.Source, it.URL)
			if it.Preview != "" {
				fmt.Fprintf(&sb, "   > %s\n", it.Preview)
			}
		}
		sb.WriteString("\n_Open a headline with get_news_article(source, url)._")
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_news_article — full text on demand
	s.AddTool(mcp.NewTool("get_news_article",
		mcp.WithDescription("Full text of one article. Params: url (required, from get_news), source (optional: sharesansar|merolagani|stockssessions|pulse, usually inferred), include_images (bool, vision-only). NE=Devanagari, SS=English. Pulse = company disclosure with financial tables. Articles cache 6h."),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL or path from get_news")),
		mcp.WithString("source", mcp.Description("sharesansar|merolagani|stockssessions|pulse (usually inferred from url)")),
		mcp.WithBoolean("include_images", mcp.Description("true ONLY if model has vision")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		articleURL := strings.TrimSpace(request.GetString("url", ""))
		if articleURL == "" {
			return mcp.NewToolResultError("url is required"), nil
		}
		source := strings.TrimSpace(request.GetString("source", ""))

		art, err := nc.GetArticle(source, articleURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch article: %v", err)), nil
		}

		lang := art.Language
		switch lang {
		case "ne":
			lang = "Nepali (Devanagari)"
		case "":
			lang = "English"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s\n", art.Title)
		fmt.Fprintf(&sb, "_%s | %s | %s_\n\n", lang, art.Date, art.URL)
		if art.Body == "" {
			sb.WriteString("_(no body text extracted)_")
		} else {
			sb.WriteString(art.Body)
		}

		if request.GetBool("include_images", false) {
			images := art.ImageURLs
			if len(images) > 0 {
				sb.WriteString("\n\n---\nImages:")
				for _, u := range images {
					fmt.Fprintf(&sb, "\n![news image](%s)", u)
				}
			}
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}
