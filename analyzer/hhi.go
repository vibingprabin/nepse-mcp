package analyzer

import (
	"sort"

	"vibinprabin/nepse-mcp/broker"
)

// HHI computes the Herfindahl-Hirschman Index over positive selector values.
func HHI(entries []broker.BrokerEntry, sel func(broker.BrokerEntry) float64) float64 {
	var quantities []float64
	var total float64
	for _, e := range entries {
		if q := sel(e); q > 0 {
			quantities = append(quantities, q)
			total += q
		}
	}
	if total == 0 {
		return 0
	}
	hhi := 0.0
	for _, q := range quantities {
		share := q / total
		hhi += share * share
	}
	return hhi
}

// Gini computes the Gini coefficient over positive selector values.
// 0 = perfectly spread across brokers, 1 = one broker holds everything.
func Gini(entries []broker.BrokerEntry, sel func(broker.BrokerEntry) float64) float64 {
	var values []float64
	var total float64
	for _, e := range entries {
		if v := sel(e); v > 0 {
			values = append(values, v)
			total += v
		}
	}
	n := len(values)
	if n < 2 || total == 0 {
		return 0
	}
	sort.Float64s(values)
	var cum float64
	for i, v := range values {
		cum += float64(i+1) * v
	}
	return (2*cum)/(float64(n)*total) - float64(n+1)/float64(n)
}

// NetQty selects net quantity (accumulation view).
func NetQty(e broker.BrokerEntry) float64 { return float64(e.NetQuantity) }

func ReleasedQty(e broker.BrokerEntry) float64 { return float64(e.ReleasedQuantity) }

func BoughtQty(e broker.BrokerEntry) float64 { return float64(e.TotalBought) }

func SoldQty(e broker.BrokerEntry) float64 { return float64(e.TotalSold) }

func AdjQty(e broker.BrokerEntry) float64 { return e.AdjustedQuantity.Float() }
