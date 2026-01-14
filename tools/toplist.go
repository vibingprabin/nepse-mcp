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

func RegisterTopListTools(s *server.MCPServer, c *client.NepseClient) {

	// Generic top list tool generator using type switch for output formatting
	createTopHandler := func(listType, title string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := c.GetTopTen(listType)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch %s: %v", listType, err)), nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("**%s**\n\n", title))
			
            // Different table headers and row formatting based on return type
            switch list := result.(type) {
            case []api.TopGainerLoserEntry:
                sb.WriteString("| Rank | Symbol | LTP | Change | % |\n")
                sb.WriteString("|---|---|---|---|---|\n")
                for i, item := range list {
                    sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n", i+1, item.Symbol, 
                        utils.FormatNumber(item.LTP), utils.FormatNumber(item.PointChange), utils.FormatPercentage(item.PercentageChange)))
                }
            case []api.TopTradeEntry: // Volume
                sb.WriteString("| Rank | Symbol | Volume | Close Price |\n")
                sb.WriteString("|---|---|---|---|\n")
                for i, item := range list {
                    sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, item.Symbol, 
                        utils.FormatVolume(item.ShareTraded), utils.FormatNumber(item.ClosingPrice)))
                }
            case []api.TopTurnoverEntry:
                 sb.WriteString("| Rank | Symbol | Turnover | Close Price |\n")
                 sb.WriteString("|---|---|---|---|\n")
                 for i, item := range list {
                     sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, item.Symbol, 
                         utils.FormatCurrency(item.Turnover), utils.FormatNumber(item.ClosingPrice)))
                 }
            case []api.TopTransactionEntry:
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
		}
	}

	s.AddTool(mcp.NewTool("get_top_gainers", mcp.WithDescription("Get top gaining securities")), createTopHandler("gainers", "Top Gainers"))
	s.AddTool(mcp.NewTool("get_top_losers", mcp.WithDescription("Get top losing securities")), createTopHandler("losers", "Top Losers"))
	s.AddTool(mcp.NewTool("get_top_turnover", mcp.WithDescription("Get securities with highest turnover")), createTopHandler("turnover", "Top Turnover"))
	s.AddTool(mcp.NewTool("get_top_volume", mcp.WithDescription("Get securities with highest volume")), createTopHandler("volume", "Top Volume"))
	s.AddTool(mcp.NewTool("get_top_transactions", mcp.WithDescription("Get securities with highest number of transactions")), createTopHandler("transactions", "Top Transactions"))
}
