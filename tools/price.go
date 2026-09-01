package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	api "github.com/voidarchive/go-nepse"
	"vibinprabin/nepse-mcp/client"
	"vibinprabin/nepse-mcp/utils"
)

func RegisterPriceTools(s *server.MCPServer, c *client.NepseClient) {

	// 1. get_price_history
	s.AddTool(mcp.NewTool("get_price_history",
		mcp.WithDescription("OHLCV history. Params: symbol, start_date (YYYY-MM-DD, optional; default 90 days back), end_date (optional; default today), limit (default 20), sort ('desc' default|'asc'), include_analysis (adds structure block: 20-session gain, period range, avg vol, volatility — no oscillators)."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol (e.g., NMB)")),
		mcp.WithString("start_date", mcp.Description("Start date (YYYY-MM-DD, optional; default: 90 days back)")),
		mcp.WithString("end_date", mcp.Description("End date (YYYY-MM-DD, optional; default: today)")),
		mcp.WithNumber("limit", mcp.Description("Max rows (default: 20)")),
		mcp.WithString("sort", mcp.Description("'desc' (newest first, default) or 'asc'")),
		mcp.WithBoolean("include_analysis", mcp.Description("Add structure block: 20-session gain, range, avg vol, volatility (default: false)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
        if err != nil { return mcp.NewToolResultError(err.Error()), nil }
        
		startStr := request.GetString("start_date", "")
        
		endStr := request.GetString("end_date", "")
		
		limit := request.GetInt("limit", 20)
		sortOrder := request.GetString("sort", "desc")
		includeAnalysis := request.GetBool("include_analysis", false)

		if err := utils.ValidateSymbol(symbol); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if endStr == "" {
			endStr = time.Now().Format("2006-01-02")
		}
		if startStr == "" {
			startStr = time.Now().AddDate(0, 0, -90).Format("2006-01-02")
		}
		_, _, err = utils.ValidateDateRange(startStr, endStr, 2000) // Relaxed range check since we have limit
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid date range: %v", err)), nil
		}

		history, err := c.GetPriceHistory(symbol, startStr, endStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch price history: %v", err)), nil
		}

		// Handle Sorting
		// Note: API usu returns Descending (Newest First)
		// If user wants ASC, reverse it.
		// If user wants DESC, keep it (assuming API is Desc).
		// We should double check API, but usually it's Desc.
		// Let's protect against API variation by checking first/last date? 
		// For now, assume API returns Newest First (standard).
		
		apiIsDesc := true 
		if len(history) > 1 {
			// Check simple string comparison of dates
			if history[0].BusinessDate < history[len(history)-1].BusinessDate {
				apiIsDesc = false
			}
		}

		if sortOrder == "asc" && apiIsDesc {
			// Reverse
			utils.ReversePriceHistory(history)
		} else if sortOrder == "desc" && !apiIsDesc {
			// Reverse
			utils.ReversePriceHistory(history)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**Price History for %s** (%s to %s)\n\n", symbol, startStr, endStr))
		sb.WriteString("| Date | Close | High | Low | Vol |\n")
		sb.WriteString("|---|---|---|---|---|\n")

		for i, h := range history {
			if i >= limit {
				sb.WriteString(fmt.Sprintf("... (%d/%d rows. Increase 'limit' for more)\n", limit, len(history)))
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

		// Server-side analysis
		if includeAnalysis && len(history) >= 5 {
			// Work with ascending order for analysis
			sorted := make([]api.PriceHistory, len(history))
			copy(sorted, history)
			if len(sorted) > 1 && sorted[0].BusinessDate > sorted[len(sorted)-1].BusinessDate {
				utils.ReversePriceHistory(sorted)
			}

			n := len(sorted)
			closes := make([]float64, n)
			volumes := make([]int64, n)
			for i, h := range sorted {
				closes[i] = h.ClosePrice
				volumes[i] = h.TotalTradedQuantity
			}

			// Structure facts (no oscillators: SMA/EMA/MACD describe mood, they don't decide)
			avgVol := avgInt64Slice(volumes)

			// Latest close
			latest := closes[n-1]

			// Min/Max close over period
			minClose, maxClose := closes[0], closes[0]
			for _, c := range closes {
				if c < minClose { minClose = c }
				if c > maxClose { maxClose = c }
			}

			// 20-session gain: the pipeline's parabolic gate (HIGH-RISK if >+50%)
			sessGain := 0.0
			base := n - 21
			if base < 0 { base = 0 }
			if closes[base] > 0 { sessGain = (latest/closes[base] - 1) * 100 }

			// Distance from period high
			distFromHigh := 0.0
			if maxClose > 0 { distFromHigh = ((maxClose - latest) / maxClose) * 100 }

			// Volatility: std dev of daily returns
			var returns []float64
			for i := 1; i < n; i++ {
				if closes[i-1] > 0 {
					returns = append(returns, (closes[i]-closes[i-1])/closes[i-1]*100)
				}
			}
			volatility := stdDev(returns)

			sb.WriteString("\n### Structure\n")
			sb.WriteString(fmt.Sprintf("- **Sessions:** %d | **20-session gain:** %+.1f%% (>+50%% = parabolic, HIGH-RISK)\n", n, sessGain))
			sb.WriteString(fmt.Sprintf("- **Period Range:** %s - %s (%.1f%% below period high)\n", utils.FormatNumber(minClose), utils.FormatNumber(maxClose), distFromHigh))
			sb.WriteString(fmt.Sprintf("- **Avg Volume:** %s | **Volatility:** %.2f%%/day\n", utils.FormatVolume(int64(avgVol)), volatility))
		}
		
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_market_depth
	s.AddTool(mcp.NewTool("get_market_depth",
		mcp.WithDescription("Bid/ask order book with quantities. Params: symbol (required). Market hours only."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
        if err != nil { return mcp.NewToolResultError(err.Error()), nil }
		
		depth, err := c.GetMarketDepth(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch market depth for %s (market hours only): %v", symbol, err)), nil
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

	// 3. get_floor_sheet — per-symbol only. The market-wide sheet is a paginated
	// full-day download (minutes during trading) to render <=20 rows, so
	// symbol-less calls fail fast with pointers to the right tools instead.
	s.AddTool(mcp.NewTool("get_floor_sheet",
		mcp.WithDescription("Trade log for ONE symbol (buyer/seller broker, qty, rate). Params: symbol (required), limit (default 20). Market hours only. Market-wide activity: get_top_list(type='turnover'|'volume'). Multi-day/broker views: get_broker_floorsheet."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
		mcp.WithNumber("limit", mcp.Description("Max trades (default: 20)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := request.GetInt("limit", 20)
		symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
		if symbol == "" {
			return mcp.NewToolResultError("Market-wide trade log means downloading the full day's floor sheet (very slow). For activity use get_top_list(type='turnover'|'volume'); pass a symbol here for its trades; use get_broker_floorsheet for broker views."), nil
		}

		if err := utils.ValidateSymbol(symbol); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		sheets, err := c.GetFloorSheetBySymbol(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch floor sheet (market hours only): %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**Floor Sheet for %s**\n\n", symbol))
		
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

// Helper functions for server-side analysis

func avgFloatSlice(s []float64) float64 {
	if len(s) == 0 { return 0 }
	sum := 0.0
	for _, v := range s { sum += v }
	return sum / float64(len(s))
}

func avgInt64Slice(s []int64) float64 {
	if len(s) == 0 { return 0 }
	sum := int64(0)
	for _, v := range s { sum += v }
	return float64(sum) / float64(len(s))
}

func stdDev(s []float64) float64 {
	if len(s) < 2 { return 0 }
	mean := avgFloatSlice(s)
	sum := 0.0
	for _, v := range s {
		diff := v - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(s)-1))
}
