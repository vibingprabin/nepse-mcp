package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/patrickmn/go-cache"
	api "github.com/voidarchive/go-nepse"
	"vibinprabin/nepse-mcp/config"
)

// NepseClient wraps the go-nepse client with caching support
type NepseClient struct {
	client *api.Client
	cache  *cache.Cache
	config *config.Config
}

// NewNepseClient creates a new NEPSE client with caching
func NewNepseClient(cfg *config.Config) *NepseClient {
	c := cache.New(cfg.NepseCacheTTL, 10*time.Minute)
	
    // Initialize with default options
    opts := api.DefaultOptions()
    opts.HTTPTimeout = cfg.NepseTimeout
    
	client, err := api.NewClient(opts)
	if err != nil {
		log.Printf("Warning: Failed to initialize NEPSE client: %v", err)
		// Return with nil client - methods will need to handle this
	}

	return &NepseClient{
		client: client,
		cache:  c,
		config: cfg,
	}
}

// fetchWithRetry retries idempotent read calls against nepalstock.com.np.
// The API intermittently returns truncated bodies ("http: ContentLength=10
// with Body length 0") and connection resets; a short backoff usually gets
// the data on the retry.
func fetchWithRetry(fetchFunc func() (interface{}, error)) (interface{}, error) {
	var lastErr error
	backoff := []time.Duration{300 * time.Millisecond, 800 * time.Millisecond}
	for attempt := 0; attempt <= len(backoff); attempt++ {
		val, err := fetchFunc()
		if err == nil {
			return val, nil
		}
		lastErr = err
		if attempt < len(backoff) {
			time.Sleep(backoff[attempt])
		}
	}
	return nil, lastErr
}

// getCached retrieves from cache or fetches using the provided function
func (c *NepseClient) getCached(key string, fetchFunc func() (interface{}, error)) (interface{}, error) {
	return c.getCachedWithTTL(key, cache.DefaultExpiration, fetchFunc)
}

// getCachedWithTTL is getCached with a per-key TTL. It exists because some
// payloads are immutable while others track a live tape — one global TTL either
// wastes round trips on history or serves stale quotes.
func (c *NepseClient) getCachedWithTTL(key string, ttl time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error) {
	if val, found := c.cache.Get(key); found {
		return val, nil
	}

	val, err := fetchWithRetry(fetchFunc)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, val, ttl)
	return val, nil
}

// checkClient ensures the underlying client is initialized
func (c *NepseClient) checkClient() error {
	if c.client == nil {
		return fmt.Errorf("NEPSE client not initialized")
	}
	return nil
}

// --- Market Data ---

// GetMarketSummary returns overall market statistics
func (c *NepseClient) GetMarketSummary() (*api.MarketSummary, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
	cacheKey := "market_summary"
	result, err := c.getCached(cacheKey, func() (interface{}, error) {
		return c.client.MarketSummary(context.Background())
	})
	if err != nil {
		return nil, err
	}
	return result.(*api.MarketSummary), nil
}

// GetMarketStatus returns current market open/close status. Short TTL: the
// flag flips at open/close, but get_market_summary calls this on every run and
// an unconditional round trip per summary is pure latency.
func (c *NepseClient) GetMarketStatus() (*api.MarketStatus, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}

	result, err := c.getCachedWithTTL("market_status", 15*time.Second, func() (interface{}, error) {
		return c.client.MarketStatus(context.Background())
	})
	if err != nil {
		return nil, err
	}
	return result.(*api.MarketStatus), nil
}

// GetNepseIndex returns main NEPSE index data
func (c *NepseClient) GetNepseIndex() (*api.NepseIndex, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
	cacheKey := "nepse_index"
	result, err := c.getCached(cacheKey, func() (interface{}, error) {
		return c.client.NepseIndex(context.Background())
	})
	if err != nil {
		return nil, err
	}
	return result.(*api.NepseIndex), nil
}

// GetSubIndices returns all sector sub-indices
func (c *NepseClient) GetSubIndices() ([]api.SubIndex, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
    cacheKey := "sub_indices"
    result, err := c.getCached(cacheKey, func() (interface{}, error) {
        return c.client.SubIndices(context.Background())
    })
    if err != nil {
        return nil, err
    }
    return result.([]api.SubIndex), nil
}

// GetLiveMarket returns real-time trading data for all securities
func (c *NepseClient) GetLiveMarket() ([]api.LiveMarketEntry, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
	cacheKey := "live_market"
	result, err := c.getCached(cacheKey, func() (interface{}, error) {
		return c.client.LiveMarket(context.Background())
	})
	if err != nil {
		return nil, err
	}
	return result.([]api.LiveMarketEntry), nil
}

// GetSupplyDemand returns aggregate supply and demand data
func (c *NepseClient) GetSupplyDemand() (*api.SupplyDemandData, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
    result, err := c.getCached("supply_demand", func() (interface{}, error) {
        return c.client.SupplyDemand(context.Background())
    })
    if err != nil {
        return nil, err
    }
    return result.(*api.SupplyDemandData), nil
}

// --- Company ---

// GetCompanyList returns all listed companies (cached for 1 hour)
func (c *NepseClient) GetCompanyList() ([]api.Company, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}

	result, err := c.getCachedWithTTL("company_list", 1*time.Hour, func() (interface{}, error) {
		return c.client.Companies(context.Background())
	})
	if err != nil {
		return nil, err
	}
	return result.([]api.Company), nil
}

// GetSecurityDetail returns detailed information for a specific symbol
func (c *NepseClient) GetSecurityDetail(symbol string) (*api.SecurityDetail, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
	cacheKey := fmt.Sprintf("security_detail:%s", symbol)
	result, err := c.getCached(cacheKey, func() (interface{}, error) {
		return c.client.SecurityDetailBySymbol(context.Background(), symbol)
	})
	if err != nil {
		return nil, err
	}
	return result.(*api.SecurityDetail), nil
}

// --- Price & Trading ---

// GetPriceHistory returns historical OHLCV data for a symbol. A range that
// ended before today is immutable — it gets a day-long TTL instead of the
// default 60s, so re-running a pipeline against the same history costs nothing.
func (c *NepseClient) GetPriceHistory(symbol string, start, end string) ([]api.PriceHistory, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("history:%s:%s:%s", symbol, start, end)
	ttl := cache.DefaultExpiration
	if end < time.Now().Format("2006-01-02") {
		ttl = 24 * time.Hour
	}
	result, err := c.getCachedWithTTL(cacheKey, ttl, func() (interface{}, error) {
		return c.client.PriceHistoryBySymbol(context.Background(), symbol, start, end)
	})
	if err != nil {
		return nil, err
	}
	return result.([]api.PriceHistory), nil
}

// GetMarketDepth returns order book data for a symbol
func (c *NepseClient) GetMarketDepth(symbol string) (*api.MarketDepth, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
	cacheKey := fmt.Sprintf("depth:%s", symbol)
	result, err := c.getCached(cacheKey, func() (interface{}, error) {
		return c.client.MarketDepthBySymbol(context.Background(), symbol)
	})
	if err != nil {
		return nil, err
	}
	return result.(*api.MarketDepth), nil
}

// GetFloorSheet returns today's floor sheet trades
func (c *NepseClient) GetFloorSheet(limit int) ([]api.FloorSheetEntry, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
    cacheKey := "floorsheet"
    result, err := c.getCached(cacheKey, func() (interface{}, error) {
        return c.client.FloorSheet(context.Background())
    })
    if err != nil {
        return nil, err
    }
    return result.([]api.FloorSheetEntry), nil
}

// GetFloorSheetBySymbol returns floor sheet trades filtered by symbol.
// The date is part of the cache key (computed outside the fetch func): a
// date-less key would serve yesterday's sheet after midnight until the TTL ran out.
func (c *NepseClient) GetFloorSheetBySymbol(symbol string) ([]api.FloorSheetEntry, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}

	loc, _ := time.LoadLocation("Asia/Kathmandu")
	today := time.Now().In(loc).Format("2006-01-02")
	cacheKey := fmt.Sprintf("floorsheet:%s:%s", symbol, today)
	result, err := c.getCached(cacheKey, func() (interface{}, error) {
		return c.client.FloorSheetBySymbol(context.Background(), symbol, today)
	})
	if err != nil {
		return nil, err
	}
	return result.([]api.FloorSheetEntry), nil
}

// --- Top Lists ---

// GetTopTen returns top 10 securities by specified metric
func (c *NepseClient) GetTopTen(listType string) (interface{}, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
    cacheKey := fmt.Sprintf("top:%s", listType)
    result, err := c.getCached(cacheKey, func() (interface{}, error) {
        switch listType {
        case "gainers":
            return c.client.TopGainers(context.Background())
        case "losers":
            return c.client.TopLosers(context.Background())
        case "turnover":
            return c.client.TopTenTurnover(context.Background())
        case "volume":
            return c.client.TopTenTrade(context.Background())
        case "transactions":
            return c.client.TopTenTransaction(context.Background())
        default:
            return nil, fmt.Errorf("unknown list type: %s", listType)
        }
    })
    return result, err
}

// --- Graphs ---

// GetNepseIndexGraph returns intraday index chart data
func (c *NepseClient) GetNepseIndexGraph() ([]api.GraphDataPoint, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
    cacheKey := "graph:nepse"
    result, err := c.getCached(cacheKey, func() (interface{}, error) {
        resp, err := c.client.DailyNepseIndexGraph(context.Background())
        if err != nil { return nil, err }
        return resp.Data, nil
    })
    if err != nil { return nil, err }
    return result.([]api.GraphDataPoint), nil
}

// GetCompanyGraph returns intraday chart data for a symbol
func (c *NepseClient) GetCompanyGraph(symbol string) ([]api.GraphDataPoint, error) {
	if err := c.checkClient(); err != nil {
		return nil, err
	}
	
    cacheKey := fmt.Sprintf("graph:company:%s", symbol)
    result, err := c.getCached(cacheKey, func() (interface{}, error) {
        resp, err := c.client.DailyScripGraphBySymbol(context.Background(), symbol)
        if err != nil { return nil, err }
        return resp.Data, nil
    })
    if err != nil { return nil, err }
    return result.([]api.GraphDataPoint), nil
}

// Raw returns the underlying go-nepse client for advanced usage
func (c *NepseClient) Raw() *api.Client {
	return c.client
}
