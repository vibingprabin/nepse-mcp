package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vibinprabin/nepse-mcp/broker"
	"vibinprabin/nepse-mcp/utils"
)

type windowFlow struct {
	from, to string
	buy      map[int64]float64
	sell     map[int64]float64
	released float64
	acc      float64
	buyers   []broker.BrokerEntry
	sellers  []broker.BrokerEntry
	vol      int64
}

// collectWindowFlow builds per-broker bought/sold maps plus released/acc totals
// from one type=all PositionSet. bought/sold are transaction sums, so they are
// additive across sub-windows — which is what makes bisection meaningful.
func collectWindowFlow(ps *broker.PositionSet) *windowFlow {
	w := &windowFlow{from: ps.Meta.FromDate, to: ps.Meta.ToDate}
	w.buy = map[int64]float64{}
	w.sell = map[int64]float64{}
	for _, e := range broker.RowsForView(ps, "buyer") {
		id := e.BrokerID.Int()
		w.buy[id] = float64(e.TotalBought)
		w.vol += e.TotalBought.Int()
		w.buyers = append(w.buyers, e)
	}
	for _, e := range broker.RowsForView(ps, "seller") {
		id := e.BrokerID.Int()
		w.sell[id] = float64(e.TotalSold)
		w.sellers = append(w.sellers, e)
	}
	for _, e := range broker.RowsForView(ps, "released") {
		w.released += float64(e.ReleasedQuantity)
	}
	for _, e := range broker.RowsForView(ps, "holding") {
		if q := float64(e.NetQuantity); q > 0 {
			w.acc += q
		}
	}
	return w
}

func (w *windowFlow) opNet(ids map[int64]bool) float64 {
	var n float64
	for id := range ids {
		n += w.buy[id] - w.sell[id]
	}
	return n
}

// detectOperator picks topN net accumulators from the FIRST window's holding
// view — the presumed operators from the early accumulation phase.
func detectOperator(ps *broker.PositionSet, topN int) []int64 {
	rows := append([]broker.BrokerEntry(nil), ps.Data.TotalHolding...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].NetQuantity.Int() > rows[j].NetQuantity.Int() })
	var ids []int64
	for _, e := range rows {
		if e.NetQuantity.Int() <= 0 {
			continue
		}
		ids = append(ids, e.BrokerID.Int())
		if len(ids) >= topN {
			break
		}
	}
	return ids
}

type bisectLeaf struct {
	from, to string
	net      float64
	ifce     *windowFlow
}

// bisectRanges finds the smallest windows (down to minDays) where the operator
// set was net SELLING, by halving the range and recursing only into halves with
// negative operator net. bought/sold are additive so a negative half is a real
// local exit signal, not an artifact. A failed upstream probe skips its branch
// and is counted — one dead window must not kill the whole trace.
func (bt *BrokerTools) bisectRanges(symbol, from, to string, ids map[int64]bool, minDays, maxLevels int) ([]bisectLeaf, int) {
	var leaves []bisectLeaf
	failedProbes := 0
	var walk func(f, t string, depth int)
	walk = func(f, t string, depth int) {
		if depth > maxLevels {
			return
		}
		ps, err := bt.fetchPositions(symbol, f, t)
		if err != nil {
			failedProbes++
			return
		}
		wf := collectWindowFlow(ps)
		net := wf.opNet(ids)
		d1, errA := time.Parse("2006-01-02", f)
		d2, errB := time.Parse("2006-01-02", t)
		if errA != nil || errB != nil {
			return
		}
		days := int(d2.Sub(d1).Hours() / 24)
		if net >= 0 || days <= minDays || len(wf.buyers)+len(wf.sellers) == 0 {
			if net < 0 {
				leaves = append(leaves, bisectLeaf{f, t, net, wf})
			}
			return
		}
		mid := d1.Add(d2.Sub(d1) / 2).Format("2006-01-02")
		walk(f, mid, depth+1)
		walk(mid, t, depth+1)
	}
	walk(from, to, 0)
	return leaves, failedProbes
}

func parseBrokerIDs(raw string) []int64 {
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(part, "%d", &id); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func (bt *BrokerTools) RegisterBisect(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("trace_distribution",
		mcp.WithDescription("Bisection probe over floorsheet history: binary-search the range for the SMALLEST windows where the operator set turned net SELLER (distribution start). Detects the operator from early-window accumulators (broker_ids overrides; e.g. '22' or '22,72'). bought/sold are additive per window, so halving is exact. Output: operator id set, full-window stats, then each suspect sub-window with net flow + top buyers/sellers. min_days floor for leaves (default 14, 1 = single trading day). Use before a group to isolate exit windows, then zoom into the narrowest leaf with get_broker_floorsheet(from_date,to_date,view='summary')."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
		mcp.WithString("from_date", mcp.Required(), mcp.Description("Start YYYY-MM-DD")),
		mcp.WithString("to_date", mcp.Required(), mcp.Description("End YYYY-MM-DD")),
		mcp.WithString("broker_ids", mcp.Description("Comma-separated broker IDs to trace (default: top-3 accumulators from the first window)")),
		mcp.WithNumber("top_n", mcp.Description("Operator auto-detect: how many top accumulators from the early window (default 3)")),
		mcp.WithNumber("min_days", mcp.Description("Smallest leaf window in days (default 14)")),
		mcp.WithNumber("max_levels", mcp.Description("Bisection depth cap (default 10)")),
	), bt.handleTraceDistribution)
}

func (bt *BrokerTools) handleTraceDistribution(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
	fromDate := strings.TrimSpace(request.GetString("from_date", ""))
	toDate := strings.TrimSpace(request.GetString("to_date", ""))
	topN := request.GetInt("top_n", 3)
	minDays := request.GetInt("min_days", 14)
	maxLevels := request.GetInt("max_levels", 10)

	if err := utils.ValidateSymbol(symbol); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if _, _, err := utils.ValidateDateRange(fromDate, toDate, 0); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid date range: %v", err)), nil
	}
	if topN < 1 {
		topN = 3
	}
	if minDays < 1 {
		minDays = 1
	}
	if maxLevels < 1 || maxLevels > 14 {
		maxLevels = 10
	}

	var ids []int64
	if raw := request.GetString("broker_ids", ""); raw != "" {
		ids = parseBrokerIDs(raw)
	}
	if len(ids) == 0 {
		opStart := fromDate
		if d, err := time.Parse("2006-01-02", fromDate); err == nil {
			opStart = d.AddDate(0, 0, 21).Format("2006-01-02")
		}
		ps, err := bt.fetchPositions(symbol, fromDate, opStart)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error detecting operator: %v", err)), nil
		}
		ids = detectOperator(ps, topN)
		if len(ids) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("%s: no net accumulators found in the early window %s→%s — nothing to trace", symbol, fromDate, opStart)), nil
		}
	}
	idSet := map[int64]bool{}
	for _, id := range ids {
		idSet[id] = true
	}

	// Full window stats first.
	fullPS, err := bt.fetchPositions(symbol, fromDate, toDate)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error fetching full window: %v", err)), nil
	}
	full := collectWindowFlow(fullPS)

	leaves, failedProbes := bt.bisectRanges(symbol, fromDate, toDate, idSet, minDays, maxLevels)
	sort.SliceStable(leaves, func(i, j int) bool { return leaves[i].net < leaves[j].net })

	var opBuy, opSell float64
	for id := range idSet {
		opBuy += full.buy[id]
		opSell += full.sell[id]
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "OPERATOR TRACE — %s %s → %s\n", symbol, fromDate, toDate)
	fmt.Fprintf(&sb, "Operator set: ")
	for _, id := range ids {
		fmt.Fprintf(&sb, "%s(%d) ", broker.BrokerLabel(id), id)
	}
	fmt.Fprintf(&sb, "\nFull window: op bought %s / sold %s / net %+.0f | released %s | accumulated %s | vol %s\n\n",
		utils.FormatVolume(int64(opBuy)),
		utils.FormatVolume(int64(opSell)),
		opBuy-opSell,
		utils.FormatVolume(int64(full.released)),
		utils.FormatVolume(int64(full.acc)),
		utils.FormatVolume(full.vol))

	if len(leaves) == 0 {
		sb.WriteString("No sub-window found where the operator was net selling. Either they never distributed in this range, or min_days is too coarse.\n")
	} else {
		fmt.Fprintf(&sb, "Exit windows (narrowest %d found, sorted by exit size):\n", len(leaves))
		for i, lf := range leaves {
			fmt.Fprintf(&sb, "\n[%d] %s → %s | op net %+.0f\n", i+1, lf.from, lf.to, lf.net)
			sort.SliceStable(lf.ifce.buyers, func(i, j int) bool { return lf.ifce.buyers[i].TotalBought.Int() > lf.ifce.buyers[j].TotalBought.Int() })
			sort.SliceStable(lf.ifce.sellers, func(i, j int) bool { return lf.ifce.sellers[i].TotalSold.Int() > lf.ifce.sellers[j].TotalSold.Int() })
			sb.WriteString("  top buyers: ")
			for i, e := range capRows(lf.ifce.buyers, 3) {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(&sb, "%s %s", broker.BrokerLabel(e.BrokerID.Int()), utils.FormatVolume(e.TotalBought.Int()))
			}
			sb.WriteString("\n  top sellers: ")
			for i, e := range capRows(lf.ifce.sellers, 3) {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(&sb, "%s %s", broker.BrokerLabel(e.BrokerID.Int()), utils.FormatVolume(e.TotalSold.Int()))
			}
			sb.WriteString("\n")
		}
	}

	if failedProbes > 0 {
		fmt.Fprintf(&sb, "\n⚠ %d probe window(s) failed upstream and were skipped — the exit windows above are found within the probes that succeeded.\n", failedProbes)
	}

	sb.WriteString("\nNext: get_broker_floorsheet(symbol, from_date=<narrow leaf>, to_date=<leaf>, view='summary') then single days to catch the exact exit session.\n")
	return mcp.NewToolResultText(sb.String()), nil
}