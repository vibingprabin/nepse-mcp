package tools

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"vibinprabin/nepse-mcp/broker"
)

func netsInView(t *testing.T, ps *broker.PositionSet, view string) []int64 {
	t.Helper()
	var b strings.Builder
	writeFloorsheetView(&b, ps, view, 0)
	var nets []int64
	for _, line := range strings.Split(b.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|---") {
			continue
		}
		fields := strings.Split(strings.Trim(line, "|"), "|")
		if len(fields) < 5 || strings.TrimSpace(fields[0]) == "Broker" {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64)
		if err != nil {
			continue
		}
		nets = append(nets, n)
	}
	return nets
}

func TestFloorsheetViewBuyerSellerSort(t *testing.T) {
	ps := &broker.PositionSet{
		Data: broker.FloorsheetAllData{
			Buyer:  []broker.BrokerEntry{{BrokerID: 10, NetQuantity: 500}, {BrokerID: 20, NetQuantity: 200}, {BrokerID: 30, NetQuantity: 100}},
			Seller: []broker.BrokerEntry{{BrokerID: 10, NetQuantity: -500}, {BrokerID: 20, NetQuantity: -100}, {BrokerID: 30, NetQuantity: -700}},
		},
		Meta: broker.FloorsheetMeta{Symbol: "TEST"},
	}

	if nets := netsInView(t, ps, "buyer"); !reflect.DeepEqual(nets, []int64{500, 200, 100}) {
		t.Errorf("buyer view nets = %v, want [500 200 100] (accumulators first)", nets)
	}
	if nets := netsInView(t, ps, "seller"); !reflect.DeepEqual(nets, []int64{-700, -500, -100}) {
		t.Errorf("seller view nets = %v, want [-700 -500 -100] (biggest distributors first)", nets)
	}

	buyRows := broker.RowsForView(ps, "buyer")
	sellRows := broker.RowsForView(ps, "seller")
	if len(buyRows) == 0 || len(sellRows) == 0 {
		t.Fatal("expected buyer and seller rows in the data model")
	}
	if reflect.DeepEqual(buyRows, sellRows) {
		t.Error("buyer and seller rows must be distinct in the data model")
	}
}
