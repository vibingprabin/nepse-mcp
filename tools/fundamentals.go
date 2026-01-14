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

func RegisterFundamentalsTools(s *server.MCPServer, c *client.NepseClient) {

	// 1. get_company_profile
	s.AddTool(mcp.NewTool("get_company_profile",
		mcp.WithDescription("Get detailed company profile"),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
        if err != nil { return mcp.NewToolResultError(err.Error()), nil }
		
		detail, err := c.GetSecurityDetail(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch profile: %v", err)), nil
		}
		
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", detail.Symbol)) 
		sb.WriteString(fmt.Sprintf("- **ISIN:** %s\n", detail.ISIN))
		sb.WriteString(fmt.Sprintf("- **Paid Up Capital:** %s\n", utils.FormatCurrency(detail.PaidUpCapital)))
		sb.WriteString(fmt.Sprintf("- **Listed Shares:** %s\n", utils.FormatVolume(detail.ListedShares)))
		sb.WriteString(fmt.Sprintf("- **Market Cap:** %s\n", utils.FormatCurrency(detail.MarketCap)))
		sb.WriteString(fmt.Sprintf("- **Promoter/Public Ratio:** %.2f%% / %.2f%%\n", detail.PromoterPercent, detail.PublicPercent))
		
		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_dividends (Stub)
	s.AddTool(mcp.NewTool("get_dividends",
		mcp.WithDescription("Get dividend declaration history"),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Dividend history not currently available via this API source."), nil
	})
	
	// 3. get_board_of_directors (Stub)
	s.AddTool(mcp.NewTool("get_board_of_directors",
		mcp.WithDescription("Get board of directors"),
        mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Board of Directors information not available."), nil
	})
    
    // 4. get_corporate_actions (Stub)
    s.AddTool(mcp.NewTool("get_corporate_actions",
        mcp.WithDescription("Get corporate actions (bonus, rights)"),
        mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
    ), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
         return mcp.NewToolResultText("Corporate actions not available."), nil
    })
    
    // 5. get_company_reports (Stub)
    s.AddTool(mcp.NewTool("get_company_reports",
        mcp.WithDescription("Get financial reports"),
         mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
    ), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
         return mcp.NewToolResultText("Financial reports not available."), nil
    })

}
