package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	api "github.com/voidarchive/go-nepse"
	"vibinprabin/nepse-mcp/client"
	"vibinprabin/nepse-mcp/utils"
)

func RegisterMarketTools(s *server.MCPServer, c *client.NepseClient) {
	// 1. get_market_summary (Merged: includes optional sector data)
	s.AddTool(mcp.NewTool("get_market_summary",
		mcp.WithDescription("Market status, NEPSE index, turnover, mkt cap, supply/demand. Params: include_sectors (bool, adds sector sub-indices)."),
		mcp.WithBoolean("include_sectors", mcp.Description("Include sector-wise sub-indices")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		summary, err := c.GetMarketSummary()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed: %v", err)), nil
		}

		includeSectors := request.GetBool("include_sectors", false)

		// status, index and supply/demand are independent — fetch in parallel
		var status *api.MarketStatus
		var idx *api.NepseIndex
		var sd *api.SupplyDemandData
		done := make(chan struct{}, 3)
		go func() { defer func() { done <- struct{}{} }(); status, _ = c.GetMarketStatus() }()
		go func() { defer func() { done <- struct{}{} }(); idx, _ = c.GetNepseIndex() }()
		go func() { defer func() { done <- struct{}{} }(); sd, _ = c.GetSupplyDemand() }()
		for i := 0; i < 3; i++ {
			<-done
		}

		var sb strings.Builder
		sb.WriteString("## NEPSE Market Overview\n\n")

		if status != nil {
			state := "CLOSED"
			if status.IsMarketOpen() {
				state = "OPEN"
			}
			sb.WriteString(fmt.Sprintf("**Status:** %s (As of: %s)\n\n", state, status.AsOf))
		}

		if idx != nil {
			sb.WriteString(fmt.Sprintf("**NEPSE:** %s (%s, %s) | Range: %s - %s\n\n",
				utils.FormatNumber(idx.CurrentValue),
				utils.FormatNumber(idx.PointChange),
				utils.FormatPercentage(idx.PercentChange),
				utils.FormatNumber(idx.Low),
				utils.FormatNumber(idx.High)))
		}

		sb.WriteString("| Metric | Value |\n|---|---|\n")
		sb.WriteString(fmt.Sprintf("| Turnover | %s |\n", utils.FormatCurrency(summary.TotalTurnover)))
		sb.WriteString(fmt.Sprintf("| Traded Shares | %s |\n", utils.FormatVolume(int64(summary.TotalTradedShares))))
		sb.WriteString(fmt.Sprintf("| Transactions | %s |\n", utils.FormatVolume(int64(summary.TotalTransactions))))
		sb.WriteString(fmt.Sprintf("| Scrips Traded | %.0f |\n", summary.TotalScripsTraded))
		sb.WriteString(fmt.Sprintf("| Market Cap | %s |\n", utils.FormatCurrency(summary.TotalMarketCapitalization)))

		if sd != nil {
			var totalS, totalD int64
			for _, s := range sd.SupplyList {
				totalS += s.TotalQuantity
			}
			for _, d := range sd.DemandList {
				totalD += d.TotalQuantity
			}
			sb.WriteString(fmt.Sprintf("| Demand | %s |\n", utils.FormatVolume(totalD)))
			sb.WriteString(fmt.Sprintf("| Supply | %s |\n", utils.FormatVolume(totalS)))
		}

		if includeSectors {
			indices, err := c.GetSubIndices()
			if err == nil && len(indices) > 0 {
				sb.WriteString("\n### Sector Indices\n")
				sb.WriteString("| Sector | Value | Change |\n|---|---|---|\n")
				for _, idx := range indices {
					sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
						idx.Index, utils.FormatNumber(idx.CurrentValue), utils.FormatPercentage(idx.PerChange)))
				}
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_live_market_data
	s.AddTool(mcp.NewTool("get_live_market_data",
		mcp.WithDescription("Live LTP feed (market hours only). Params: symbols (comma-separated, omit = top active), limit (default 20)."),
		mcp.WithString("symbols", mcp.Description("Comma-separated (e.g., 'NABIL,NICA')")),
		mcp.WithNumber("limit", mcp.Description("Max securities (default: 20)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		liveData, err := c.GetLiveMarket()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch live market data: %v", err)), nil
		}

		limit := request.GetInt("limit", 20)
		symbolsStr := strings.ToUpper(request.GetString("symbols", ""))

		var targetSymbols map[string]bool
		if symbolsStr != "" {
			targetSymbols = make(map[string]bool)
			for _, s := range strings.Split(symbolsStr, ",") {
				targetSymbols[strings.TrimSpace(s)] = true
			}
			limit = 100 // Allow more results if specifically requested
		}

		var sb strings.Builder
		sb.WriteString("### Live Market Feed")
		if symbolsStr != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", symbolsStr))
		}
		sb.WriteString("\n\n")

		sb.WriteString("| Symbol | LTP | Change | High | Low | Volume |\n")
		sb.WriteString("|---|---|---|---|---|---|\n")

		count := 0
		for _, item := range liveData {
			// Filter by symbol if requested
			if len(targetSymbols) > 0 {
				if !targetSymbols[item.Symbol] {
					continue
				}
			}

			if count >= limit {
				break
			}

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				item.Symbol, utils.FormatNumber(item.LastTradedPrice),
				utils.FormatPercentage(item.PercentageChange), utils.FormatNumber(item.HighPrice),
				utils.FormatNumber(item.LowPrice), utils.FormatVolume(item.TotalTradeQuantity)))
			count++
		}

		if count == 0 {
			if len(targetSymbols) > 0 {
				sb.WriteString("\n*No matching symbols found in active trading.*")
			} else {
				sb.WriteString("\n*No active trading data available.*")
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}
