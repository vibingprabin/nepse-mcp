package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vibinprabin/nepse-mcp/analyzer"
	"vibinprabin/nepse-mcp/broker"
	"vibinprabin/nepse-mcp/utils"
)

type BrokerTools struct {
	brokerClient *broker.Client
}

func NewBrokerTools() *BrokerTools {
	return &BrokerTools{
		brokerClient: broker.NewClient(),
	}
}

func (bt *BrokerTools) Register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_broker_floorsheet",
		mcp.WithDescription("Broker positions in ONE call. view: summary(default)|holding|released|buyer|seller|inferred(ALL-TIME as of to_date, from_date ignored; breakeven+confidence). buyer lists net accumulators (sorted by net qty desc), seller lists net distributors (sorted by net qty asc, biggest sellers first). top_n rows (10, 0=all). Guides: 'broker' topic."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
		mcp.WithString("from_date", mcp.Description("Start YYYY-MM-DD (optional; default: 30 days before to_date; ignored for view='inferred')")),
		mcp.WithString("to_date", mcp.Description("End YYYY-MM-DD (optional; default: today)")),
		mcp.WithString("view", mcp.Description("summary|holding|released|buyer|seller|inferred")),
		mcp.WithNumber("top_n", mcp.Description("Rows per view (10, 0=all)")),
	), bt.handleGetBrokerFloorsheet)

	s.AddTool(mcp.NewTool("analyze_broker_sentiment",
		mcp.WithDescription("Single-stock broker flow & concentration: read first (one-sided accumulation/distribution vs churn/standoff), then Gini x5, HHI/Eff#, cohort purity, underwater holders. Run 7d/14d/30d. show_brokers names top accumulators. Combine with: analyze_flow_change (window change), get_broker_floorsheet(view='inferred') (cost basis), get_price_history (where in the move this stands)."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol")),
		mcp.WithNumber("period_days", mcp.Description("Lookback days (default: 7)")),
		mcp.WithBoolean("show_brokers", mcp.Description("Name top 5 accumulators with % share")),
	), bt.handleAnalyzeBrokerSentiment)

	s.AddTool(mcp.NewTool("get_market_sentiment",
		mcp.WithDescription("NEPSE Fear & Greed 0-100: score, label, 7 components, breadth, history. history_days default 30 (max 120). index_key for sector sentiment."),
		mcp.WithNumber("history_days", mcp.Description("Score history days (30, max 120)")),
		mcp.WithString("index_key", mcp.Description("Index key (default: nepse composite)")),
	), bt.handleGetMarketSentiment)
}

func (bt *BrokerTools) fetchPositions(symbol, fromDate, toDate string) (*broker.PositionSet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return bt.brokerClient.GetPositionSet(ctx, symbol, fromDate, toDate)
}

func sortBy(entries []broker.BrokerEntry, key func(broker.BrokerEntry) float64, ascending bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		if ascending {
			return key(entries[i]) < key(entries[j])
		}
		return key(entries[i]) > key(entries[j])
	})
}

func (bt *BrokerTools) handleGetBrokerFloorsheet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
	fromDate := request.GetString("from_date", "")
	toDate := request.GetString("to_date", "")
	view := strings.ToLower(strings.TrimSpace(request.GetString("view", "summary")))
	topN := request.GetInt("top_n", 10)

	if err := utils.ValidateSymbol(symbol); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if toDate == "" {
		toDate = time.Now().Format("2006-01-02")
	}
	if fromDate == "" {
		fromDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if _, _, err := utils.ValidateDateRange(fromDate, toDate, 0); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid date range: %v", err)), nil
	}

	ps, err := bt.fetchPositions(symbol, fromDate, toDate)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	var sb strings.Builder
	if view == "summary" || view == "" {
		writeFloorsheetSummary(&sb, ps, topN)
	} else {
		writeFloorsheetView(&sb, ps, view, topN)
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func writeMetaBlock(sb *strings.Builder, ps *broker.PositionSet) {
	m := ps.Meta
	fmt.Fprintf(sb, "%s %s → %s | model:%s | confidence:%s(%d)", m.Symbol, m.FromDate, m.ToDate, m.Model, m.ConfidenceLabel, m.Confidence.Int())
	if ps.Source != "all" {
		sb.WriteString(" | degraded(partial views)")
	}
	sb.WriteString("\n")
	if m.CoverageStart != "" {
		fmt.Fprintf(sb, "coverage from %s (txns:%s) | actions: bonus %d, right %d, div %d, merger %d\n",
			m.CoverageStart, utils.FormatVolume(m.TransactionCount.Int()), m.ActionCounts.Bonus, m.ActionCounts.Right, m.ActionCounts.CashDividend, m.ActionCounts.Merger)
	}
	for _, w := range m.Warnings {
		fmt.Fprintf(sb, "⚠ %s\n", w)
	}
	sb.WriteString("\n")
}

func capRows(rows []broker.BrokerEntry, topN int) []broker.BrokerEntry {
	if topN > 0 && topN < len(rows) {
		return rows[:topN]
	}
	return rows
}

func writeFloorsheetSummary(sb *strings.Builder, ps *broker.PositionSet, topN int) {
	writeMetaBlock(sb, ps)
	if topN <= 0 || topN > 5 {
		topN = 5
	}

	acc := append([]broker.BrokerEntry(nil), ps.Data.TotalHolding...)
	sortBy(acc, func(e broker.BrokerEntry) float64 { return float64(e.NetQuantity) }, false)
	rel := append([]broker.BrokerEntry(nil), ps.Data.Released...)
	sortBy(rel, func(e broker.BrokerEntry) float64 { return float64(e.ReleasedQuantity) }, false)
	buy := append([]broker.BrokerEntry(nil), ps.Data.Buyer...)
	sortBy(buy, func(e broker.BrokerEntry) float64 { return float64(e.TotalBought) }, false)
	sel := append([]broker.BrokerEntry(nil), ps.Data.Seller...)
	sortBy(sel, func(e broker.BrokerEntry) float64 { return float64(e.TotalSold) }, false)

	writeMiniTable(sb, "Top Accumulators (net qty)", acc, topN, func(e broker.BrokerEntry) string {
		return utils.FormatVolume(e.NetQuantity.Int())
	})
	writeMiniTable(sb, "Top Distributors (released qty)", rel, topN, func(e broker.BrokerEntry) string {
		return utils.FormatVolume(e.ReleasedQuantity.Int())
	})
	writeMiniTable(sb, "Top Buyers (bought qty)", buy, topN, func(e broker.BrokerEntry) string {
		return utils.FormatVolume(e.TotalBought.Int())
	})
	writeMiniTable(sb, "Top Sellers (sold qty)", sel, topN, func(e broker.BrokerEntry) string {
		return utils.FormatVolume(e.TotalSold.Int())
	})

	sb.WriteString("\nviews: holding=released-filtered accumulators, inferred/positions=current positions with cost basis & confidence.\n")
}

func writeMiniTable(sb *strings.Builder, title string, rows []broker.BrokerEntry, n int, val func(broker.BrokerEntry) string) {
	fmt.Fprintf(sb, "**%s**\n", title)
	if len(rows) == 0 {
		sb.WriteString("- none\n\n")
		return
	}
	for _, e := range capRows(rows, n) {
		fmt.Fprintf(sb, "- %s — %s (%dd)\n", broker.BrokerLabel(e.BrokerID.Int()), val(e), e.TradingDays.Int())
	}
	sb.WriteString("\n")
}

func writeFloorsheetView(sb *strings.Builder, ps *broker.PositionSet, view string, topN int) {
	if view == "inferred" || view == "positions" {
		writePositionsView(sb, ps, topN)
		return
	}
	rows := broker.RowsForView(ps, view)
	writeMetaBlock(sb, ps)
	if len(rows) == 0 {
		fmt.Fprintf(sb, "%s: no %s data\n", ps.Meta.Symbol, view)
		return
	}
	sortBy(rows, func(e broker.BrokerEntry) float64 { return float64(e.NetQuantity) }, view == "seller")

	var totalNet int64
	var totalBuy, totalSell float64
	for _, e := range rows {
		totalNet += e.NetQuantity.Int()
		totalBuy += e.TotalBuyAmount.Float()
		totalSell += e.TotalSellAmount.Float()
	}
	fmt.Fprintf(sb, "%s view | %d brokers | net:%s | bought %s / sold %s\n\n",
		view, len(rows), utils.FormatVolume(totalNet), utils.FormatCurrency(totalBuy), utils.FormatCurrency(totalSell))
	sb.WriteString("| Broker | Bought | Sold | Net | Days |\n|---|---|---|---|---|\n")
	for _, e := range capRows(rows, topN) {
		fmt.Fprintf(sb, "| %s | %s | %s | %s | %d |\n",
			broker.BrokerLabel(e.BrokerID.Int()),
			utils.FormatVolume(e.TotalBought.Int()),
			utils.FormatVolume(e.TotalSold.Int()),
			utils.FormatVolume(e.NetQuantity.Int()),
			e.TradingDays.Int())
	}
}

func writePositionsView(sb *strings.Builder, ps *broker.PositionSet, topN int) {
	writeMetaBlock(sb, ps)
	rows := append([]broker.BrokerEntry(nil), ps.Data.Holding...)
	if len(rows) == 0 {
		rows = append([]broker.BrokerEntry(nil), ps.Data.TotalHolding...)
	}
	if len(rows) == 0 {
		sb.WriteString("no inferred position data\n")
		return
	}
	sortBy(rows, func(e broker.BrokerEntry) float64 { return e.MarketValue.Float() }, false)

	fmt.Fprintf(sb, "Inferred positions (%d brokers, sorted by market value):\n\n", len(rows))
	sb.WriteString("| Broker | AdjQty | Breakeven | MktValue | UnrealPL | DivRecvd | Conf |\n|---|---|---|---|---|---|---|\n")
	for _, e := range capRows(rows, topN) {
		be := "-"
		if e.BreakevenPrice != nil {
			be = utils.FormatNumber(e.BreakevenPrice.Float())
		}
		fmt.Fprintf(sb, "| %s | %s | %s | %s | %s | %s | %d |\n",
			broker.BrokerLabel(e.BrokerID.Int()),
			utils.FormatVolume(int64(e.AdjustedQuantity.Float())),
			be,
			utils.FormatCurrency(e.MarketValue.Float()),
			utils.FormatCurrency(e.UnrealizedPL.Float()),
			utils.FormatCurrency(e.CashDividend.Float()),
			e.Confidence.Int())
	}
	sb.WriteString("\nConf: per-broker model confidence (0-100). Breakeven '-' = untraceable cost basis.\n")
}

func (bt *BrokerTools) handleAnalyzeBrokerSentiment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
	days := request.GetInt("period_days", 7)
	showBrokers := request.GetBool("show_brokers", false)

	if err := utils.ValidateSymbol(symbol); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if days < 1 {
		days = 7
	}

	toDate := time.Now().Format("2006-01-02")
	fromDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	ps, err := bt.fetchPositions(symbol, fromDate, toDate)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	holdRows := broker.RowsForView(ps, "holding")
	relRows := broker.RowsForView(ps, "released")
	buyRows := broker.RowsForView(ps, "buyer")
	selRows := broker.RowsForView(ps, "seller")
	infRows := ps.Data.Holding
	if len(infRows) == 0 {
		infRows = ps.Data.TotalHolding
	}

	if len(holdRows) == 0 && len(relRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("%s: no broker activity in %dd", symbol, days)), nil
	}

	var totalAcc, totalDist float64
	var accBuy, accSell, distBuy, distSell float64
	type brokerInfo struct {
		id  int64
		qty float64
	}
	var accBrokers []brokerInfo
	for _, e := range holdRows {
		n := float64(e.NetQuantity)
		totalAcc += n
		accBuy += float64(e.TotalBought)
		accSell += float64(e.TotalSold)
		accBrokers = append(accBrokers, brokerInfo{e.BrokerID.Int(), n})
	}
	for _, e := range relRows {
		totalDist += float64(e.ReleasedQuantity)
		distBuy += float64(e.TotalBought)
		distSell += float64(e.TotalSold)
	}

	accPurity := 0.0
	if accBuy+accSell > 0 {
		accPurity = accBuy / (accBuy + accSell) * 100
	}
	distPurity := 0.0
	if distBuy+distSell > 0 {
		distPurity = distSell / (distBuy + distSell) * 100
	}

	hhi := analyzer.HHI(holdRows, analyzer.NetQty)
	effBrokers := 0.0
	if hhi > 0 {
		effBrokers = 1.0 / hhi
	}

	gAcc := analyzer.Gini(buyRows, analyzer.BoughtQty)
	gDist := analyzer.Gini(selRows, analyzer.SoldQty)
	gHold := analyzer.Gini(holdRows, analyzer.NetQty)
	gRel := analyzer.Gini(relRows, analyzer.ReleasedQty)
	gInf := analyzer.Gini(infRows, analyzer.AdjQty)

	underwater := 0
	for _, e := range infRows {
		if e.BreakevenPrice != nil && e.AdjustedQuantity.Float() > 0 {
			if e.BreakevenPrice.Float() > e.MarketValue.Float()/e.AdjustedQuantity.Float() {
				underwater++
			}
		}
	}

	var sb strings.Builder
	read := "dispersed churn — no strong directional edge"
	switch {
	case accPurity > 60 && distPurity > 60:
		read = "two-sided standoff — conviction on BOTH sides (rotation between strong hands; no clean net edge)"
	case accPurity > 60:
		read = "one-sided accumulation — buyers dominate this window"
	case distPurity > 60:
		read = "one-sided distribution — sellers dominate this window"
	}
	fmt.Fprintf(&sb, "%s (%dd, %s → %s)\n", symbol, days, fromDate, toDate)
	fmt.Fprintf(&sb, "Read: %s (compare across 7/14/30d — persistent = material, short-range only = fleeting)\n", read)
	fmt.Fprintf(&sb, "Flow: Acc %.0f | Dist %.0f | Net %+.0f (position-model artifact — every buy has a sell)\n", totalAcc, totalDist, totalAcc-totalDist)
	fmt.Fprintf(&sb, "Cohort purity: acc %.0f%% buy-side | dist %.0f%% sell-side (>60%% one-sided conviction, ~50%% churn)\n", accPurity, distPurity)
	fmt.Fprintf(&sb, "Gini (0=dispersed, 1=one broker): acc:%.2f dist:%.2f holding:%.2f released:%.2f inferred:%.2f\n",
		gAcc, gDist, gHold, gRel, gInf)
	fmt.Fprintf(&sb, "HHI(holding):%.3f | Eff#:%.1f\n", hhi, effBrokers)
	if len(infRows) > 0 {
		fmt.Fprintf(&sb, "Underwater: %d/%d inferred holders (below cost — normal for any name off its highs; breakevens are one input, trend and volume decide).\n", underwater, len(infRows))
	}

	m := ps.Meta
	if m.ConfidenceLabel != "" {
		fmt.Fprintf(&sb, "Model confidence: %s (%d/100)\n", m.ConfidenceLabel, m.Confidence.Int())
	}
	for _, w := range m.Warnings {
		fmt.Fprintf(&sb, "⚠ %s\n", w)
	}

	if showBrokers && len(accBrokers) > 0 {
		sort.Slice(accBrokers, func(i, j int) bool { return accBrokers[i].qty > accBrokers[j].qty })
		sb.WriteString("Top accumulators: ")
		n := 5
		if len(accBrokers) < n {
			n = len(accBrokers)
		}
		for i := 0; i < n; i++ {
			pct := 0.0
			if totalAcc > 0 {
				pct = (accBrokers[i].qty / totalAcc) * 100
			}
			fmt.Fprintf(&sb, "%s:%.0f%% ", broker.BrokerLabel(accBrokers[i].id), pct)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nTip: compare 7d / 14d / 30d — a pattern present at all timeframes is material; one visible only at short range is fleeting. Cost basis: get_broker_floorsheet(..., view='inferred')")
	sb.WriteString("\n\nBRANCHES (pick the edge whose condition matches the facts):\n")
	switch {
	case accPurity > 60 && distPurity > 60:
		sb.WriteString("  • two-sided conviction -> analyze_flow_change(symbol, lookback_days=28): is the standoff older than the window?\n")
		sb.WriteString("  • -> get_news(company=symbol): what catalyst would resolve it?\n")
	case accPurity > 60:
		sb.WriteString("  • one-sided accumulation -> analyze_flow_change(symbol, lookback_days=14): new hand or persistent?\n")
		sb.WriteString("  • -> get_news(company=symbol): the catalyst check\n")
	case distPurity > 60:
		sb.WriteString("  • one-sided distribution -> get_price_history(symbol, limit=30, include_analysis): breakdown or shakeout?\n")
		sb.WriteString("  • -> get_news(query=symbol, category='announcement'): the trigger (rights, promoter sale, results)\n")
	default:
		sb.WriteString("  • dispersed churn -> analyze_flow_change(symbol, lookback_days=14): has anything changed vs the prior window?\n")
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func (bt *BrokerTools) handleGetMarketSentiment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	historyDays := request.GetInt("history_days", 30)
	if historyDays < 1 {
		historyDays = 30
	}
	if historyDays > 120 {
		historyDays = 120
	}
	indexKey := strings.TrimSpace(request.GetString("index_key", ""))

	fgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fg, err := bt.brokerClient.GetFearGreed(fgCtx, indexKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}
	if !fg.Success {
		return mcp.NewToolResultText("fear/greed endpoint returned success=false"), nil
	}

	l := fg.Latest
	var sb strings.Builder
	fmt.Fprintf(&sb, "## NEPSE Fear & Greed — %s (%s)\n\n", l.IndexName, l.TradeDate)
	fmt.Fprintf(&sb, "**Score: %.1f / 100 — %s**\n\n", l.IndexScore.Float(), l.Label)

	if len(l.Components) > 0 {
		sb.WriteString("### Components\n")
		sb.WriteString("| Component | Score |\n|---|---|\n")
		keys := make([]string, 0, len(l.Components))
		for k := range l.Components {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "| %s | %.1f |\n", k, l.Components[k].Float())
		}
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "### Breadth\nAdvancers %d / Decliners %d / Unchanged %d | UpVol %s / DownVol %s\n",
		l.Advancers.Int(), l.Decliners.Int(), l.Unchanged.Int(),
		utils.FormatVolume(l.UpVolume.Int()), utils.FormatVolume(l.DownVolume.Int()))
	fmt.Fprintf(&sb, "Turnover %s | Txns %s | Shares %s\n\n",
		utils.FormatCurrency(l.TotalTurnover.Float()),
		utils.FormatVolume(l.TotalTransactions.Int()),
		utils.FormatVolume(l.TotalTradedShares.Int()))

	if len(fg.History) > 1 {
		start := len(fg.History) - historyDays
		if start < 0 {
			start = 0
		}
		recent := fg.History[start:]
		fmt.Fprintf(&sb, "### Last %d readings\n", len(recent))
		for i, h := range recent {
			fmt.Fprintf(&sb, "%s:%.0f(%s)", h.TradeDate[5:], h.IndexScore.Float(), shortLabel(h.Label))
			if i < len(recent)-1 {
				sb.WriteString(" ")
			}
			if (i+1)%6 == 0 {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nScale: 0-24 Extreme Fear, 25-44 Fear, 45-55 Neutral, 56-75 Greed, 76-100 Extreme Greed. Guide: get_usage_guide('sentiment')")
	return mcp.NewToolResultText(sb.String()), nil
}

func shortLabel(label string) string {
	switch strings.ToLower(label) {
	case "extreme fear":
		return "EF"
	case "fear":
		return "F"
	case "neutral":
		return "N"
	case "greed":
		return "G"
	case "extreme greed":
		return "EG"
	default:
		if len(label) > 3 {
			return label[:3]
		}
		return label
	}
}
