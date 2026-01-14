package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	api "github.com/voidarchive/go-nepse"
	"github.com/voidarchive/nepse-mcp-server/client"
	"github.com/voidarchive/nepse-mcp-server/utils"
)

func RegisterPriceTools(s *server.MCPServer, c *client.NepseClient) {

	// 1. get_price_history
	s.AddTool(mcp.NewTool("get_price_history",
		mcp.WithDescription("Get historical OHLCV data for a stock"),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol (e.g., NMB)")),
		mcp.WithString("start_date", mcp.Required(), mcp.Description("Start date (YYYY-MM-DD)")),
		mcp.WithString("end_date", mcp.Required(), mcp.Description("End date (YYYY-MM-DD)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
        if err != nil { return mcp.NewToolResultError(err.Error()), nil }
        
		startStr, err := request.RequireString("start_date")
        if err != nil { return mcp.NewToolResultError(err.Error()), nil }
        
		endStr, err := request.RequireString("end_date")
        if err != nil { return mcp.NewToolResultError(err.Error()), nil }

		if err := utils.ValidateSymbol(symbol); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		
		_, _, err = utils.ValidateDateRange(startStr, endStr, 365)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid date range: %v", err)), nil
		}

		history, err := c.GetPriceHistory(symbol, startStr, endStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch price history: %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**Price History for %s** (%s to %s)\n\n", symbol, startStr, endStr))
		sb.WriteString("| Date | Close | High | Low | Vol |\n")
		sb.WriteString("|---|---|---|---|---|\n")

		maxRows := 50
		for i, h := range history {
			if i >= maxRows {
				sb.WriteString("...\n")
				break
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				h.BusinessDate,
				utils.FormatNumber(h.ClosePrice),
				utils.FormatNumber(h.HighPrice),
				utils.FormatNumber(h.LowPrice),
				utils.FormatVolume(h.TotalTradedQuantity),
			))
		}
		
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_market_depth
	s.AddTool(mcp.NewTool("get_market_depth",
		mcp.WithDescription("Get order book (bid/ask) for a security"),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
        if err != nil { return mcp.NewToolResultError(err.Error()), nil }
		
		depth, err := c.GetMarketDepth(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch market depth: %v", err)), nil
		}
		
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**Market Depth: %s**\n\n", symbol))
		
		sb.WriteString("| Bid Qty | Bid Rate | -- | Ask Rate | Ask Qty |\n")
		sb.WriteString("|---|---|---|---|---|\n")
		
		lenBuy := len(depth.BuyDepth)
		lenSell := len(depth.SellDepth)
		maxLen := lenBuy
		if lenSell > maxLen {
			maxLen = lenSell
		}
		
		for i := 0; i < maxLen; i++ {
			bQty, bRate := "", ""
			if i < lenBuy {
				bQty = utils.FormatVolume(depth.BuyDepth[i].Quantity)
				bRate = utils.FormatNumber(depth.BuyDepth[i].Price)
			}
			
			aQty, aRate := "", ""
			if i < lenSell {
				aQty = utils.FormatVolume(depth.SellDepth[i].Quantity)
				aRate = utils.FormatNumber(depth.SellDepth[i].Price)
			}
			
			sb.WriteString(fmt.Sprintf("| %s | %s | | %s | %s |\n", bQty, bRate, aRate, aQty))
		}
		
		sb.WriteString(fmt.Sprintf("\n**Total Buy Qty:** %s | **Total Sell Qty:** %s", 
            utils.FormatVolume(depth.TotalBuyQty), 
            utils.FormatVolume(depth.TotalSellQty),
        ))

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 3. get_floor_sheet
	s.AddTool(mcp.NewTool("get_floor_sheet",
		mcp.WithDescription("Get trades for current trading day"),
		mcp.WithString("symbol", mcp.Description("Filter by symbol")),
		mcp.WithNumber("limit", mcp.Description("Limit results (default: 50)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := request.GetInt("limit", 50)
		symbol := request.GetString("symbol", "")
		
		var sheets []api.FloorSheetEntry
		var err error

		if symbol != "" {
			if err := utils.ValidateSymbol(symbol); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sheets, err = c.GetFloorSheetBySymbol(symbol)
		} else {
			sheets, err = c.GetFloorSheet(limit)
		}
		
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch floor sheet: %v", err)), nil
		}

		var sb strings.Builder
		if symbol != "" {
			sb.WriteString(fmt.Sprintf("**Floor Sheet for %s**\n\n", symbol))
		} else {
			sb.WriteString(fmt.Sprintf("**Market Floor Sheet** (Top %d)\n\n", limit))
		}
		
		sb.WriteString("| Symbol | Buyer | Seller | Qty | Rate | Amount |\n")
		sb.WriteString("|---|---|---|---|---|---|\n")

		count := 0
		for _, trade := range sheets {
			if count >= limit {
				break
			}
			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %s | %s | %s |\n",
				trade.StockSymbol,
				trade.BuyerMemberID,
				trade.SellerMemberID,
				utils.FormatVolume(trade.ContractQuantity),
				utils.FormatNumber(trade.ContractRate),
				utils.FormatVolume(int64(trade.ContractAmount)),
			))
			count++
		}
		
		if len(sheets) == 0 {
			sb.WriteString("\n*No trades found.*")
		} else if len(sheets) > limit {
			sb.WriteString(fmt.Sprintf("\n*Showing first %d trades*", limit))
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}
