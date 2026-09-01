# NEPSE MCP Server

Model Context Protocol (MCP) server for the Nepal Stock Exchange — NEPSE official API + LaganiLab broker model v2 + StockSessions pulse + Ansu Invest valuations. Built with Go.

The server exposes a **buyability research pipeline**: combine six evidence axes (structure, hands, catalyst, regime, history, valuation-as-counterweight) to produce a decisive verdict — `BUYABLE` with entry/target/stop/falsifier or `NOT-NOW` with the one missing condition.

## Features

- **Market & regime:** index, turnover, breadth, supply/demand, sector sub-indices, Fear & Greed (0-100, 7 components)
- **Securities & fundamentals:** search, quote, 52W/BV/PE, Scrips 50D volume, company profiles with fundamentals & corporate actions
- **Price & tape:** OHLCV history with structure block, intraday graph, order-book depth, per-symbol floor sheet
- **Lists & movers:** gainers/losers/turnover/volume/transactions; capital-gains screener (top lists × float % × volume × 52W)
- **Broker flow (LaganiLab v2):** floorsheet (all views in one call), sentiment/concentration, flow-change (half-vs-half), bisection distribution probe; shared client cache across all broker tools
- **News:** ShareSansar (EN) + Merolagani (NE) + StockSessions (NE aggregator) + Pulse (company disclosure filings + financial tables)
- **Valuation (Ansu Invest, terminal counterweight):** intrinsic verdict + screener + research reports
- **Guide system:** 13 on-server docs (`get_usage_guide`) — the model loads `topic='pipeline'` once per query

## Prerequisites

- Go 1.25.5+ (`go.mod:3`)
- Git

## Installation

```bash
git clone https://github.com/vibingprabin/nepse-mcp.git
cd nepse-mcp
go mod tidy
go build -o nepse-mcp-server.exe .
# optional: go test ./...
```

## Configuration

| Variable | Description | Default |
|---|---|---|
| `NEPSE_CACHE_TTL` | Cache duration (`time.ParseDuration`) | `60s` (`config/config.go:18`) |
| `NEPSE_API_TIMEOUT` | Request timeout | `30s` |
| `NEPSE_LOG_LEVEL` | Logging verbosity | `info` |

## Usage

### Claude Desktop

`claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "nepse": {
      "command": "C:\\absolute\\path\\to\\nepse-mcp-server.exe"
    }
  }
}
```

### Cursor / Zed

Point the MCP server command to the absolute path of `nepse-mcp-server.exe` (or `nepse-mcp` on Unix).

### Server instructions (loaded by the model)

Defined in `main.go:15` — the model is told to load `get_usage_guide(topic='pipeline')` once per query, then investigate as deeply as needed. Valuation tools are terminal counterweights (`tools/ansu.go`), never the entry point.

## Buyability Pipeline (summary of `tools/guide.go:guidePipeline`)

Six lenses, combined until they agree or the conflict is named:

1. **Structure** — `get_price_history(include_analysis=true)` + `get_security_details` (base/breakout/extension, 52W, float %, volume)
2. **Hands** — `analyze_flow_change` + `analyze_broker_sentiment(7/14/30)` + `get_broker_floorsheet(view='inferred')` (new hand? persistent?)
3. **Catalyst** — `get_news(source='pulse', company=)` first, then `get_news(company=)` + `get_research_articles` (filings, rights/bonus, board decisions)
4. **Regime** — `get_market_sentiment` + market 20-session return (highest weight)
5. **History** — longer histories / sector analogs
6. **Valuation** — `get_stock_valuation` / `get_valuation_screener` **last only**

Verdict: `BUYABLE` (entry zone, target, stop, size vs avg vol, horizon, falsifier) or `NOT-NOW` (one missing condition) — no hedged verdicts.

## Tool Reference (24 tools, exact param names)

### Market

| Tool | Params | Notes |
|---|---|---|
| `get_market_summary` | `include_sectors?: bool` | Status, NEPSE index, turnover, mkt cap, supply/demand (`tools/market.go:18`) |
| `get_live_market_data` | `symbols?: string` (comma-separated), `limit?: int` default 20 | Market hours only Sun-Thu 11:00-15:00 NPT |
| `get_market_sentiment` | `history_days?: int` default 30 max 120, `index_key?: string` | Fear & Greed 0-100, 7 components, breadth, history (`tools/broker_analysis.go`) |

### Securities & Company

| Tool | Params |
|---|---|
| `search_securities` | `query?: string`, `sector?: string`, `limit?: int` default 20 (`tools/company.go`) |
| `get_security_details` | `symbol!: string` — LTP, 52W, fundamentals, Scrips meta (50D vol) |
| `compare_securities` | `symbols!: string` comma-separated, max 5 — parallel fetch (`tools/analytics.go`) |
| `get_company_profile` | `symbol!: string` — LaganiLab LLNMP: facts, fundamentals per period, 5-period trends, corporate actions (`tools/company_profile.go`) |

### Price, Depth & Charts

| Tool | Params |
|---|---|
| `get_price_history` | `symbol!: string`, `start_date?: YYYY-MM-DD` (default 90d back), `end_date?: YYYY-MM-DD`, `limit?: int` default 20, `sort?: 'desc'\|'asc'`, `include_analysis?: bool` (adds 20d gain, range, avg vol, volatility) (`tools/price.go`) |
| `get_market_depth` | `symbol!: string` — bid/ask order book; market hours only |
| `get_floor_sheet` | `symbol!: string`, `limit?: int` default 20 — per-symbol trade log; market hours only |
| `get_intraday_graph` | `symbol?: string` (omit = NEPSE index), `format?: 'summary'\|'points'` (`points` adds ~20 samples) (`tools/graph.go`) |

### Lists & Movers

| Tool | Params |
|---|---|
| `get_top_list` | `type!: 'gainers'\|'losers'\|'turnover'\|'volume'\|'transactions'`, `limit?: int` default 10 max 500 (`tools/toplist.go`) |
| `get_movers_screen` | `limit?: int` default 15 max 30 — cross-references turnover/volume/gainers × float % × mkt cap × volume × 52W (`tools/movers.go`) |

### Broker Flow (LaganiLab adjusted-broker-position-v2, coverage 2014-05-05)

One shared `broker.Client` (`tools/broker_analysis.go`, `main.go:71`) — single position cache & nonce lifecycle across all broker tools. `analyze_flow_change` current half reuses `analyze_broker_sentiment(period_days=N/2)` cache key.

| Tool | Params | Notes |
|---|---|---|
| `get_broker_floorsheet` | `symbol!: string`, `view?: 'summary'\|'holding'\|'released'\|'buyer'\|'seller'\|'inferred'` default summary, `top_n?: int` (0=all), `from_date?: YYYY-MM-DD`, `to_date?: YYYY-MM-DD` | **One call returns all views** — switch `view`, don't re-call. `inferred` = all-time holdings as of `to_date` (`from_date` ignored): AdjQty, Breakeven, MktValue, UnrealPL, Conf. `tools/guide.go:guideBroker` |
| `analyze_broker_sentiment` | `symbol!: string`, `period_days?: int` default 7 (use 7/14/30), `show_brokers?: bool` | Flow read + Gini ×5, HHI/Eff#, cohort purity, underwater holders |
| `analyze_flow_change` | `symbol!: string`, `lookback_days?: int` default 14 (8-60) | Current half vs prior half: new hand? accelerating? flipped? (`tools/flow_change.go`) |
| `trace_distribution` | `symbol!: string`, `from_date!: YYYY-MM-DD`, `to_date!: YYYY-MM-DD`, `broker_ids?: string` (e.g. `22,72`), `top_n?: int` default 3, `min_days?: int` default 14, `max_levels?: int` default 10 | Bisection probe for narrowest window where operator turned net seller (`tools/bisect.go`) |

### News (ShareSansar + Merolagani + StockSessions + Pulse)

| Tool | Params |
|---|---|
| `get_news` | `source?: 'sharesansar'\|'merolagani'\|'stockssessions'\|'pulse'\|'both'` (both = merged deduped), `category?: string` (SS: `latest\|announcement\|exclusive\|...` ML: `latest\|corporate\|...` — full lists in `get_usage_guide('news')`), `company?: string` ticker filter, `query?: string` keyword (ML-only), `limit?: int` default 15 max 30, `page?: int` default 1 (`tools/news.go`) |
| `get_news_article` | `url!: string` (from `get_news`), `source?: string` inferred, `include_images?: bool` vision-only | Pulse articles carry full financial tables |

Latency: lists cache 10 min; articles 6 h. StockSessions rate limit 60/min.

### Valuation — Ansu Invest (terminal counterweight, `tools/ansu.go`)

> Do not call to start a query — run the pipeline first; use last.

| Tool | Params |
|---|---|
| `get_stock_valuation` | `symbol!: string` — verdict (`Highly Undervalued` … `Highly Overvalued`), intrinsic, LTP, P/E, P/B, EPS, upside |
| `get_valuation_screener` | `sector?: string` omit=all, `limit?: int` default 20 max 100 — most-undervalued first |
| `get_research_articles` | `symbol?: string`, `limit?: int` default 10 max 50, `include_images?: bool` |
| `get_research_article` | `slug!: string` (from `get_research_articles`), `include_images?: bool` |

Assumptions live in the research report — always read it and check `get_company_profile` actuals before citing a verdict.

### Guide

| Tool | Params |
|---|---|
| `get_usage_guide` | `topic?: 'overview'\|'pipeline'\|'movers'\|'speculation'\|'value'\|'sentiment'\|'surface'\|'workflows'\|'broker'\|'market'\|'ansu'\|'news'\|'reliability'` default `overview` (`tools/guide.go:293`) |

## Workflows (from `tools/guide.go:guideWorkflows`)

- **Morning brief:** `get_market_sentiment` + `get_market_summary(include_sectors)` + `get_top_list(type='gainers'|'losers'|'turnover')`
- **Speculative trade:** `get_price_history(include_analysis)` + `analyze_broker_sentiment(7/14/30)` + `get_broker_floorsheet(view)` + `get_market_sentiment` + `get_news(company=)`
- **Value hunt:** `get_valuation_screener` → `get_stock_valuation` + `get_company_profile` + `get_price_history(include_analysis)` + `get_research_articles` → `get_research_article` + `get_broker_floorsheet(view='inferred')` + `get_news(company=)`
- **Quick quote:** one call (`get_security_details` or `get_usage_guide(topic='surface')`)

## Latency, Hours & Reliability

- `get_broker_floorsheet` fetches all views in one request; `view='inferred'` ignores `from_date` (`main.go:37`)
- `analyze_flow_change` fetches two windows in parallel; shares cache with `analyze_broker_sentiment`
- `get_market_depth`, `get_floor_sheet`, `get_live_market_data` work Sun-Thu 11:00-15:00 NPT (off-hours return last session)
- Broker replay for `type=all` takes 2-10 s over long ranges — keep ranges tight; failure auto-falls back to per-view `degraded(partial views)` (`tools/guide.go:guideReliability`)
- Pre-2014 holdings inferred, lower confidence — respect ⚠ `warnings` in floorsheet `meta`
- Floorsheets lag close; holidays serve last trading day; WASM-auth token self-heals on 401/403

## Project Layout

```
broker/        LaganiLab broker model client (names, HHI, auth)
company/       LaganiLab company data client
client/        NEPSE API client (go-nepse + WASM auth)
ansu/          Ansu Invest client
news/          ShareSansar / Merolagani / StockSessions / Pulse clients
tools/         MCP tool registrations (analytics, broker_analysis, flow_change, bisect, movers, news, ansu, guide, ...)
config/        env config
data/          CSV handler (Scrips.csv)
analyzer/      HHI helpers
```

## Development

```bash
go vet ./...    # clean
go test ./...   # ansu, broker, company, news, tools
go build -o nepse-mcp-server.exe .
```

Artifacts ignored by `.gitignore:1`: `*.exe`, `*.exe~`, `cgm_traces.json`, `*.bak`, `*.cgm-old`, `*.flat-backup`, `*.old-default`.

## License

MIT
