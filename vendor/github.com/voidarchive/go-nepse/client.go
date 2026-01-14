package nepse

import (
	"net/http"
	"time"

	"github.com/voidarchive/go-nepse/internal/auth"
)

// Client is the NEPSE API client. Use [NewClient] to create one.
type Client struct {
	httpClient  *http.Client
	config      *Config
	authManager *auth.Manager
	options     *Options
}

// Options configures the NEPSE client.
type Options struct {
	BaseURL         string        // Override default API URL (useful for testing/proxying)
	TLSVerification bool          // Set false only for development; NEPSE uses self-signed certs
	HTTPTimeout     time.Duration // Per-request timeout
	MaxRetries      int           // Retry count for transient failures (5xx, rate limits)
	RetryDelay      time.Duration // Base delay; actual delay uses exponential backoff
	Config          *Config       // API endpoint paths and headers
	HTTPClient      *http.Client  // Bring your own client; nil uses sensible defaults
}

// DefaultOptions returns sensible defaults for the NEPSE client.
func DefaultOptions() *Options {
	return &Options{
		BaseURL:         DefaultBaseURL,
		TLSVerification: true,
		HTTPTimeout:     30 * time.Second,
		MaxRetries:      3,
		RetryDelay:      time.Second,
		Config:          DefaultConfig(),
	}
}

// NewClient creates a NEPSE API client.
// If options is nil, DefaultOptions() is used.
func NewClient(options *Options) (*Client, error) {
	if options == nil {
		options = DefaultOptions()
	}
	if options.Config == nil {
		options.Config = DefaultConfig()
	}
	return initClient(options)
}

// Config returns the client's configuration.
func (c *Client) Config() *Config {
	return c.config
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	if c.authManager != nil {
		return c.authManager.Close()
	}
	return nil
}
