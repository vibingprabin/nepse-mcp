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

func RegisterTopListTools(s *server.MCPServer, c *client.NepseClient) {

	// Consolidated top list tool
	s.AddTool(mcp.NewTool("get_top_list",
		mcp.WithDescription("Top securities. Params: type (required: 'gainers'|'losers'|'turnover'|'volume'|'transactions'), limit (default 10, max 500)."),
		mcp.WithString("type", 
			mcp.Required(), 
			mcp.Description("'gainers', 'losers', 'turnover', 'volume', 'transactions'")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 10)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		listType := request.GetString("type", "gainers")
		limit := request.GetInt("limit", 10)
		if limit > 500 { limit = 500 }
		
		var title string
		switch listType {
		case "gainers": title = "Top Gainers"
		case "losers": title = "Top Losers"
		case "turnover": title = "Top Turnover"
		case "volume": title = "Top Volume"
		case "transactions": title = "Top Transactions"
		default:
			return mcp.NewToolResultError(fmt.Sprintf("Invalid list type: %s. Use 'gainers', 'losers', 'turnover', 'volume', or 'transactions'.", listType)), nil
		}

		result, err := c.GetTopTen(listType)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch %s: %v", listType, err)), nil
		}
		
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**%s** (Top %d)\n\n", title, limit))
		
		switch list := result.(type) {
		case []api.TopGainerLoserEntry:
			if len(list) > limit { list = list[:limit] }
			sb.WriteString("| Rank | Symbol | LTP | Change | % |\n")
			sb.WriteString("|---|---|---|---|---|\n")
			for i, item := range list {
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n", i+1, item.Symbol, 
					utils.FormatNumber(item.LTP), utils.FormatNumber(item.PointChange), utils.FormatPercentage(item.PercentageChange)))
			}
		case []api.TopTradeEntry: // Volume
			if len(list) > limit { list = list[:limit] }
			sb.WriteString("| Rank | Symbol | Volume | Close Price |\n")
			sb.WriteString("|---|---|---|---|\n")
			for i, item := range list {
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, item.Symbol, 
					utils.FormatVolume(item.ShareTraded), utils.FormatNumber(item.ClosingPrice)))
			}
		case []api.TopTurnoverEntry:
			if len(list) > limit { list = list[:limit] }
			sb.WriteString("| Rank | Symbol | Turnover | Close Price |\n")
			sb.WriteString("|---|---|---|---|\n")
			for i, item := range list {
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, item.Symbol, 
					utils.FormatCurrency(item.Turnover), utils.FormatNumber(item.ClosingPrice)))
			}
		case []api.TopTransactionEntry:
			if len(list) > limit { list = list[:limit] }
			sb.WriteString("| Rank | Symbol | Trades | Last Price |\n")
			sb.WriteString("|---|---|---|---|\n")
			for i, item := range list {
				sb.WriteString(fmt.Sprintf("| %d | %s | %d | %s |\n", i+1, item.Symbol, 
					item.TotalTrades, utils.FormatNumber(item.LastTradedPrice)))
			}
		default:
			return mcp.NewToolResultError("Unknown data format returned"), nil
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}
