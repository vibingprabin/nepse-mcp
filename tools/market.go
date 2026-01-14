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

func RegisterMarketTools(s *server.MCPServer, c *client.NepseClient) {
	// 1. get_market_summary
	s.AddTool(mcp.NewTool("get_market_summary",
		mcp.WithDescription("Get overall market statistics including turnover, volume, and capitalization"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		summary, err := c.GetMarketSummary()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch market summary: %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString("**NEPSE Market Summary**\n\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|---|---|\n")

		sb.WriteString(fmt.Sprintf("| Total Turnover | %s |\n", utils.FormatCurrency(summary.TotalTurnover)))
		sb.WriteString(fmt.Sprintf("| Total Traded Shares | %s |\n", utils.FormatVolume(int64(summary.TotalTradedShares))))
		sb.WriteString(fmt.Sprintf("| Total Transactions | %s |\n", utils.FormatVolume(int64(summary.TotalTransactions))))
		sb.WriteString(fmt.Sprintf("| Total Scrips Traded | %s |\n", utils.FormatVolume(int64(summary.TotalScripsTraded))))
		sb.WriteString(fmt.Sprintf("| Total Market Cap | %s |\n", utils.FormatCurrency(summary.TotalMarketCapitalization)))
		sb.WriteString(fmt.Sprintf("| Float Market Cap | %s |\n", utils.FormatCurrency(summary.TotalFloatMarketCap)))
		
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_market_status
	s.AddTool(mcp.NewTool("get_market_status",
		mcp.WithDescription("Check if the market is currently OPEN or CLOSED"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := c.GetMarketStatus()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch market status: %v", err)), nil
		}

		state := "CLOSED"
		if status.IsMarketOpen() {
			state = "OPEN"
		}
		
		return mcp.NewToolResultText(fmt.Sprintf("Market Status: **%s**\nAs of: %s", 
			state, 
			status.AsOf,
		)), nil
	})

	// 3. get_nepse_index
	s.AddTool(mcp.NewTool("get_nepse_index",
		mcp.WithDescription("Get main NEPSE index details"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idx, err := c.GetNepseIndex()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch NEPSE index: %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString("**NEPSE Index**\n\n")
		sb.WriteString("| Index | Current | Change | % Change | High | Low |\n")
		sb.WriteString("|---|---|---|---|---|---|\n")

        sb.WriteString(fmt.Sprintf("| NEPSE | %s | %s | %s | %s | %s |\n",
            utils.FormatNumber(idx.CurrentValue),
            utils.FormatNumber(idx.PointChange),
            utils.FormatPercentage(idx.PercentChange),
            utils.FormatNumber(idx.High),
            utils.FormatNumber(idx.Low),
        ))

		return mcp.NewToolResultText(sb.String()), nil
	})
    
    // 4. get_sub_indices
    s.AddTool(mcp.NewTool("get_sub_indices",
		mcp.WithDescription("Get all sector sub-indices"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		indices, err := c.GetSubIndices()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch indices: %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString("**Sector Indices**\n\n")
		sb.WriteString("| Sector | Current | Change | % Change |\n")
		sb.WriteString("|---|---|---|---|\n")

		for _, idx := range indices {
            sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
                idx.Index,
                utils.FormatNumber(idx.CurrentValue),
                utils.FormatNumber(idx.Change),
                utils.FormatPercentage(idx.PerChange),
            ))
		}

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 5. get_live_market
	s.AddTool(mcp.NewTool("get_live_market",
		mcp.WithDescription("Get real-time price and volume data for all securities"),
		mcp.WithNumber("limit", mcp.Description("Number of securities to show (default: 20)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		liveData, err := c.GetLiveMarket()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch live market data: %v", err)), nil
		}
		
		limit := request.GetInt("limit", 20)
		
		var sb strings.Builder
		sb.WriteString("**Live Market Data**\n\n")
		sb.WriteString("| Symbol | LTP | Change | High | Low | Volume |\n")
		sb.WriteString("|---|---|---|---|---|---|\n")

		count := 0
		for _, item := range liveData {
			if count >= limit {
				break
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				item.Symbol,
				utils.FormatNumber(item.LastTradedPrice),
				utils.FormatPercentage(item.PercentageChange),
				utils.FormatNumber(item.HighPrice),
				utils.FormatNumber(item.LowPrice),
				utils.FormatVolume(item.TotalTradeQuantity),
			))
			count++
		}
		
		if len(liveData) > limit {
			sb.WriteString(fmt.Sprintf("\n*Showing top %d of %d securities*", limit, len(liveData)))
		}

		return mcp.NewToolResultText(sb.String()), nil
	})

    // 6. get_supply_demand
    s.AddTool(mcp.NewTool("get_supply_demand",
        mcp.WithDescription("Get aggregate supply and demand data"),
    ), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        data, err := c.GetSupplyDemand()
        if err != nil {
             return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch supply demand: %v", err)), nil
        }
        
        var sb strings.Builder
        sb.WriteString("**Market Supply & Demand**\n\n")
        
        var totalSupplyQty int64
        var totalDemandQty int64
        
        for _, s := range data.SupplyList { totalSupplyQty += s.TotalQuantity }
        for _, d := range data.DemandList { totalDemandQty += d.TotalQuantity }
        
        sb.WriteString(fmt.Sprintf("**Total Demand:** %s | **Total Supply:** %s\n\n", 
            utils.FormatVolume(totalDemandQty), 
            utils.FormatVolume(totalSupplyQty)))
            
        return mcp.NewToolResultText(sb.String()), nil
    })
}
