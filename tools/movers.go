package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	api "github.com/voidarchive/go-nepse"
	"vibinprabin/nepse-mcp/client"
	"vibinprabin/nepse-mcp/utils"
)

type moverCandidate struct {
	symbol        string
	turnover      float64
	changePct     float64
	lists         []string
	ltp           float64
	publicPercent float64
	marketCap     float64
	volumeToday   int64
	weekHigh      float64
	weekLow       float64
}

func RegisterMoversTool(s *server.MCPServer, c *client.NepseClient) {
	s.AddTool(mcp.NewTool("get_movers_screen",
		mcp.WithDescription("Screen the tape for capital-gains candidates. Cross-references top turnover/volume/gainers, then pulls float %, market cap, today's volume and 52W position per name. Returns a ranked table — volume on a small float = the squeeze profile. Combine with: get_price_history (structure), analyze_flow_change / analyze_broker_sentiment (hands), get_news (catalyst). Guide: topic='movers'."),
		mcp.WithNumber("limit", mcp.Description("Max candidates (default 15, max 30)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := request.GetInt("limit", 15)
		if limit > 30 {
			limit = 30
		}
		if limit < 1 {
			limit = 1
		}

		lists := map[string]interface{}{}
		for _, lt := range []string{"turnover", "volume", "gainers"} {
			res, err := c.GetTopTen(lt)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch top %s: %v", lt, err)), nil
			}
			lists[lt] = res
		}

		bySymbol := map[string]*moverCandidate{}
		add := func(symbol, lt string) {
			if symbol == "" {
				return
			}
			if _, ok := bySymbol[symbol]; !ok {
				bySymbol[symbol] = &moverCandidate{symbol: symbol}
			}
			bySymbol[symbol].lists = append(bySymbol[symbol].lists, lt)
		}

		if v, ok := lists["turnover"].([]api.TopTurnoverEntry); ok {
			for _, e := range v {
				add(e.Symbol, "turnover")
				bySymbol[e.Symbol].turnover = e.Turnover
			}
		}
		if v, ok := lists["volume"].([]api.TopTradeEntry); ok {
			for _, e := range v {
				add(e.Symbol, "volume")
				bySymbol[e.Symbol].volumeToday = e.ShareTraded
			}
		}
		if v, ok := lists["gainers"].([]api.TopGainerLoserEntry); ok {
			for _, e := range v {
				add(e.Symbol, "gainers")
				bySymbol[e.Symbol].changePct = e.PercentageChange
			}
		}

		if len(bySymbol) == 0 {
			return mcp.NewToolResultText("No candidates found — the tape is quiet."), nil
		}

		cands := make([]*moverCandidate, 0, len(bySymbol))
		for _, cand := range bySymbol {
			cands = append(cands, cand)
		}
		sort.Slice(cands, func(i, j int) bool {
			li, lj := len(cands[i].lists), len(cands[j].lists)
			if li != lj {
				return li > lj
			}
			return cands[i].turnover > cands[j].turnover
		})
		if len(cands) > limit {
			cands = cands[:limit]
		}

		// Fan out the per-candidate detail fetches (bounded at 8). Sequentially
		// this loop cost one NEPSE round trip per name (~0.7s each) and made
		// the flagship screen take 10s+; in parallel it is one round trip
		// total. The detail client's go-cache is thread-safe, so concurrent
		// reads are fine.
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for _, cand := range cands {
			wg.Add(1)
			go func(cand *moverCandidate) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if d, err := c.GetSecurityDetail(cand.symbol); err == nil {
					cand.ltp = d.LastTradedPrice
					if cand.ltp == 0 {
						cand.ltp = d.ClosePrice
					}
					cand.publicPercent = d.PublicPercent
					cand.marketCap = d.MarketCap
					if cand.volumeToday == 0 {
						cand.volumeToday = d.TotalTradedQuantity
					}
					cand.weekHigh = d.FiftyTwoWeekHigh
					cand.weekLow = d.FiftyTwoWeekLow
				}
			}(cand)
		}
		wg.Wait()

		var sb strings.Builder
		sb.WriteString("# Movers screen\n\n")
		sb.WriteString("| Symbol | LTP | Chg% | Vol | Float% | MktCap | 52W pos | Lists |\n")
		sb.WriteString("|---|---|---|---|---|---|---|---|\n")
		for _, cand := range cands {
			pos := "—"
			if cand.weekHigh > cand.weekLow && cand.ltp > 0 {
				pos = fmt.Sprintf("%.0f%%", (cand.ltp-cand.weekLow)/(cand.weekHigh-cand.weekLow)*100)
			}
			floatPct := "—"
			if cand.publicPercent > 0 {
				floatPct = fmt.Sprintf("%.1f%%", cand.publicPercent)
			}
			vol := "—"
			if cand.volumeToday > 0 {
				vol = utils.FormatVolume(cand.volumeToday)
			}
			fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
				cand.symbol,
				utils.FormatNumber(cand.ltp),
				utils.FormatPercentage(cand.changePct),
				vol,
				floatPct,
				utils.FormatCurrency(cand.marketCap),
				pos,
				strings.Join(cand.lists, ","),
			)
		}
		sb.WriteString("\n_Lists: which top lists the name appears in (turnover/volume/gainers). Low float % + high volume + multiple lists = the squeeze profile. Drill into candidates with get_price_history / analyze_broker_sentiment / get_broker_floorsheet / get_news._\n")
		sb.WriteString("\nBRANCHES (pick the edge whose condition matches the facts):\n")
		sb.WriteString("  • small float + volume expansion + room to the 52W high -> the structure axis: get_price_history(symbol, limit=90, include_analysis) + get_security_details(symbol)\n")
		sb.WriteString("  • parabolic (>+50%/20d) or listing <12wk -> label HIGH-RISK; size down or skip\n")
		sb.WriteString("  • deep-discount name with no volume -> skip (nothing moves it = no edge)\n")
		return mcp.NewToolResultText(sb.String()), nil
	})
}
