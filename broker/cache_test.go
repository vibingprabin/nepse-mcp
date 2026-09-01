package broker

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPositionSetCached(t *testing.T) {
	c := NewClient()
	ctx := context.Background()
	const symbol = "SBL"
	const from, to = "2026-07-01", "2026-07-31"

	start := time.Now()
	a, err := c.GetPositionSet(ctx, symbol, from, to)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	first := time.Since(start)

	start = time.Now()
	b, err := c.GetPositionSet(ctx, symbol, from, to)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	second := time.Since(start)

	fmt.Printf("cold: %s | warm: %s\n", first.Round(time.Millisecond), second.Round(time.Millisecond))
	if second > 500*time.Millisecond {
		t.Fatalf("cache not working: warm call took %s", second)
	}
	if a != b {
		t.Fatal("cached call returned a different pointer")
	}
	if len(a.Data.TotalHolding) != len(b.Data.TotalHolding) {
		t.Fatalf("row count changed: %d vs %d", len(a.Data.TotalHolding), len(b.Data.TotalHolding))
	}
	fmt.Printf("holding rows: %d — identical, cached\n", len(a.Data.TotalHolding))
}
