package company

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client fetches company fundamentals and corporate actions from the LLNMP API
// (LaganiLab's SSR-embedded JSON — no auth required).
type Client struct {
	BaseURL    string
	UserAgent  string
	HTTPClient *http.Client
}

// NewClient creates a company data client pointing at the LLNMP endpoint.
func NewClient() *Client {
	return &Client{
		BaseURL:   getEnv("LLNMP_API_URL", "https://laganilab.com/wp-json/llnmp/v1"),
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:147.0) Gecko/20100101 Firefox/147.0",
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

// CompanyData is the top-level response from /llnmp/v1/instrument?type=company.
type CompanyData struct {
	Type              string              `json:"type"`
	Key               string              `json:"key"`
	Title             string              `json:"title"`
	Subtitle          string              `json:"subtitle"`
	Facts             []Fact              `json:"facts"`
	Fundamentals      []FundamentalItem  `json:"fundamentals"`
	FundamentalTrends *FundamentalTrend  `json:"fundamentalTrends"`
	Timeline          []TimelineEvent    `json:"timeline"`
	BrokerURL         string              `json:"brokerUrl"`
	EntityID          interface{}         `json:"entityId"`
	FundamentalsURL   string              `json:"fundamentalsUrl"`
	Chart             []json.RawMessage   `json:"chart"`
}

type Fact struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

// FundamentalItem is one sector-specific metric for a given period.
type FundamentalItem struct {
	Key    string      `json:"key"`
	Label  string      `json:"label"`
	Value  interface{} `json:"value"`
	Format string      `json:"format"`
	Period string      `json:"period"`
}

// FundamentalTrend holds 5-period lookback for EPS, NetProfit, NetInterestIncome.
type FundamentalTrend struct {
	Periods   []TrendPeriod `json:"periods"`
	Metrics   []TrendMetric `json:"metrics"`
	Source    string        `json:"source"`
	UpdatedAt string        `json:"updatedAt"`
	Note      string        `json:"note"`
}

// TrendPeriod identifies one fiscal quarter in a trend series.
type TrendPeriod struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	FiscalYear string `json:"fiscalYear"`
	Quarter    string `json:"quarter"`
}

// TrendMetric is one metric across 5 periods (e.g. EPS: [25.05, 26.34, ...]).
type TrendMetric struct {
	Role      string      `json:"role"`
	Key       string      `json:"key"`
	Label     string      `json:"label"`
	Values    []FlexFloat `json:"values"`
	Format    string      `json:"format"`
	Available bool        `json:"available"`
}

// TimelineEvent is a corporate action or milestone (bonus, dividend, right, FPO, listing).
type TimelineEvent struct {
	Date          string         `json:"date"`
	Title         string         `json:"title"`
	Type          string         `json:"type"`
	Importance    int            `json:"importance"`
	Description   string         `json:"description"`
	ActionDetails *ActionDetails `json:"actionDetails,omitempty"`
}

// ActionDetails contains structured corporate action metadata.
type ActionDetails struct {
	Percent    FlexFloat `json:"percent"`
	FiscalYear string    `json:"fiscalYear"`
	Source     string    `json:"source"`
}

// FlexFloat accepts both JSON number and string (for resilience).
type FlexFloat float64

func (f *FlexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		v, err := parseFloat(str)
		if err != nil {
			*f = 0
			return nil
		}
		*f = FlexFloat(v)
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		*f = 0
		return nil
	}
	*f = FlexFloat(v)
	return nil
}

func (f FlexFloat) Float() float64 { return float64(f) }

func parseFloat(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

// GetCompanyData fetches the full company profile from LLNMP.
func (c *Client) GetCompanyData(symbol string) (*CompanyData, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	url := fmt.Sprintf("%s/instrument?type=company&key=%s", c.BaseURL, symbol)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", symbol, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var data CompanyData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing %s response: %w", symbol, err)
	}
	return &data, nil
}

// FactsMap converts the facts slice into a map for easy lookup.
func (d *CompanyData) FactsMap() map[string]string {
	m := make(map[string]string, len(d.Facts))
	for _, f := range d.Facts {
		m[strings.ToLower(f.Label)] = fmt.Sprintf("%v", f.Value)
	}
	return m
}

// FundamentalsByPeriod groups fundamentals by their period label.
func (d *CompanyData) FundamentalsByPeriod() map[string][]FundamentalItem {
	m := make(map[string][]FundamentalItem)
	for _, fi := range d.Fundamentals {
		period := fi.Period
		if period == "" {
			period = "current"
		}
		m[period] = append(m[period], fi)
	}
	return m
}

// CorporateActions filters timeline to meaningful corporate events.
func (d *CompanyData) CorporateActions() []TimelineEvent {
	var actions []TimelineEvent
	for _, e := range d.Timeline {
		switch e.Type {
		case "bonus", "cash_dividend", "right", "fpo", "merger", "acquisition_share_addition", "company_action", "listing":
			actions = append(actions, e)
		}
	}
	return actions
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
