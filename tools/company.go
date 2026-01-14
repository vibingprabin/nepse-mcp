package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/voidarchive/nepse-mcp-server/client"
	"github.com/voidarchive/nepse-mcp-server/utils"
)

func RegisterCompanyTools(s *server.MCPServer, c *client.NepseClient) {

	// 1. search_securities
	s.AddTool(mcp.NewTool("search_securities",
		mcp.WithDescription("Search for companies by name or symbol"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query (symbol or name)")),
		mcp.WithNumber("limit", mcp.Description("Maximum results to return (default: 10)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("Query is required"), nil
		}
		
		limit := request.GetInt("limit", 10)

		companies, err := c.GetCompanyList()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch company list: %v", err)), nil
		}

		var matches []string
		query = strings.ToUpper(query)
		count := 0

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**Search Results for '%s'**\n\n", query))
		sb.WriteString("| Symbol | Company Name | Sector |\n")
		sb.WriteString("|---|---|---|\n")

		for _, company := range companies {
			if strings.Contains(strings.ToUpper(company.Symbol), query) || 
			   strings.Contains(strings.ToUpper(company.CompanyName), query) {
				
				sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
					company.Symbol,
					company.CompanyName,
					company.SectorName, 
				))
				matches = append(matches, company.Symbol)
				count++
				if count >= limit {
					break
				}
			}
		}

		if count == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No securities found matching '%s'", query)), nil
		}

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 2. get_all_securities (Filtered)
	s.AddTool(mcp.NewTool("get_all_securities",
		mcp.WithDescription("List all tradable securities, optionally filtered by sector"),
		mcp.WithString("sector", mcp.Description("Filter by sector name (e.g., 'Commercial Banks')")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sectorFilter := request.GetString("sector", "")
		if sectorFilter != "" {
			sectorFilter = strings.ToUpper(sectorFilter)
		}

		companies, err := c.GetCompanyList()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch company list: %v", err)), nil
		}

		var sb strings.Builder
		if sectorFilter != "" {
			sb.WriteString(fmt.Sprintf("**Securities in Sector: %s**\n\n", sectorFilter))
		} else {
			sb.WriteString("**All Listed Securities**\n\n")
		}
		sb.WriteString("| Symbol | Company Name | Sector | Status |\n")
		sb.WriteString("|---|---|---|---|\n")

		count := 0
		for _, company := range companies {
			if sectorFilter != "" && !strings.Contains(strings.ToUpper(company.SectorName), sectorFilter) {
				continue
			}
			
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				company.Symbol,
				company.CompanyName,
				company.SectorName,
				company.Status, 
			))
			count++
		}
		
		sb.WriteString(fmt.Sprintf("\n*Total: %d securities*", count))

		return mcp.NewToolResultText(sb.String()), nil
	})

	// 3. get_company_by_symbol
	s.AddTool(mcp.NewTool("get_company_by_symbol",
		mcp.WithDescription("Get comprehensive company information and trading data by symbol"),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol (e.g., NABIL)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
		if err != nil || symbol == "" {
			return mcp.NewToolResultError("Symbol is required"), nil
		}

		if err := utils.ValidateSymbol(symbol); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		detail, err := c.GetSecurityDetail(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch details for %s: %v", symbol, err)), nil
		}
        
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", detail.Symbol)) 
		sb.WriteString("### Trading Snapshot\n")
		sb.WriteString(fmt.Sprintf("- **LTP:** %s\n", utils.FormatCurrency(detail.LastTradedPrice)))
		sb.WriteString(fmt.Sprintf("- **Volume:** %s\n", utils.FormatVolume(detail.TotalTradedQuantity)))
		sb.WriteString(fmt.Sprintf("- **Open:** %s | **High:** %s | **Low:** %s\n", 
			utils.FormatNumber(detail.OpenPrice), utils.FormatNumber(detail.HighPrice), utils.FormatNumber(detail.LowPrice)))
		sb.WriteString(fmt.Sprintf("- **52 Week High/Low:** %s / %s\n", 
			utils.FormatNumber(detail.FiftyTwoWeekHigh), utils.FormatNumber(detail.FiftyTwoWeekLow)))
		
		return mcp.NewToolResultText(sb.String()), nil
	})

    // 4. get_sector_scrips
    s.AddTool(mcp.NewTool("get_sector_scrips",
        mcp.WithDescription("Get securities grouped by sector"),
    ), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        companies, err := c.GetCompanyList()
        if err != nil {
            return mcp.NewToolResultError(fmt.Sprintf("Failed to list companies: %v", err)), nil
        }

        sectors := make(map[string][]string)
        for _, comp := range companies {
            sectors[comp.SectorName] = append(sectors[comp.SectorName], fmt.Sprintf("%s (%s)", comp.Symbol, comp.CompanyName))
        }
        
        var sectorNames []string
        for k := range sectors {
            sectorNames = append(sectorNames, k)
        }
        sort.Strings(sectorNames)

        var sb strings.Builder
        sb.WriteString("**Securities by Sector**\n\n")
        
        for _, sect := range sectorNames {
            sb.WriteString(fmt.Sprintf("### %s\n", sect))
            for _, scrip := range sectors[sect] {
                sb.WriteString(fmt.Sprintf("- %s\n", scrip))
            }
            sb.WriteString("\n")
        }
        
        return mcp.NewToolResultText(sb.String()), nil
    })

}
