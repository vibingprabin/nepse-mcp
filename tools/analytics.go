package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/voidarchive/nepse-mcp-server/client"
	"github.com/voidarchive/nepse-mcp-server/utils"
)

func RegisterAnalyticsTools(s *server.MCPServer, c *client.NepseClient) {

	// 1. get_financial_ratios
	s.AddTool(mcp.NewTool("get_financial_ratios",
		mcp.WithDescription("Get key financial metrics and ratios for a security"),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol (e.g., NABIL)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := utils.ValidateSymbol(symbol); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		detail, err := c.GetSecurityDetail(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch details: %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Financial Ratios: %s\n\n", detail.Symbol))
		
		sb.WriteString("## Valuation\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|---|---|\n")
		sb.WriteString(fmt.Sprintf("| Last Traded Price | %s |\n", utils.FormatCurrency(detail.LastTradedPrice)))
		sb.WriteString(fmt.Sprintf("| Market Cap | %s |\n", utils.FormatCurrency(detail.MarketCap)))
		sb.WriteString(fmt.Sprintf("| Paid-Up Capital | %s |\n", utils.FormatCurrency(detail.PaidUpCapital)))
		
		sb.WriteString("\n## Trading Range\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|---|---|\n")
		sb.WriteString(fmt.Sprintf("| 52-Week High | %s |\n", utils.FormatNumber(detail.FiftyTwoWeekHigh)))
		sb.WriteString(fmt.Sprintf("| 52-Week Low | %s |\n", utils.FormatNumber(detail.FiftyTwoWeekLow)))
		sb.WriteString(fmt.Sprintf("| Current Price | %s |\n", utils.FormatCurrency(detail.LastTradedPrice)))
		
		// Calculate distance from highs/lows
		if detail.FiftyTwoWeekHigh > 0 {
			distFromHigh := ((detail.FiftyTwoWeekHigh - detail.LastTradedPrice) / detail.FiftyTwoWeekHigh) * 100
			sb.WriteString(fmt.Sprintf("| Distance from 52W High | %.2f%% |\n", distFromHigh))
		}
		
		sb.WriteString("\n## Ownership Structure\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|---|---|\n")
		sb.WriteString(fmt.Sprintf("| Promoter Holdings | %.2f%% |\n", detail.PromoterPercent))
		sb.WriteString(fmt.Sprintf("| Public Holdings | %.2f%% |\n", detail.PublicPercent))
		sb.WriteString(fmt.Sprintf("| Listed Shares | %s |\n", utils.FormatVolume(detail.ListedShares)))

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. compare_securities
	s.AddTool(mcp.NewTool("compare_securities",
		mcp.WithDescription("Compare multiple securities side-by-side"),
		mcp.WithString("symbols", mcp.Required(), mcp.Description("Comma-separated symbols (e.g., NABIL,SCB,NICA)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbolsStr, err := request.RequireString("symbols")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		symbols := strings.Split(symbolsStr, ",")
		if len(symbols) < 2 {
			return mcp.NewToolResultError("Please provide at least 2 symbols to compare"), nil
		}
		if len(symbols) > 5 {
			return mcp.NewToolResultError("Maximum 5 symbols allowed for comparison"), nil
		}

		// Trim and validate symbols
		for i := range symbols {
			symbols[i] = strings.TrimSpace(strings.ToUpper(symbols[i]))
			if err := utils.ValidateSymbol(symbols[i]); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid symbol %s: %v", symbols[i], err)), nil
			}
		}

		// Fetch details for all symbols
		type CompareData struct {
			Symbol      string
			LTP         float64
			Change      float64
			Volume      int64
			MarketCap   float64
			FiftyTwoHigh float64
			FiftyTwoLow  float64
		}

		var compareList []CompareData
		for _, sym := range symbols {
			detail, err := c.GetSecurityDetail(sym)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch %s: %v", sym, err)), nil
			}

			// Calculate percentage change
			var changePercent float64
			if detail.PreviousClose > 0 {
				changePercent = ((detail.LastTradedPrice - detail.PreviousClose) / detail.PreviousClose) * 100
			}

			compareList = append(compareList, CompareData{
				Symbol:      detail.Symbol,
				LTP:         detail.LastTradedPrice,
				Change:      changePercent,
				Volume:      detail.TotalTradedQuantity,
				MarketCap:   detail.MarketCap,
				FiftyTwoHigh: detail.FiftyTwoWeekHigh,
				FiftyTwoLow:  detail.FiftyTwoWeekLow,
			})
		}

		var sb strings.Builder
		sb.WriteString("# Securities Comparison\n\n")
		sb.WriteString("| Symbol | LTP | Change % | Volume | Market Cap | 52W High | 52W Low |\n")
		sb.WriteString("|---|---|---|---|---|---|---|\n")

		for _, data := range compareList {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
				data.Symbol,
				utils.FormatCurrency(data.LTP),
				utils.FormatPercentage(data.Change),
				utils.FormatVolume(data.Volume),
				utils.FormatCurrency(data.MarketCap),
				utils.FormatNumber(data.FiftyTwoHigh),
				utils.FormatNumber(data.FiftyTwoLow),
			))
		}

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 3. screen_stocks
	s.AddTool(mcp.NewTool("screen_stocks",
		mcp.WithDescription("Filter and screen stocks by various criteria"),
		mcp.WithString("sector", mcp.Description("Filter by sector (e.g., 'Commercial Banks')")),
		mcp.WithNumber("min_volume", mcp.Description("Minimum traded volume")),
		mcp.WithNumber("min_change", mcp.Description("Minimum percentage change")),
		mcp.WithNumber("max_change", mcp.Description("Maximum percentage change")),
		mcp.WithNumber("limit", mcp.Description("Maximum results (default: 20)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sector := request.GetString("sector", "")
		minVolume := int64(request.GetInt("min_volume", 0))
		minChange := request.GetFloat("min_change", -999999)
		maxChange := request.GetFloat("max_change", 999999)
		limit := request.GetInt("limit", 20)

		// Get live market data
		liveData, err := c.GetLiveMarket()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch market data: %v", err)), nil
		}

		// Get company list for sector filtering
		var sectorMap map[string]string
		if sector != "" {
			companies, err := c.GetCompanyList()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch company list: %v", err)), nil
			}
			sectorMap = make(map[string]string)
			for _, comp := range companies {
				sectorMap[comp.Symbol] = comp.SectorName
			}
		}

		// Apply filters
		var filtered []struct {
			Symbol  string
			LTP     float64
			Change  float64
			Volume  int64
			Sector  string
		}

		sectorUpper := strings.ToUpper(sector)
		for _, item := range liveData {
			// Sector filter
			if sector != "" {
				itemSector, ok := sectorMap[item.Symbol]
				if !ok || !strings.Contains(strings.ToUpper(itemSector), sectorUpper) {
					continue
				}
			}

			// Volume filter
			if item.TotalTradeQuantity < minVolume {
				continue
			}

			// Change filters
			if item.PercentageChange < minChange || item.PercentageChange > maxChange {
				continue
			}

			itemSector := ""
			if sectorMap != nil {
				itemSector = sectorMap[item.Symbol]
			}

			filtered = append(filtered, struct {
				Symbol  string
				LTP     float64
				Change  float64
				Volume  int64
				Sector  string
			}{
				Symbol: item.Symbol,
				LTP:    item.LastTradedPrice,
				Change: item.PercentageChange,
				Volume: item.TotalTradeQuantity,
				Sector: itemSector,
			})

			if len(filtered) >= limit {
				break
			}
		}

		if len(filtered) == 0 {
			return mcp.NewToolResultText("No stocks match the specified criteria."), nil
		}

		var sb strings.Builder
		sb.WriteString("# Stock Screener Results\n\n")
		
		// Show active filters
		sb.WriteString("**Active Filters:**\n")
		if sector != "" {
			sb.WriteString(fmt.Sprintf("- Sector: %s\n", sector))
		}
		if minVolume > 0 {
			sb.WriteString(fmt.Sprintf("- Min Volume: %s\n", utils.FormatVolume(minVolume)))
		}
		if minChange > -999999 {
			sb.WriteString(fmt.Sprintf("- Min Change: %.2f%%\n", minChange))
		}
		if maxChange < 999999 {
			sb.WriteString(fmt.Sprintf("- Max Change: %.2f%%\n", maxChange))
		}
		sb.WriteString(fmt.Sprintf("\n**Results: %d stocks**\n\n", len(filtered)))

		sb.WriteString("| Symbol | LTP | Change % | Volume | Sector |\n")
		sb.WriteString("|---|---|---|---|---|\n")

		for _, item := range filtered {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				item.Symbol,
				utils.FormatCurrency(item.LTP),
				utils.FormatPercentage(item.Change),
				utils.FormatVolume(item.Volume),
				item.Sector,
			))
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}
