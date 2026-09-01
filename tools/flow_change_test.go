package tools

import (
	"strings"
	"testing"
)

func mkWindow(buys, sells map[int64]float64, from, to string, days int) flowWindowData {
	return mkWindowDir(buys, sells, nil, nil, from, to, days)
}

func mkWindowDir(buys, sells, acc, dist map[int64]float64, from, to string, days int) flowWindowData {
	w := flowWindowData{from: from, to: to, days: days}
	for id, b := range buys {
		fb := flowBroker{id: id, buy: b}
		w.brokers = append(w.brokers, fb)
		w.totalBuy += b
	}
	for id, s := range sells {
		found := false
		for i := range w.brokers {
			if w.brokers[i].id == id {
				w.brokers[i].sell = s
				found = true
			}
		}
		if !found {
			w.brokers = append(w.brokers, flowBroker{id: id, sell: s})
		}
		w.totalSell += s
	}
	for _, q := range acc {
		w.accQty += q
	}
	for _, q := range dist {
		w.distQty += q
	}
	w.net = w.totalBuy - w.totalSell
	return w
}

// TestNewBuyerDetected: broker 5 was absent in the prior window, then takes
// 30% of current buys -> must land in newBuyers and trigger the hands branch.
func TestNewBuyerDetected(t *testing.T) {
	prior := mkWindow(map[int64]float64{1: 1000, 2: 900}, map[int64]float64{1: 1500, 2: 800}, "2026-07-01", "2026-07-07", 5)
	cur := mkWindow(map[int64]float64{1: 800, 2: 700, 5: 700}, map[int64]float64{1: 900, 2: 600, 5: 50}, "2026-07-08", "2026-07-14", 5)
	fc := analyzeFlowChange(cur, prior)

	if len(fc.newBuyers) != 1 || fc.newBuyers[0].id != 5 {
		t.Fatalf("newBuyers = %+v, want broker 5", fc.newBuyers)
	}
	b := strings.Join(branchesFor(fc, cur, prior), "\n")
	if !strings.Contains(b, "analyze_broker_sentiment") {
		t.Errorf("branches missing persistence check: %q", b)
	}
	if !strings.Contains(b, "get_news(company=") {
		t.Errorf("branches missing catalyst check: %q", b)
	}
}

// TestAccelerationBranch: no new names but buy volume doubles with positive
// accumulation direction -> acceleration branch, no new-buyer claim.
func TestAccelerationBranch(t *testing.T) {
	prior := mkWindowDir(map[int64]float64{1: 500, 2: 500}, map[int64]float64{1: 600, 2: 600},
		map[int64]float64{1: 80}, nil, "2026-07-01", "2026-07-07", 5)
	cur := mkWindowDir(map[int64]float64{1: 1100, 2: 1100}, map[int64]float64{1: 800, 2: 900},
		map[int64]float64{1: 400}, nil, "2026-07-08", "2026-07-14", 5)
	fc := analyzeFlowChange(cur, prior)

	if len(fc.newBuyers) != 0 {
		t.Fatalf("newBuyers = %+v, want none (same hands)", fc.newBuyers)
	}
	if len(fc.accelBuyers) != 2 {
		t.Fatalf("accelBuyers = %+v, want 2", fc.accelBuyers)
	}
	b := strings.Join(branchesFor(fc, cur, prior), "\n")
	if !strings.Contains(b, "analyze_broker_sentiment") {
		t.Errorf("acceleration should branch to persistence check: %q", b)
	}
}

// TestDistributionFlip: direction (holding-released) flips positive->negative
// -> distribution branch.
func TestDistributionFlip(t *testing.T) {
	prior := mkWindowDir(map[int64]float64{1: 1000, 2: 1000}, map[int64]float64{1: 800, 2: 700},
		map[int64]float64{1: 400}, map[int64]float64{1: 100}, "2026-07-01", "2026-07-07", 5)
	cur := mkWindowDir(map[int64]float64{1: 500, 2: 600}, map[int64]float64{1: 1200, 2: 1300},
		map[int64]float64{1: 60}, map[int64]float64{1: 900}, "2026-07-08", "2026-07-14", 5)
	fc := analyzeFlowChange(cur, prior)

	if fc.netPrior <= 0 || fc.netNow >= 0 {
		t.Fatalf("expected direction flip positive->negative, got %+.0f -> %+.0f", fc.netPrior, fc.netNow)
	}
	b := strings.Join(branchesFor(fc, cur, prior), "\n")
	if !strings.Contains(b, "get_price_history") {
		t.Errorf("flip should branch to the price tape: %q", b)
	}
}

// TestQuiet: no activity either window -> no-case branch.
func TestQuiet(t *testing.T) {
	prior := mkWindow(nil, nil, "2026-07-01", "2026-07-07", 0)
	cur := mkWindow(nil, nil, "2026-07-08", "2026-07-14", 0)
	fc := analyzeFlowChange(cur, prior)
	b := strings.Join(branchesFor(fc, cur, prior), "\n")
	if !strings.Contains(b, "no case") {
		t.Errorf("quiet windows should branch to no-case: %q", b)
	}
}

// TestPersistence: same two hands both windows, stable volume -> persistent,
// and with sellers present too it's a standoff branch.
func TestPersistence(t *testing.T) {
	prior := mkWindow(map[int64]float64{1: 900, 2: 900}, map[int64]float64{1: 800, 2: 800}, "2026-07-01", "2026-07-07", 5)
	cur := mkWindow(map[int64]float64{1: 950, 2: 850}, map[int64]float64{1: 850, 2: 800}, "2026-07-08", "2026-07-14", 5)
	fc := analyzeFlowChange(cur, prior)

	if len(fc.newBuyers) != 0 {
		t.Fatalf("newBuyers = %+v, want none", fc.newBuyers)
	}
	if len(fc.persistentBuy) != 2 {
		t.Fatalf("persistentBuy = %+v, want 2", fc.persistentBuy)
	}
	b := strings.Join(branchesFor(fc, cur, prior), "\n")
	if !strings.Contains(b, "lookback_days=28") {
		t.Errorf("standoff should branch to a wider window: %q", b)
	}
}
