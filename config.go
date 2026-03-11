package rustyclaw

import "time"

// ClientConfig holds all configuration for a RustyClaw client.
type ClientConfig struct {
	GatewayURL         string
	RPCURL             string
	PreferEscrow       bool
	Timeout            time.Duration
	ExpectedRecipient  string
	MaxPaymentAmount   *uint64
	EnableCache        bool
	EnableSessions     bool
	SessionTTL         time.Duration
	EnableQualityCheck bool
	MaxQualityRetries  int
	FreeFallbackModel  string
}

// DefaultConfig returns a ClientConfig with sensible defaults.
func DefaultConfig() ClientConfig {
	return ClientConfig{
		GatewayURL:        "http://localhost:8402",
		RPCURL:            "https://api.mainnet-beta.solana.com",
		Timeout:           180 * time.Second,
		SessionTTL:        1800 * time.Second,
		MaxQualityRetries: 1,
	}
}

// Option is a functional option for configuring ClientConfig.
type Option func(*ClientConfig)

// WithGatewayURL sets the gateway URL.
func WithGatewayURL(url string) Option { return func(c *ClientConfig) { c.GatewayURL = url } }

// WithRPCURL sets the Solana RPC URL.
func WithRPCURL(url string) Option { return func(c *ClientConfig) { c.RPCURL = url } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(c *ClientConfig) { c.Timeout = d } }

// WithExpectedRecipient sets the expected payment recipient for verification.
func WithExpectedRecipient(r string) Option {
	return func(c *ClientConfig) { c.ExpectedRecipient = r }
}

// WithMaxPaymentAmount sets the maximum payment amount in atomic units.
func WithMaxPaymentAmount(max uint64) Option {
	return func(c *ClientConfig) { m := max; c.MaxPaymentAmount = &m }
}

// WithCache enables or disables response caching.
func WithCache(enable bool) Option { return func(c *ClientConfig) { c.EnableCache = enable } }

// WithSessions enables or disables session tracking.
func WithSessions(enable bool) Option { return func(c *ClientConfig) { c.EnableSessions = enable } }

// WithSessionTTL sets the session time-to-live.
func WithSessionTTL(d time.Duration) Option { return func(c *ClientConfig) { c.SessionTTL = d } }

// WithQualityCheck enables or disables quality checking.
func WithQualityCheck(enable bool) Option {
	return func(c *ClientConfig) { c.EnableQualityCheck = enable }
}

// WithMaxQualityRetries sets the maximum number of quality retries.
func WithMaxQualityRetries(n int) Option {
	return func(c *ClientConfig) { c.MaxQualityRetries = n }
}

// WithFreeFallbackModel sets the free model to fall back to.
func WithFreeFallbackModel(model string) Option {
	return func(c *ClientConfig) { c.FreeFallbackModel = model }
}
