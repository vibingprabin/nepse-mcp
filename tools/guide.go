package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const guideOverview = `# NEPSE MCP Guide

Sources: NEPSE official API (market, prices, lists, depth, floorsheet, graphs) + LaganiLab broker model v2 (broker flow, inferred positions, fear & greed; coverage from 2014-05-05).

Tools:
- Market: get_market_summary, get_live_market_data, get_market_sentiment
- Securities: search_securities, get_security_details, get_company_profile, compare_securities, screen_stocks
- Price: get_price_history, get_market_depth, get_floor_sheet, get_intraday_graph
- Lists: get_top_list
- Broker: get_broker_floorsheet, analyze_broker_sentiment

Topics: overview | workflows | broker | sentiment | market | interpretation | reliability`

const guideWorkflows = `# Workflows

Morning brief:
1. get_market_sentiment — fear/greed + breadth + trend
2. get_market_summary include_sectors=true
3. get_top_list gainers + losers

Stock deep dive:
1. get_company_profile — full fundamentals, trends, corporate actions
2. get_security_details — price snapshot, 52W position
3. get_price_history include_analysis=true — trend, SMA, volatility
4. analyze_broker_sentiment at 7d + 30d — persistent or fleeting?
5. get_broker_floorsheet view=inferred — all-time holders, breakeven
6. floorsheet summary 6mo/1yr — dormant vs active holders; slice weekly if uncertain

Sector rotation:
1. screen_stocks sector='Hydro Power' min_volume=50000
2. compare_securities SYM1,SYM2,SYM3

Combos:
- Price new high but fear/greed falling = weakening internals (reversal watch).
- floorsheet summary: Top Accumulators ≠ Top Buyers = intraday churn, not conviction.`

const guideBroker = `# Broker Flow (LaganiLab adjusted-broker-position-v2)

Views (get_broker_floorsheet):
- summary (default) — meta + top 5 each side. Cheapest full picture.
- holding/released/buyer/seller — activity within from→to window
- inferred (alias: positions) — ALL-TIME holdings as of to_date: AdjQty, Breakeven, MktValue, UnrealPL, DivRecvd, Conf. Same table for any from_date; only to_date matters.

Inferred fields:
- Breakeven: adjusted cost/share ('-' = untraceable). Price below = underwater (overhead supply).
- Conf <40 = directional only. DivRecvd/Bonus = corporate-action-adjusted P&L.

Meta: confidence_label + warnings (mergers, pre-2014 gap, unmatched sells) — respect before trusting quantities.

Signals (analyze_broker_sentiment):
- Purity: cohort one-sidedness. >60% conviction, ~50% churn
- Gini x5 views: inequality across ALL brokers; rising across timeframes = concentration building
- HHI/Eff#: top-share dominance; Eff#<4 = fragile/monolithic
- Acc = Dist always (every buy has a sell) — read magnitude, not ratio

Workflow:
- 7d/14d/30d sentiment: pattern at all ranges = persistent; short-range only = fleeting
- 6mo & 1yr floorsheet: dormancy check — inferred holder absent from recent activity = dormant old money; present = active hand
- Uncertain? Slice weekly from→to ranges; watch accumulators/distributors evolve week by week`

const guideSentiment = `# Fear & Greed (get_market_sentiment)

Score 0-100 from 7 components: momentum, volatility (high=fear), turnover, drawdown, risk_appetite, downside_pressure (100=heavy selling), clv_pressure (closes near lows=fear).

Labels: 0-24 Extreme Fear, 25-44 Fear, 45-55 Neutral, 56-75 Greed, 76-100 Extreme Greed.

Use:
- Extreme Fear + rising AccRatio on quality = accumulation opportunity
- Extreme Greed + rising HHI concentration = late stage, tighten risk
- Price new high + falling score = weakening internals
- downside_pressure 100 on selloff = capitulation check; watch next-day breadth
- index_key (banking_subindex, hydropower_index, ...) for sector sentiment`

const guideMarket = `# Market Tools

- get_market_summary: status, index, turnover, mktcap, supply/demand; include_sectors adds sub-indices
- get_live_market_data: LTP feed, symbols to filter — market hours only
- get_top_list: gainers | losers | turnover | volume | transactions
- get_price_history: OHLCV; include_analysis adds SMA10/20, trend, range, avg vol, volatility
- get_market_depth / get_floor_sheet: order book / trade log — market hours only
- get_intraday_graph: OHLC+trend; format=points for ~20 samples

Hours: Sun-Thu 11:00-15:00 NPT. Off-hours, most endpoints return last session's data.`

const guideInterpretation = `# Interpretation

Broker flow:
- Accumulators growing + flat price = quiet accumulation (watchlist)
- Top Buyers = Top Accumulators → conviction; disjoint → churn
- released_quantity rising across brokers = broad distribution
- Low Conf + high MktValue = treat size skeptically
- High unmatched sells = model lost buy-side trail; positions understated

Fear & Greed:
- Score is context, not a trigger — combine with breadth + broker flow
- volatility 88+ = complacency; turnover <25 = participation drought

Price/volume:
- BULLISH = 10SMA > 20SMA by 0.5%; volatility >2.5% is hot for large caps
- >30% below 52W high = deep drawdown; check broker flow before bottom-fishing`

const guideReliability = `# Reliability

- Broker data lags: floorsheets processed after close; holidays serve last trading day
- type=all broker calls take ~2-10s for long ranges (full-history replay) — keep ranges tight
- Aggregated broker endpoint failure → auto per-view fallback, marked "degraded(partial views)"
- Pre-2014 holdings are inferred, lower confidence — respect ⚠ warnings in meta
- depth/floor-sheet/live-feed need market hours; WASM-auth token self-heals on 401/403
- Broker names map to NEPSE member codes 1-101; unknown render as "Broker N"`

func RegisterGuideTool(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_usage_guide",
		mcp.WithDescription("Docs: tool catalog, workflows, broker & sentiment interpretation, reliability. Topics: overview(default)|workflows|broker|sentiment|market|interpretation|reliability. Read once at conversation start."),
		mcp.WithString("topic", mcp.Description("overview|workflows|broker|sentiment|market|interpretation|reliability")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		topic := strings.ToLower(strings.TrimSpace(request.GetString("topic", "overview")))
		switch topic {
		case "", "overview":
			return mcp.NewToolResultText(guideOverview), nil
		case "workflows", "workflow", "recipes":
			return mcp.NewToolResultText(guideWorkflows), nil
		case "broker", "brokers", "floorsheet":
			return mcp.NewToolResultText(guideBroker), nil
		case "sentiment", "fear", "greed", "feargreed":
			return mcp.NewToolResultText(guideSentiment), nil
		case "market":
			return mcp.NewToolResultText(guideMarket), nil
		case "interpretation", "signals", "cheatsheet":
			return mcp.NewToolResultText(guideInterpretation), nil
		case "reliability", "limits", "errors":
			return mcp.NewToolResultText(guideReliability), nil
		default:
			return mcp.NewToolResultText(fmt.Sprintf("Unknown topic %q. Valid: overview, workflows, broker, sentiment, market, interpretation, reliability.\n\n%s", topic, guideOverview)), nil
		}
	})
}
