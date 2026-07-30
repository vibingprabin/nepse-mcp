package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/server"
	"vibinprabin/nepse-mcp/client"
	"vibinprabin/nepse-mcp/config"
	"vibinprabin/nepse-mcp/tools"
)

const serverInstructions = `NEPSE Market Data MCP (NEPSE official API + LaganiLab broker model).

Start: get_usage_guide('overview') once per conversation; 'workflows' for call patterns; other topics as needed.

Latency:
- get_broker_floorsheet fetches ALL views in one request — switch the view param, don't re-call.
- inferred view = ALL-TIME holdings as of to_date (from_date irrelevant); other views are windowed. Run 6mo/1yr to spot dormant holders; slice weekly ranges when uncertain.
- analyze_broker_sentiment at 7d/14d/30d separates persistent patterns from fleeting.
- Live tools (get_live_market_data, get_market_depth, get_floor_sheet, screen_stocks) work in market hours only (Sun-Thu 11:00-15:00 NPT).`

func main() {
	cfg := config.LoadConfig()

	nepseClient := client.NewNepseClient(cfg)

	s := server.NewMCPServer(
		"NEPSE Market Data",
		"1.1.0",
		server.WithLogging(),
		server.WithInstructions(serverInstructions),
	)

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}
	exeDir := filepath.Dir(exePath)

	scripsPath := filepath.Join(exeDir, "Scrips.csv")

	tools.RegisterMarketTools(s, nepseClient)
	tools.RegisterSecurityTools(s, nepseClient, scripsPath)
	tools.RegisterPriceTools(s, nepseClient)
	tools.RegisterTopListTools(s, nepseClient)
	tools.RegisterGraphTools(s, nepseClient)
	tools.RegisterAnalyticsTools(s, nepseClient)

	tools.NewBrokerTools().Register(s)

	tools.RegisterCompanyProfileTools(s)

	tools.RegisterGuideTool(s)

	stdioServer := server.NewStdioServer(s)
	if err := stdioServer.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
