package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/voidarchive/nepse-mcp-server/client"
)

func RegisterGraphTools(s *server.MCPServer, c *client.NepseClient) {

	// 1. get_nepse_index_graph
	s.AddTool(mcp.NewTool("get_nepse_index_graph",
		mcp.WithDescription("Get intraday graph data for main NEPSE index"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		graphData, err := c.GetNepseIndexGraph()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch graph data: %v", err)), nil
		}

        jsonData, _ := json.Marshal(graphData)
		return mcp.NewToolResultText(string(jsonData)), nil
	})

	// 2. get_security_graph
	s.AddTool(mcp.NewTool("get_security_graph",
		mcp.WithDescription("Get intraday chart data for a security"),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol, err := request.RequireString("symbol")
         if err != nil { return mcp.NewToolResultError(err.Error()), nil }
		
		graphData, err := c.GetCompanyGraph(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch graph data for %s: %v", symbol, err)), nil
		}

        jsonData, _ := json.Marshal(graphData)
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}
