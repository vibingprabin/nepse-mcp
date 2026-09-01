package company

import (
	"fmt"
	"testing"
	"time"
)

func TestGetCompanyDataCached(t *testing.T) {
	c := NewClient()
	start := time.Now()
	d, err := c.GetCompanyData("NABIL")
	if err != nil {
		t.Fatal(err)
	}
	cold := time.Since(start)

	start = time.Now()
	d2, err := c.GetCompanyData("NABIL")
	if err != nil {
		t.Fatal(err)
	}
	warm := time.Since(start)

	fmt.Printf("cold: %s | warm: %s\n", cold.Round(time.Millisecond), warm.Round(time.Millisecond))
	if d2 == nil || d2.Key == "" {
		t.Fatal("nil data on cached call")
	}
	if warm > 500*time.Millisecond {
		t.Fatalf("cache not working: warm call took %s", warm)
	}
	if d != d2 {
		t.Fatal("cached call returned a different pointer")
	}
}
