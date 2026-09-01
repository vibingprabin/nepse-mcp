package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vibinprabin/nepse-mcp/broker"
	"vibinprabin/nepse-mcp/utils"
)

// flowBroker is one broker's activity inside one window.
type flowBroker struct {
	id   int64
	buy  float64
	sell float64
}

// flowWindowData is the per-broker activity of a single window.
type flowWindowData struct {
	brokers   []flowBroker
	totalBuy  float64
	totalSell float64
	net       float64
	accQty    float64 // net accumulation (holding view) within the window
	distQty   float64 // released quantity (released view) within the window
	top5Share float64
	days      int
	from, to  string
}

// dirQty is the window's direction: accumulation minus released.
// The buyer/seller views always net to ~0 (every buy has a sell in-window),
// so direction must come from the holding/released views.
func (w flowWindowData) dirQty() float64 { return w.accQty - w.distQty }

// flowChange is the deterministic window-vs-window comparison.
// It reports FACTS and the BRANCHES conditioned on them — no verdict.
type flowChange struct {
	newBuyers      []flowBroker // absent (or near-absent) prior, significant now
	newSellers     []flowBroker
	accelBuyers    []flowBroker // >= 2x prior volume, still significant
	accelSellers   []flowBroker
	persistentBuy  []flowBroker // significant in BOTH windows (continuity)
	persistentSell []flowBroker
	buyDeltaPct    float64
	sellDeltaPct   float64
	netPrior       float64
	netNow         float64
}

const (
	sigFloorPct   = 0.01 // broker must hold >=1% of window buy volume to count
	absentPct     = 0.0025
	accelFactor   = 2.0
	newBuyerShare = 0.10 // >=1 new broker holding >=10% of buys => notable
)

func buildFlowWindow(ps *broker.PositionSet) flowWindowData {
	w := flowWindowData{from: ps.Meta.FromDate, to: ps.Meta.ToDate}
	buyRows := broker.RowsForView(ps, "buyer")
	sellRows := broker.RowsForView(ps, "seller")

	byID := map[int64]*flowBroker{}
	for _, e := range buyRows {
		b := float64(e.TotalBought)
		if b <= 0 {
			continue
		}
		fb, ok := byID[e.BrokerID.Int()]
		if !ok {
			fb = &flowBroker{id: e.BrokerID.Int()}
			byID[fb.id] = fb
		}
		fb.buy += b
		w.totalBuy += b
		if int(e.TradingDays) > w.days {
			w.days = int(e.TradingDays)
		}
	}
	for _, e := range sellRows {
		s := float64(e.TotalSold)
		if s <= 0 {
			continue
		}
		fb, ok := byID[e.BrokerID.Int()]
		if !ok {
			fb = &flowBroker{id: e.BrokerID.Int()}
			byID[fb.id] = fb
		}
		fb.sell += s
		w.totalSell += s
		if int(e.TradingDays) > w.days {
			w.days = int(e.TradingDays)
		}
	}
	w.net = w.totalBuy - w.totalSell
	for _, e := range broker.RowsForView(ps, "holding") {
		if q := float64(e.NetQuantity); q > 0 {
			w.accQty += q
		}
	}
	for _, e := range broker.RowsForView(ps, "released") {
		w.distQty += float64(e.ReleasedQuantity)
	}
	for _, fb := range byID {
		w.brokers = append(w.brokers, *fb)
	}
	sort.Slice(w.brokers, func(i, j int) bool { return w.brokers[i].buy > w.brokers[j].buy })
	if len(w.brokers) > 0 {
		top := w.brokers
		if len(top) > 5 {
			top = top[:5]
		}
		var t float64
		for _, fb := range top {
			t += fb.buy
		}
		if w.totalBuy > 0 {
			w.top5Share = t / w.totalBuy
		}
	}
	return w
}

// analyzeFlowChange compares current vs prior window. Facts only.
func analyzeFlowChange(cur, prior flowWindowData) flowChange {
	fc := flowChange{netPrior: prior.dirQty(), netNow: cur.dirQty()}
	if prior.totalBuy > 0 {
		fc.buyDeltaPct = (cur.totalBuy/prior.totalBuy - 1) * 100
	}
	if prior.totalSell > 0 {
		fc.sellDeltaPct = (cur.totalSell/prior.totalSell - 1) * 100
	}

	priorBuy, priorSell := map[int64]float64{}, map[int64]float64{}
	for _, fb := range prior.brokers {
		priorBuy[fb.id] = fb.buy
		priorSell[fb.id] = fb.sell
	}

	for _, fb := range cur.brokers {
		if cur.totalBuy <= 0 || fb.buy/cur.totalBuy < sigFloorPct {
			continue
		}
		pb := priorBuy[fb.id]
		switch {
		case pb == 0 || (prior.totalBuy > 0 && pb/prior.totalBuy < absentPct):
			fc.newBuyers = append(fc.newBuyers, fb)
		case pb*accelFactor <= fb.buy:
			fc.accelBuyers = append(fc.accelBuyers, fb)
		default:
			if prior.totalBuy > 0 && pb/prior.totalBuy >= sigFloorPct {
				fc.persistentBuy = append(fc.persistentBuy, fb)
			}
		}
	}
	for _, fb := range cur.brokers {
		if cur.totalSell <= 0 || fb.sell/cur.totalSell < sigFloorPct {
			continue
		}
		ps := priorSell[fb.id]
		switch {
		case ps == 0 || (prior.totalSell > 0 && ps/prior.totalSell < absentPct):
			fc.newSellers = append(fc.newSellers, fb)
		case ps*accelFactor <= fb.sell:
			fc.accelSellers = append(fc.accelSellers, fb)
		default:
			if prior.totalSell > 0 && ps/prior.totalSell >= sigFloorPct {
				fc.persistentSell = append(fc.persistentSell, fb)
			}
		}
	}
	return fc
}

func flowLine(fb flowBroker, total float64) string {
	share := 0.0
	if total > 0 {
		share = fb.buy / total * 100
	}
	return fmt.Sprintf("%s — %.1f%% of buys", broker.BrokerLabel(fb.id), share)
}

func flowSellLine(fb flowBroker, total float64) string {
	share := 0.0
	if total > 0 {
		share = fb.sell / total * 100
	}
	return fmt.Sprintf("%s — %.1f%% of sells", broker.BrokerLabel(fb.id), share)
}

// branchesFor computes the next-call edges conditioned on the facts.
// Every branch names the tool to call and the question it answers.
func branchesFor(fc flowChange, cur flowWindowData, prior flowWindowData) []string {
	var b []string
	newBuyShare := 0.0
	for _, fb := range fc.newBuyers {
		newBuyShare += fb.buy
	}
	if cur.totalBuy > 0 {
		newBuyShare /= cur.totalBuy
	}
	newSellShare := 0.0
	for _, fb := range fc.newSellers {
		newSellShare += fb.sell
	}
	if cur.totalSell > 0 {
		newSellShare /= cur.totalSell
	}

	handsFresh := (len(fc.newBuyers) >= 1 && newBuyShare >= newBuyerShare) ||
		(cur.totalBuy >= accelFactor*prior.totalBuy && cur.dirQty() > 0)
	distribution := (len(fc.newSellers) >= 1 && newSellShare >= newBuyerShare) ||
		(cur.totalSell >= accelFactor*prior.totalSell && cur.dirQty() < 0) ||
		(prior.dirQty() >= 0 && cur.dirQty() < 0)
	churn := !handsFresh && !distribution && len(fc.persistentBuy) > 0 && len(fc.persistentSell) > 0

	if handsFresh {
		b = append(b,
			"new buyer or accelerating buy volume -> analyze_broker_sentiment(symbol, period_days=7) then (14): is the new hand persistent or a flash?",
			"-> get_broker_floorsheet(symbol, view='inferred'): who now holds and at what cost (fuel below the price, overhead above it)",
			"-> get_news(company=symbol): the catalyst that would explain a fresh hand (dividend, AGM, rights, earnings, board action)",
		)
	}
	if distribution {
		b = append(b,
			"net flow turned negative or new sellers -> get_price_history(symbol, limit=30, include_analysis): breakdown or shakeout?",
			"-> get_news(query=symbol, category='announcement'): a distribution trigger (promoter sale, rights, weak results)",
		)
	}
	if churn {
		b = append(b,
			"persistent hands on both sides -> widen the view: analyze_flow_change(symbol, lookback_days=28) — is the standoff older than the window?",
		)
	}
	if len(fc.newBuyers) == 0 && len(fc.newSellers) == 0 && len(fc.accelBuyers) == 0 && len(fc.accelSellers) == 0 && len(fc.persistentBuy) == 0 && len(fc.persistentSell) == 0 {
		b = append(b, "no material flow in either window -> no case from the hands; move to the next candidate or check the price tape for a setup the flow hasn't confirmed")
	}
	if len(b) == 0 {
		b = append(b, "flow is stable but one-sided -> read get_price_history(symbol, limit=90, include_analysis) for where in the move this stands (base, breakout, extension)")
	}
	return b
}

func (bt *BrokerTools) RegisterFlowChange(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("analyze_flow_change",
		mcp.WithDescription("Window-vs-window broker flow comparison: the CHANGE read. Splits lookback_days (default 14) into current half vs prior half and answers, from the floorsheet facts: did a NEW hand start buying (absent before, active now)? Is buying accelerating, persistent, or churn? Did net flow flip? Output = facts + BRANCHES (the next call each fact implies). Combine with: analyze_broker_sentiment (persistence across 7/14/30d), get_broker_floorsheet(view='inferred') (who holds at what cost), get_news (the catalyst that would explain a fresh hand)."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
		mcp.WithNumber("lookback_days", mcp.Description("Total lookback split in half: 14 = last 7d vs prior 7d (default), 28 = 14d vs 14d")),
	), bt.handleAnalyzeFlowChange)
}

func (bt *BrokerTools) handleAnalyzeFlowChange(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
	lookback := request.GetInt("lookback_days", 14)
	if lookback < 8 {
		lookback = 8
	}
	if lookback > 60 {
		lookback = 60
	}
	if err := utils.ValidateSymbol(symbol); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	toDate := time.Now().Format("2006-01-02")
	half := lookback / 2
	curFrom := time.Now().AddDate(0, 0, -half).Format("2006-01-02")
	priorFrom := time.Now().AddDate(0, 0, -2*half).Format("2006-01-02")

	type fetch struct {
		ps  *broker.PositionSet
		err error
	}
	results := make([]fetch, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ps, err := bt.fetchPositions(symbol, curFrom, toDate)
		results[0] = fetch{ps, err}
	}()
	go func() {
		defer wg.Done()
		ps, err := bt.fetchPositions(symbol, priorFrom, curFrom)
		results[1] = fetch{ps, err}
	}()
	wg.Wait()

	if results[0].err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error fetching current window: %v", results[0].err)), nil
	}
	cur := buildFlowWindow(results[0].ps)

	var sb strings.Builder
	if results[1].err != nil || (len(results[1].ps.Data.Buyer) == 0 && len(results[1].ps.Data.Seller) == 0) {
		fmt.Fprintf(&sb, "%s — prior window (%s→%s) has no data; a change read needs two windows. Single-window read: analyze_broker_sentiment(symbol, period_days).\n", symbol, priorFrom, curFrom)
		return mcp.NewToolResultText(sb.String()), nil
	}
	prior := buildFlowWindow(results[1].ps)
	fc := analyzeFlowChange(cur, prior)

	fmt.Fprintf(&sb, "FLOW CHANGE — %s (last %dd vs prior %dd; %s→%s vs %s→%s)\n\n",
		symbol, half, half, cur.from, cur.to, prior.from, prior.to)

	if len(fc.newBuyers) > 0 {
		sb.WriteString("New buyers (no prior buys, active now):\n")
		for _, fb := range fc.newBuyers {
			fmt.Fprintf(&sb, "  - %s\n", flowLine(fb, cur.totalBuy))
		}
		sb.WriteString("\n")
	}
	if len(fc.accelBuyers) > 0 {
		sb.WriteString("Accelerating buyers (>=2x prior volume):\n")
		for _, fb := range fc.accelBuyers {
			fmt.Fprintf(&sb, "  - %s\n", flowLine(fb, cur.totalBuy))
		}
		sb.WriteString("\n")
	}
	if len(fc.persistentBuy) > 0 {
		sb.WriteString("Persistent buyers (significant in both windows):\n")
		for _, fb := range fc.persistentBuy {
			fmt.Fprintf(&sb, "  - %s\n", flowLine(fb, cur.totalBuy))
		}
		sb.WriteString("\n")
	}
	if len(fc.newSellers) > 0 {
		sb.WriteString("New sellers (no prior sells, active now):\n")
		for _, fb := range fc.newSellers {
			fmt.Fprintf(&sb, "  - %s\n", flowSellLine(fb, cur.totalSell))
		}
		sb.WriteString("\n")
	}
	if len(fc.accelSellers) > 0 {
		sb.WriteString("Accelerating sellers (>=2x prior volume):\n")
		for _, fb := range fc.accelSellers {
			fmt.Fprintf(&sb, "  - %s\n", flowSellLine(fb, cur.totalSell))
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "Flow: buy volume %+.0f%% vs prior | sell volume %+.0f%% vs prior\n", fc.buyDeltaPct, fc.sellDeltaPct)
	fmt.Fprintf(&sb, "Direction (holding vs released): %+.0f shares accumulated net (prior %+.0f) | %+.0f released (prior %+.0f)\n",
		cur.dirQty(), prior.dirQty(), -cur.distQty, -prior.distQty)
	fmt.Fprintf(&sb, "Concentration: top-5 buyers hold %.0f%% of buys (prior %.0f%%)\n", cur.top5Share*100, prior.top5Share*100)
	fmt.Fprintf(&sb, "Active trading days: %d now / %d prior\n", cur.days, prior.days)

	m := results[0].ps.Meta
	if m.ConfidenceLabel != "" {
		fmt.Fprintf(&sb, "Model confidence: %s (%d/100)\n", m.ConfidenceLabel, m.Confidence.Int())
	}
	for _, w := range m.Warnings {
		fmt.Fprintf(&sb, "⚠ %s\n", w)
	}

	sb.WriteString("\nBRANCHES (pick the edge whose condition matches the facts):\n")
	for _, br := range branchesFor(fc, cur, prior) {
		fmt.Fprintf(&sb, "  • %s\n", br)
	}
	return mcp.NewToolResultText(sb.String()), nil
}
