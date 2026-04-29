package solvela

import "time"

// DefaultMaxPaymentAmount is the default cap on a single payment, in atomic
// USDC units (10 USDC = 10_000_000). This is high enough to cover any
// reasonable single LLM call, but low enough to block obvious wallet drains
// when a misbehaving or malicious gateway returns a wildly inflated amount.
//
// Callers that need a higher per-request cap must override this explicitly via
// [WithMaxPaymentAmount]. Callers that want no cap at all should pass the
// largest uint64 value.
const DefaultMaxPaymentAmount uint64 = 10_000_000

// ClientConfig holds all configuration for a Solvela client.
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
//
// Security defaults:
//   - GatewayURL points to the production HTTPS endpoint.
//   - MaxPaymentAmount is set to [DefaultMaxPaymentAmount] (10 USDC atomic) to
//     prevent wallet drains from a misconfigured or malicious gateway.
func DefaultConfig() ClientConfig {
	defaultMax := DefaultMaxPaymentAmount
	return ClientConfig{
		GatewayURL:        "https://api.solvela.ai",
		RPCURL:            "https://api.mainnet-beta.solana.com",
		Timeout:           180 * time.Second,
		SessionTTL:        1800 * time.Second,
		MaxQualityRetries: 1,
		MaxPaymentAmount:  &defaultMax,
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
