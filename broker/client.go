package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	noncePattern1 = regexp.MustCompile(`(?i)["']X-WP-Nonce["']\s*:\s*["']([a-f0-9]+)["']`)
	noncePattern2 = regexp.MustCompile(`(?i)nonce\s*:\s*["']([a-f0-9]+)["']`)
	noncePattern3 = regexp.MustCompile(`(?i)wpApiSettings\s*=\s*{[^}]*nonce["']\s*:\s*["']([a-f0-9]+)`)
	noncePattern4 = regexp.MustCompile(`(?i)"nonce":"([a-f0-9]+)"`)
	noncePattern5 = regexp.MustCompile(`(?i)lfData\s*=\s*\{[^}]*nonce["']\s*:\s*["']([a-f0-9]+)`)
)

var nonceGroup singleflight.Group

// requestSem caps in-flight HTTP requests. Release must RECEIVE from the
// channel: the old code sent into it and deadlocked once the cap filled.
var requestSem = make(chan struct{}, 20)

const (
	nonceLifespan         = 10 * time.Hour // WP nonces live 12-24h; refresh early
	companiesCacheTTL     = 1 * time.Hour
	positionCacheTTL      = 1 * time.Hour
	// Windows that ended before today are deterministic (verified live: the
	// floorsheet replay for a closed range never changes), so they can be held
	// far longer than a hot window whose last day is still trading.
	positionClosedWindowTTL = 24 * time.Hour
	defaultAttemptTimeout   = 45 * time.Second
	floorsheetTimeout       = 150 * time.Second // long-history type=all replays are slow
	maxRetries              = 3
)

// The WordPress API inconsistently returns the same field as a string on one
// endpoint and a number on another (verified live: fear-greed history scores
// arrive as strings, latest score as a number; broker rows flipped from
// strings to numbers in the v2 model). FlexFloat/FlexInt accept both plus null,
// so a server-side type flip never silently zeroes out our data again.
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
		str = strings.ReplaceAll(strings.TrimSpace(str), ",", "")
		v, err := strconv.ParseFloat(str, 64)
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

type FlexInt int64

func (f *FlexInt) UnmarshalJSON(b []byte) error {
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
		str = strings.ReplaceAll(strings.TrimSpace(str), ",", "")
		v, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			fv, ferr := strconv.ParseFloat(str, 64)
			if ferr != nil {
				*f = 0
				return nil
			}
			*f = FlexInt(int64(math.Round(fv)))
			return nil
		}
		*f = FlexInt(v)
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		*f = 0
		return nil
	}
	*f = FlexInt(int64(math.Round(v)))
	return nil
}

func (f FlexInt) Int() int64 { return int64(f) }

// BrokerEntry is one broker's position/activity row in the v2 model
// (format verified against live API 2026-07-29).
type BrokerEntry struct {
	BrokerID                FlexInt    `json:"broker_id"`
	TotalBought             FlexInt    `json:"total_bought"`
	TotalSold               FlexInt    `json:"total_sold"`
	TotalBuyAmount          FlexFloat  `json:"total_buy_amount"`
	TotalSellAmount         FlexFloat  `json:"total_sell_amount"`
	NetQuantity             FlexInt    `json:"net_quantity"`
	RawNetQuantity          FlexInt    `json:"raw_net_quantity"`
	AdjustedQuantity        FlexFloat  `json:"adjusted_quantity"`
	ReleasedQuantity        FlexInt    `json:"released_quantity"`
	FaceValue               FlexFloat  `json:"face_value"`
	BonusUnits              FlexFloat  `json:"bonus_units"`
	BonusTax                FlexFloat  `json:"bonus_tax"`
	RightUnits              FlexFloat  `json:"right_units"`
	RightCost               FlexFloat  `json:"right_cost"`
	CashDividendGross       FlexFloat  `json:"cash_dividend_gross"`
	CashDividendTax         FlexFloat  `json:"cash_dividend_tax"`
	CashDividend            FlexFloat  `json:"cash_dividend"`
	AdjustedCostBasis       FlexFloat  `json:"adjusted_cost_basis"`
	BreakevenPrice          *FlexFloat `json:"breakeven_price"`
	MarketValue             FlexFloat  `json:"market_value"`
	UnrealizedPL            FlexFloat  `json:"unrealized_pl"`
	TotalReturnPL           FlexFloat  `json:"total_return_pl"`
	PeriodBonusTax          FlexFloat  `json:"period_bonus_tax"`
	PeriodCashDividendGross FlexFloat  `json:"period_cash_dividend_gross"`
	PeriodCashDividendTax   FlexFloat  `json:"period_cash_dividend_tax"`
	PeriodCashDividend      FlexFloat  `json:"period_cash_dividend"`
	PeriodRightCost         FlexFloat  `json:"period_right_cost"`
	NetCashflow             FlexFloat  `json:"net_cashflow"`
	UnmatchedSellUnits      FlexFloat  `json:"unmatched_sell_units"`
	NettingConfidence       FlexFloat  `json:"netting_confidence"`
	Confidence              FlexInt    `json:"confidence"`
	TradingDays             FlexInt    `json:"trading_days"`
}

type ActionCounts struct {
	Bonus           int `json:"bonus"`
	Right           int `json:"right"`
	CashDividend    int `json:"cash_dividend"`
	FPOUnattributed int `json:"fpo_unattributed"`
	Merger          int `json:"merger"`
	NameChange      int `json:"name_change"`
}

type FloorsheetMeta struct {
	Symbol                    string       `json:"symbol"`
	FromDate                  string       `json:"from_date"`
	ToDate                    string       `json:"to_date"`
	Type                      string       `json:"type"`
	Count                     int          `json:"count"`
	Model                     string       `json:"model"`
	ModelLabel                string       `json:"model_label"`
	CoverageStart             string       `json:"coverage_start"`
	CoverageIsPartial         bool         `json:"coverage_is_partial"`
	InferredScope             string       `json:"inferred_scope"`
	LineageSymbols            []string     `json:"lineage_symbols"`
	CorporateActionsAvailable bool         `json:"corporate_actions_available"`
	ActionCounts              ActionCounts `json:"action_counts"`
	CashDividendTaxRate       FlexFloat    `json:"cash_dividend_tax_rate"`
	BonusShareTaxRate         FlexFloat    `json:"bonus_share_tax_rate"`
	RightSubscriptionRate     FlexFloat    `json:"right_subscription_rate"`
	DefaultFaceValue          FlexFloat    `json:"default_face_value"`
	TransactionCount          FlexInt      `json:"transaction_count"`
	Confidence                FlexInt      `json:"confidence"`
	ConfidenceLabel           string       `json:"confidence_label"`
	Warnings                  []string     `json:"warnings"`
}

// FloorsheetAllData holds the five broker views returned by one type=all call.
type FloorsheetAllData struct {
	TotalHolding []BrokerEntry `json:"total_holding"` // net accumulators (legacy "holding")
	Holding      []BrokerEntry `json:"holding"`       // inferred current positions
	Released     []BrokerEntry `json:"released"`      // net distributors
	Buyer        []BrokerEntry `json:"buyer"`
	Seller       []BrokerEntry `json:"seller"`
}

type floorsheetAllResponse struct {
	Success bool              `json:"success"`
	Data    FloorsheetAllData `json:"data"`
	Meta    FloorsheetMeta    `json:"meta"`
}

type floorsheetSingleResponse struct {
	Success bool          `json:"success"`
	Data    []BrokerEntry `json:"data"`
	Meta    struct {
		Symbol   string `json:"symbol"`
		FromDate string `json:"from_date"`
		ToDate   string `json:"to_date"`
		Type     string `json:"type"`
		Count    int    `json:"count"`
	} `json:"meta"`
}

// PositionSet is the normalized fetch result. Source records whether data came
// from one type=all call ("all") or the merged single-view fallback.
type PositionSet struct {
	Data   FloorsheetAllData
	Meta   FloorsheetMeta
	Source string
}

type CompanyList struct {
	Success bool `json:"success"`
	Data    []struct {
		Symbol   string `json:"symbol"`
		Name     string `json:"name"`
		Sector   string `json:"sector"`
		IsActive string `json:"is_active"`
	} `json:"data"`
}

type DateRange struct {
	Success bool `json:"success"`
	Data    struct {
		Symbol  string `json:"symbol"`
		MinDate string `json:"min_date"`
		MaxDate string `json:"max_date"`
	} `json:"data"`
}

type FearGreedResponse struct {
	Success       bool `json:"success"`
	SelectedIndex struct {
		Key       string `json:"key"`
		ID        int    `json:"id"`
		Name      string `json:"name"`
		WeightSet string `json:"weight_set"`
	} `json:"selected_index"`
	Indices []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		ID   int    `json:"id"`
	} `json:"indices"`
	Latest  FearGreedLatest         `json:"latest"`
	History []FearGreedHistoryPoint `json:"history"`
}

type FearGreedLatest struct {
	TradeDate         string               `json:"trade_date"`
	IndexScore        FlexFloat            `json:"index_score"`
	Label             string               `json:"label"`
	IndexKey          string               `json:"index_key"`
	IndexName         string               `json:"index_name"`
	IndexID           int                  `json:"index_id"`
	NepseClose        FlexFloat            `json:"nepse_close"`
	NepseTurnover     FlexFloat            `json:"nepse_turnover"`
	TotalTurnover     FlexFloat            `json:"total_turnover"`
	TotalTransactions FlexInt              `json:"total_transactions"`
	TotalTradedShares FlexInt              `json:"total_traded_shares"`
	Advancers         FlexInt              `json:"advancers"`
	Decliners         FlexInt              `json:"decliners"`
	Unchanged         FlexInt              `json:"unchanged"`
	UpVolume          FlexInt              `json:"up_volume"`
	DownVolume        FlexInt              `json:"down_volume"`
	Components        map[string]FlexFloat `json:"components"`
}

type FearGreedHistoryPoint struct {
	TradeDate  string    `json:"trade_date"`
	IndexScore FlexFloat `json:"index_score"`
	Label      string    `json:"label"`
}

type Client struct {
	BaseURL    string
	SiteRoot   string
	Referer    string
	UserAgent  string
	HTTPClient *http.Client

	nonce      string
	nonceTime  time.Time
	nonceMutex sync.RWMutex

	compMu    sync.RWMutex
	companies *CompanyList
	compAt    time.Time

	posMu sync.RWMutex
	pos   map[string]cachedPositionSet

	// posGroup collapses concurrent GetPositionSet calls for the same window
	// into one upstream fetch, the way nonceGroup does for nonce refreshes.
	posGroup singleflight.Group
}

// cachedPositionSet is a per-symbol+range position set. Floorsheet data is
// deterministic for a fixed window (verified live), so caching turns repeated
// 2-7s calls into ~0ms.
type cachedPositionSet struct {
	ps *PositionSet
	at time.Time
}

func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       45 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	c := &Client{
		BaseURL:   getEnv("BROKER_API_URL", "https://laganilab.com/wp-json/laganilab/v1"),
		SiteRoot:  getEnv("BROKER_SITE_ROOT", "https://laganilab.com"),
		Referer:   getEnv("BROKER_API_REFERER", "https://laganilab.com/free-floorsheet-analysis-nepse/"),
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:147.0) Gecko/20100101 Firefox/147.0",
		HTTPClient: &http.Client{
			Jar:       jar,
			Transport: transport,
		},
		pos: make(map[string]cachedPositionSet),
	}

	// Warm the nonce in the BACKGROUND instead of blocking server startup for
	// up to 30s: doJSON self-heals an empty/stale nonce via its forced-refresh
	// path on 403, so an unwarmed client only costs one extra round trip on
	// the very first broker call. A slow or down site can no longer delay boot.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.FetchNonce(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "[Broker] nonce prefetch failed; will retry on demand via 403 path: %v\n", err)
			c.nonceMutex.Lock()
			if c.nonce == "" {
				c.nonce = getEnv("BROKER_API_TOKEN", "")
			}
			c.nonceMutex.Unlock()
		}
	}()

	return c
}

func (c *Client) getNonce() string {
	c.nonceMutex.RLock()
	defer c.nonceMutex.RUnlock()
	return c.nonce
}

// setNonce stores a nonce. Called on every response that echoes X-WP-Nonce,
// which gives us zero-cost self-healing refresh without dedicated fetches.
func (c *Client) setNonce(n string) {
	if n == "" {
		return
	}
	c.nonceMutex.Lock()
	c.nonce = n
	c.nonceTime = time.Now()
	c.nonceMutex.Unlock()
}

func (c *Client) EnsureNonce(ctx context.Context) error {
	c.nonceMutex.RLock()
	t := c.nonceTime
	c.nonceMutex.RUnlock()

	if c.getNonce() == "" || t.IsZero() || time.Since(t) > nonceLifespan {
		return c.FetchNonce(ctx)
	}
	return nil
}

// FetchNonce refreshes the nonce under singleflight. HTML scrape is tried
// first because admin-ajax currently returns 400.
func (c *Client) FetchNonce(ctx context.Context) error {
	_, err, _ := nonceGroup.Do("fetch-nonce", func() (interface{}, error) {
		if nonce, err := c.fetchNonceViaHTML(ctx); err == nil && nonce != "" {
			c.setNonce(nonce)
			return nonce, nil
		}
		if nonce, err := c.fetchNonceViaAjax(ctx); err == nil && nonce != "" && nonce != "0" {
			c.setNonce(nonce)
			return nonce, nil
		}
		if c.getNonce() != "" {
			return c.getNonce(), nil
		}
		return nil, fmt.Errorf("all nonce acquisition strategies failed")
	})
	return err
}

func (c *Client) fetchNonceViaAjax(ctx context.Context) (string, error) {
	ajaxURL := fmt.Sprintf("%s/wp-admin/admin-ajax.php?action=rest-nonce", c.SiteRoot)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ajaxURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Referer", c.Referer)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ajax nonce status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func (c *Client) fetchNonceViaHTML(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.Referer, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("html nonce status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", err
	}
	bodyStr := string(bodyBytes)

	patterns := []*regexp.Regexp{noncePattern4, noncePattern5, noncePattern1, noncePattern2, noncePattern3}
	for _, re := range patterns {
		if match := re.FindStringSubmatch(bodyStr); len(match) > 1 {
			return match[1], nil
		}
	}
	return "", fmt.Errorf("nonce not found in page (%d bytes)", len(bodyBytes))
}

// doJSON is the reliability core: bounded concurrency, per-attempt timeouts,
// exponential backoff on 5xx/429/network errors, one forced nonce refresh on
// 403, and nonce capture from every successful response header.
func (c *Client) doJSON(ctx context.Context, endpoint string, params url.Values, attemptTimeout time.Duration, out interface{}) error {
	if ctx == nil {
		ctx = context.Background()
	}

	forcedRefresh := false
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(400*(1<<(attempt-1))) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		body, status, err := c.doOnce(ctx, endpoint, params, attemptTimeout)
		if err != nil {
			lastErr = err
			if isRetryable(status) {
				continue
			}
			if status == http.StatusForbidden && !forcedRefresh {
				forcedRefresh = true
				fmt.Fprintf(os.Stderr, "[Broker] 403 on %s, refreshing nonce\n", endpoint)
				if ferr := c.FetchNonce(ctx); ferr == nil {
					attempt--
					continue
				}
			}
			return err
		}

		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("parsing %s response: %w", endpoint, err)
		}
		return nil
	}
	return fmt.Errorf("%s failed after %d attempts: %w", endpoint, maxRetries, lastErr)
}

func (c *Client) doOnce(ctx context.Context, endpoint string, params url.Values, attemptTimeout time.Duration) ([]byte, int, error) {
	select {
	case requestSem <- struct{}{}:
		defer func() { <-requestSem }()
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}

	attCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()

	reqURL := fmt.Sprintf("%s/%s", c.BaseURL, endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(attCtx, "GET", reqURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", c.Referer)
	req.Header.Set("X-WP-Nonce", c.getNonce())
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if nh := resp.Header.Get("X-WP-Nonce"); nh != "" {
		c.setNonce(nh)
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading body: %w", err)
	}
	return body, resp.StatusCode, nil
}

func isRetryable(status int) bool {
	return status == 0 || status == http.StatusTooManyRequests || status >= 500
}

// GetPositionSet fetches all five broker views in one type=all request. On
// repeated transport failure it fans out five concurrent single-view requests
// and merges whatever succeeds, so a single broken endpoint never kills analysis.
//
// Caching is tiered: windows ending before today are immutable and cached for
// 24h; a window that includes today expires hourly. Concurrent callers asking
// for the same window share one upstream replay via singleflight.
func (c *Client) GetPositionSet(ctx context.Context, symbol, fromDate, toDate string) (*PositionSet, error) {
	key := symbol + "|" + fromDate + "|" + toDate

	// YYYY-MM-DD strings compare correctly lexicographically — no TZ math needed.
	ttl := positionCacheTTL
	if toDate < time.Now().Format("2006-01-02") {
		ttl = positionClosedWindowTTL
	}

	c.posMu.RLock()
	if cp, ok := c.pos[key]; ok && time.Since(cp.at) < ttl {
		c.posMu.RUnlock()
		return cp.ps, nil
	}
	c.posMu.RUnlock()

	v, err, _ := c.posGroup.Do(key, func() (interface{}, error) {
		return c.fetchPositionSet(ctx, symbol, fromDate, toDate, key)
	})
	if err != nil {
		return nil, err
	}
	return v.(*PositionSet), nil
}

// fetchPositionSet performs the type=all call (with the single-view fallback),
// stores the result under key, and prunes expired entries to bound memory.
func (c *Client) fetchPositionSet(ctx context.Context, symbol, fromDate, toDate, key string) (*PositionSet, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("from_date", fromDate)
	params.Set("to_date", toDate)
	params.Set("type", "all")

	var resp floorsheetAllResponse
	err := c.doJSON(ctx, "floorsheet", params, floorsheetTimeout, &resp)
	if err == nil {
		ps := &PositionSet{Data: resp.Data, Meta: resp.Meta, Source: "all"}
		c.storePositionSet(key, ps)
		return ps, nil
	}

	fmt.Fprintf(os.Stderr, "[Broker] type=all failed for %s (%v), trying single-view fallback\n", symbol, err)
	ps, err := c.getPositionSetFallback(ctx, symbol, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	c.storePositionSet(key, ps)
	return ps, nil
}

// storePositionSet saves one entry and evicts anything older than the longest
// TTL ever served. Without this the pos map grew without bound: every distinct
// symbol|range stayed forever, and long screening sessions scanning many names
// and windows accumulated full PositionSets.
func (c *Client) storePositionSet(key string, ps *PositionSet) {
	now := time.Now()
	c.posMu.Lock()
	c.pos[key] = cachedPositionSet{ps: ps, at: now}
	for k, cp := range c.pos {
		if now.Sub(cp.at) > positionClosedWindowTTL {
			delete(c.pos, k)
		}
	}
	c.posMu.Unlock()
}

func (c *Client) getPositionSetFallback(ctx context.Context, symbol, fromDate, toDate string) (*PositionSet, error) {
	views := []string{"total_holding", "holding", "released", "buyer", "seller"}
	type result struct {
		view string
		rows []BrokerEntry
		meta floorsheetSingleResponse
	}
	results := make(chan result, len(views))
	var wg sync.WaitGroup

	for _, v := range views {
		wg.Add(1)
		go func(view string) {
			defer wg.Done()
			params := url.Values{}
			params.Set("symbol", symbol)
			params.Set("from_date", fromDate)
			params.Set("to_date", toDate)
			params.Set("type", view)
			var sr floorsheetSingleResponse
			if err := c.doJSON(ctx, "floorsheet", params, defaultAttemptTimeout, &sr); err == nil && sr.Success {
				results <- result{view: view, rows: sr.Data, meta: sr}
			}
		}(v)
	}
	wg.Wait()
	close(results)

	ps := &PositionSet{Source: "single:fallback"}
	got := 0
	for r := range results {
		got++
		switch r.view {
		case "total_holding":
			ps.Data.TotalHolding = r.rows
		case "holding":
			ps.Data.Holding = r.rows
		case "released":
			ps.Data.Released = r.rows
		case "buyer":
			ps.Data.Buyer = r.rows
		case "seller":
			ps.Data.Seller = r.rows
		}
		if ps.Meta.Symbol == "" {
			ps.Meta.Symbol = r.meta.Meta.Symbol
			ps.Meta.FromDate = r.meta.Meta.FromDate
			ps.Meta.ToDate = r.meta.Meta.ToDate
		}
	}
	if got == 0 {
		return nil, fmt.Errorf("all fallback views failed for %s", symbol)
	}
	return ps, nil
}

// GetView returns one view's rows plus meta, still via the single type=all call.
func (c *Client) GetView(ctx context.Context, symbol, fromDate, toDate, view string) ([]BrokerEntry, *FloorsheetMeta, error) {
	ps, err := c.GetPositionSet(ctx, symbol, fromDate, toDate)
	if err != nil {
		return nil, nil, err
	}
	return RowsForView(ps, view), &ps.Meta, nil
}

// RowsForView extracts one view. Legacy "holding" maps to total_holding
// (net accumulators), matching the old API's semantics; "inferred" selects
// the v2 inferred-positions view.
func RowsForView(ps *PositionSet, view string) []BrokerEntry {
	switch view {
	case "total_holding":
		return ps.Data.TotalHolding
	case "holding":
		if len(ps.Data.TotalHolding) > 0 {
			return ps.Data.TotalHolding
		}
		return ps.Data.Holding
	case "inferred":
		return ps.Data.Holding
	case "released":
		return ps.Data.Released
	case "buyer":
		return ps.Data.Buyer
	case "seller":
		return ps.Data.Seller
	default:
		return nil
	}
}

// GetFloorsheet preserves the old call signature; internally it now makes one
// type=all request, so callers get every view for the price of one round trip.
func (c *Client) GetFloorsheet(symbol, fromDate, toDate, dataType string) (*PositionSet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), floorsheetTimeout+30*time.Second)
	defer cancel()
	return c.GetPositionSet(ctx, symbol, fromDate, toDate)
}

func (c *Client) GetCompanies() (*CompanyList, error) {
	c.compMu.RLock()
	if c.companies != nil && time.Since(c.compAt) < companiesCacheTTL {
		defer c.compMu.RUnlock()
		return c.companies, nil
	}
	c.compMu.RUnlock()

	var data CompanyList
	ctx, cancel := context.WithTimeout(context.Background(), defaultAttemptTimeout)
	defer cancel()
	if err := c.doJSON(ctx, "companies", url.Values{}, defaultAttemptTimeout, &data); err != nil {
		c.compMu.RLock()
		defer c.compMu.RUnlock()
		if c.companies != nil {
			return c.companies, nil
		}
		return nil, err
	}

	c.compMu.Lock()
	c.companies = &data
	c.compAt = time.Now()
	c.compMu.Unlock()
	return &data, nil
}

func (c *Client) GetDateRange(symbol string) (*DateRange, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	var data DateRange
	ctx, cancel := context.WithTimeout(context.Background(), defaultAttemptTimeout)
	defer cancel()
	if err := c.doJSON(ctx, "date-range", params, defaultAttemptTimeout, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetFearGreed fetches the fear/greed index. indexKey is optional
// (e.g. "banking_subindex"); empty returns the NEPSE composite reading.
func (c *Client) GetFearGreed(ctx context.Context, indexKey string) (*FearGreedResponse, error) {
	params := url.Values{}
	if indexKey != "" {
		params.Set("index", indexKey)
	}
	var data FearGreedResponse
	if err := c.doJSON(ctx, "nepse-fear-greed", params, defaultAttemptTimeout, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
