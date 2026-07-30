package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"vibinprabin/nepse-mcp/client"
	"vibinprabin/nepse-mcp/data"
	"vibinprabin/nepse-mcp/utils"
)

func RegisterSecurityTools(s *server.MCPServer, c *client.NepseClient, scripsPath string) {

	// 1. search_securities
	s.AddTool(mcp.NewTool("search_securities",
		mcp.WithDescription("Find companies by name, symbol, or sector."),
		mcp.WithString("query", mcp.Description("Symbol or name to search")),
		mcp.WithString("sector", mcp.Description("e.g., 'Commercial Banks', 'Hydro Power'")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 20)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := strings.ToUpper(request.GetString("query", ""))
		sectorFilter := strings.ToUpper(request.GetString("sector", ""))
		limit := request.GetInt("limit", 20)

		companies, err := c.GetCompanyList()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch list: %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString("## Search Results\n\n")
		sb.WriteString("| Symbol | Company Name | Sector | Status |\n")
		sb.WriteString("|---|---|---|---|\n")

		count := 0
		for _, company := range companies {
			if count >= limit {
				break
			}

			matchQuery := query == "" || strings.Contains(strings.ToUpper(company.Symbol), query) || strings.Contains(strings.ToUpper(company.CompanyName), query)
			matchSector := sectorFilter == "" || strings.Contains(strings.ToUpper(company.SectorName), sectorFilter)

			if matchQuery && matchSector {
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
					company.Symbol, company.CompanyName, company.SectorName, company.Status))
				count++
			}
		}

		if count == 0 {
			return mcp.NewToolResultText("No matches found."), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_security_details
	s.AddTool(mcp.NewTool("get_security_details",
		mcp.WithDescription("Full security profile: price snapshot, 52W range, fundamentals, Scrips.csv meta. Trends: get_price_history. Flow: analyze_broker_sentiment."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol (e.g., NABIL)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol := strings.ToUpper(request.GetString("symbol", ""))
		if symbol == "" {
			return mcp.NewToolResultError("Symbol is required"), nil
		}

		detail, err := c.GetSecurityDetail(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch details for %s: %v", symbol, err)), nil
		}

		// Try to enrich with Scrips.csv data if available
		var scripInfo string
		_, scripMap, err := data.LoadScripsCSV(scripsPath)
		if err == nil && scripMap != nil {
			if sInfo := data.GetScripBySymbol(scripMap, symbol); sInfo != nil {
				scripInfo = fmt.Sprintf("\n### Additional Meta (Scrips.csv)\n- **Company Name:** %s\n- **Impact in NEPSE:** %s\n- **50D Avg Volume:** %s\n",
					sInfo.Company, sInfo.ImpactInNEPSE, utils.FormatVolume(int64(sInfo.AvgVolume50D)))
			}
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", detail.Symbol))
		sb.WriteString("### Price Snapshot\n")
		sb.WriteString(fmt.Sprintf("- **LTP:** %s\n", utils.FormatCurrency(detail.LastTradedPrice)))
		sb.WriteString(fmt.Sprintf("- **Open:** %s | **High:** %s | **Low:** %s\n",
			utils.FormatNumber(detail.OpenPrice), utils.FormatNumber(detail.HighPrice), utils.FormatNumber(detail.LowPrice)))
		sb.WriteString(fmt.Sprintf("- **52W High/Low:** %s / %s\n",
			utils.FormatNumber(detail.FiftyTwoWeekHigh), utils.FormatNumber(detail.FiftyTwoWeekLow)))
		if detail.FiftyTwoWeekHigh > 0 {
			dist := ((detail.FiftyTwoWeekHigh - detail.LastTradedPrice) / detail.FiftyTwoWeekHigh) * 100
			sb.WriteString(fmt.Sprintf("- **Distance from 52W High:** %.2f%%\n", dist))
		}
		sb.WriteString("\n")

		sb.WriteString("### Fundamental Data\n")
		sb.WriteString(fmt.Sprintf("- **Paid Up Capital:** %s\n", utils.FormatCurrency(detail.PaidUpCapital)))
		sb.WriteString(fmt.Sprintf("- **Market Cap:** %s\n", utils.FormatCurrency(detail.MarketCap)))
		sb.WriteString(fmt.Sprintf("- **Listed Shares:** %s\n", utils.FormatVolume(detail.ListedShares)))
		sb.WriteString(fmt.Sprintf("- **Public/Promoter Ratio:** %.2f%% / %.2f%%\n", detail.PublicPercent, detail.PromoterPercent))

		sb.WriteString(scripInfo)

		return mcp.NewToolResultText(sb.String()), nil
	})
}
