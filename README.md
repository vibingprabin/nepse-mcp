# NEPSE MCP Server

A Model Context Protocol (MCP) server for retrieving real-time and historical market data from the Nepal Stock Exchange (NEPSE). Built with Go.

## Features

- **Real-Time Data:** Market summary, live prices, indices, and depth.
- **Company Fundamentals:** Profiles, listed securities, and sector breakdown.
- **Analytics:** Financial ratios, peer comparison, and screening tools.
- **Top Lists:** Gainers, losers, turnover, and volume leaders.
- **Visuals:** Intraday price and index charts.

## Installation

### Prerequisites

- Go 1.23 or newer.

### Build from Source

```bash
git clone https://github.com/vibingprabin/nepse-mcp.git
cd nepse-mcp
go mod tidy
go build -o nepse-mcp-server.exe
```

## Configuration

Set environment variables to customize behavior.

| Variable | Description | Default |
|---|---|---|
| `NEPSE_CACHE_TTL` | Cache duration | `1m` |
| `NEPSE_API_TIMEOUT` | Request timeout | `30s` |
| `NEPSE_LOG_LEVEL` | Logging verbosity | `info` |

## Usage

### Claude Desktop

Edit `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "nepse": {
      "command": "absolute/path/to/nepse-mcp-server.exe"
    }
  }
}
```

### Cursor / Zed

Configure the MCP server command in your editor settings. Point to the absolute path of the executable.

## Tool Reference

### Market Analysis
- `get_market_summary`: Overall market statistics.
- `get_nepse_index`: Main index performance.
- `get_live_market`: Real-time trading data.
- `get_supply_demand`: Aggregate market depth.

### Stock Analysis
- `search_securities`: Fuzzy search by symbol/name.
- `get_company_by_symbol`: Detailed quote and status.
- `get_financial_ratios`: Key valuation metrics.
- `compare_securities`: Side-by-side comparison of multiple stocks.
- `screen_stocks`: Filter by sector, volume, and change.

### Charts & Lists
- `get_price_history`: OHLCV data.
- `get_nepse_index_graph`: Intraday index chart.
- `get_top_gainers`: Highest percentage gainers.
- `get_top_turnover`: Most traded by value.

## License

MIT
