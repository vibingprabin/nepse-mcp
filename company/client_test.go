package company

import (
	"testing"
)

func TestGetCompanyDataNABIL(t *testing.T) {
	c := NewClient()
	data, err := c.GetCompanyData("NABIL")
	if err != nil {
		t.Fatalf("GetCompanyData failed: %v", err)
	}
	if data.Key != "NABIL" {
		t.Fatalf("expected NABIL, got %s", data.Key)
	}
	if len(data.Facts) == 0 {
		t.Fatal("no facts returned")
	}
	fm := data.FactsMap()
	if fm["ltp"] == "" {
		t.Fatal("LTP fact missing")
	}
	if len(data.Fundamentals) == 0 {
		t.Fatal("no fundamentals returned")
	}
	if data.FundamentalTrends != nil {
		if len(data.FundamentalTrends.Metrics) == 0 {
			t.Log("fundamental trends have no metrics (possible)")
		}
	}
	if len(data.CorporateActions()) == 0 {
		t.Fatal("no corporate actions returned")
	}
	trendsAvail := 0
	if data.FundamentalTrends != nil {
		trendsAvail = 1
	}
	t.Logf("NABIL profile OK: %d facts, %d fundamentals, trends:%v, %d actions",
		len(data.Facts), len(data.Fundamentals), trendsAvail, len(data.CorporateActions()))
}

func TestGetCompanyDataInvalid(t *testing.T) {
	c := NewClient()
	_, err := c.GetCompanyData("INVALIDXYZ")
	if err == nil {
		t.Fatal("expected error for invalid symbol")
	}
	t.Logf("Got expected error: %v", err)
}

func TestCorporateActionsFilter(t *testing.T) {
	c := NewClient()
	data, err := c.GetCompanyData("SBI")
	if err != nil {
		t.Fatalf("GetCompanyData failed for SBI: %v", err)
	}
	actions := data.CorporateActions()
	t.Logf("SBI: %d facts, %d fundamentals, %d corporate actions",
		len(data.Facts), len(data.Fundamentals), len(actions))
	for _, a := range actions {
		t.Logf("  %s %s: %s", a.Date, a.Type, a.Title)
	}
}


