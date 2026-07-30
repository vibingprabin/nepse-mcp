package analyzer

import "vibinprabin/nepse-mcp/broker"

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

// NetQty selects net quantity (accumulation view).
func NetQty(e broker.BrokerEntry) float64 { return float64(e.NetQuantity) }
