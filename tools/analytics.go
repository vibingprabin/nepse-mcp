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

func RegisterAnalyticsTools(s *server.MCPServer, c *client.NepseClient) {

	// 1. compare_securities
	s.AddTool(mcp.NewTool("compare_securities",
		mcp.WithDescription("Compare 2-5 symbols. Params: symbols (required, comma-separated, max 5). LTP, change, volume, mkt cap, 52W range."),
		mcp.WithString("symbols", mcp.Required(), mcp.Description("Comma-separated stock symbols (max 5)")),
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

		// Fetch details for all symbols in parallel
		type CompareData struct {
			Symbol       string
			LTP          float64
			Change       float64
			Volume       int64
			MarketCap    float64
			FiftyTwoHigh float64
			FiftyTwoLow  float64
		}

		type fetchResult struct {
			index  int
			detail *api.SecurityDetail
			err    error
		}

		results := make(chan fetchResult, len(symbols))
		for i, sym := range symbols {
			go func(i int, sym string) {
				d, err := c.GetSecurityDetail(sym)
				results <- fetchResult{index: i, detail: d, err: err}
			}(i, sym)
		}

		details := make([]*api.SecurityDetail, len(symbols))
		var failures []string
		for range symbols {
			r := <-results
			if r.err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", symbols[r.index], r.err))
			}
			details[r.index] = r.detail
		}
		if len(failures) == len(symbols) {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch all securities: %v", failures[0])), nil
		}

		var compareList []CompareData
		for i := range symbols {
			detail := details[i]
			if detail == nil {
				continue // reported in the failures note below
			}

			// Calculate percentage change
			var changePercent float64
			if detail.PreviousClose > 0 {
				changePercent = ((detail.LastTradedPrice - detail.PreviousClose) / detail.PreviousClose) * 100
			}

			compareList = append(compareList, CompareData{
				Symbol:       detail.Symbol,
				LTP:          detail.LastTradedPrice,
				Change:       changePercent,
				Volume:       detail.TotalTradedQuantity,
				MarketCap:    detail.MarketCap,
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

		if len(failures) > 0 {
			sb.WriteString("\n_Unavailable (fetch failed after retries): ")
			sb.WriteString(strings.Join(failures, "; "))
			sb.WriteString("_")
		}

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. screen_stocks — REMOVED: strictly weaker than get_movers_screen
	// (same live-feed source, fewer signals) and market-hours-only. The
	// movers screen plus get_top_list cover every query it served. Recoverable
	// from git history if ever needed.
}
