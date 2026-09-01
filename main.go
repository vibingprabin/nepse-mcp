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

const serverInstructions = `NEPSE Market Data MCP (NEPSE official API + LaganiLab broker model + StockSessions pulse).

You are an independent research agent: decide BUYABILITY by combining evidence
axes — structure, hands, catalyst, regime, history, valuation-as-context.
Load get_usage_guide(topic='pipeline') once per query for the default path;
investigate as deeply as the question demands — alternatives, comparisons,
and long histories are welcome. Valuation tools are terminal counterweights,
never gates. Verdicts are decisive: BUYABLE with entry/target/stop/falsifier,
or NOT-NOW with the one missing condition.

Quick quote -> get_security_details(symbol). Market direction -> topic='sentiment'.

Exact param names (use these, not shorthand):
- get_movers_screen(limit) — capital-gains screen (top lists × float % × volume × 52W position)
- get_price_history(symbol, start_date, end_date, limit, sort='desc'|'asc', include_analysis)
- get_broker_floorsheet(symbol, view=summary|holding|released|buyer|seller|inferred, top_n, from_date, to_date — dates optional, default trailing 30d; inferred = all-time as of to_date)
- analyze_broker_sentiment(symbol, period_days=7/14/30, show_brokers)
- analyze_flow_change(symbol, lookback_days=14) — current half vs prior half; new hand? changed? (facts + branches)
- trace_distribution(symbol, from_date, to_date, broker_ids?, min_days) — bisection probe isolating the narrowest windows where the operator went net seller; zoom into leaves with get_broker_floorsheet
- get_news(source, category, company, query, limit, page) → get_news_article(url, source, include_images). Sources: sharesansar (EN) | merolagani (NE) | stockssessions (NE headlines) | pulse (company Q4/disclosure reports, EN, full analysis inline — the filings feed)

Latency:
- get_broker_floorsheet fetches ALL views in one request — switch view, don't re-call. view='inferred' = ALL-TIME holdings as of to_date (from_date ignored).
- analyze_flow_change fetches two windows in parallel.
- get_news lists cache 10min; articles 6h.
- Live tools (get_live_market_data, get_market_depth, get_floor_sheet) work in market hours only (Sun-Thu 11:00-15:00 NPT).`

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
	tools.RegisterMoversTool(s, nepseClient)
	tools.RegisterGraphTools(s, nepseClient)
	tools.RegisterAnalyticsTools(s, nepseClient)

	// ONE shared BrokerTools => one broker.Client => one position cache and one
	// nonce lifecycle shared by all four broker tools. analyze_flow_change's
	// current half (now-N/2 -> today) is the SAME window key as
	// analyze_broker_sentiment(period_days=N/2), so the second call becomes a
	// cache hit instead of another 2-10s type=all replay. Three instances used
	// to mean three caches, three startup nonce fetches, and zero sharing.
	bt := tools.NewBrokerTools()
	bt.Register(s)
	bt.RegisterFlowChange(s)
	bt.RegisterBisect(s)

	tools.RegisterCompanyProfileTools(s)

	tools.RegisterNewsTools(s)

	tools.RegisterGuideTool(s)

	// Valuation tools register last: tool-list position biases selection, and
	// the counterweight layer should be the last thing the model sees.
	tools.RegisterAnsuTools(s)

	stdioServer := server.NewStdioServer(s)
	if err := stdioServer.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
