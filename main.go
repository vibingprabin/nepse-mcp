package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/voidarchive/nepse-mcp-server/client"
	"github.com/voidarchive/nepse-mcp-server/config"
	"github.com/voidarchive/nepse-mcp-server/tools"
)

func main() {
	// Load Configuration
	cfg := config.LoadConfig()

	// Initialize NEPSE Client
	nepseClient := client.NewNepseClient(cfg)

	// Create MCP Server
	s := server.NewMCPServer(
		"NEPSE Market Data",
		"1.0.0",
		server.WithLogging(),
	)

	// Register Tools
	tools.RegisterMarketTools(s, nepseClient)
	tools.RegisterCompanyTools(s, nepseClient)
	tools.RegisterPriceTools(s, nepseClient)
	tools.RegisterTopListTools(s, nepseClient)
	tools.RegisterGraphTools(s, nepseClient)
    tools.RegisterFundamentalsTools(s, nepseClient)
	tools.RegisterAnalyticsTools(s, nepseClient)

	// Start Server (Stdio)
    stdioServer := server.NewStdioServer(s)
	if err := stdioServer.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
