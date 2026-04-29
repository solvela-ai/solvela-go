package solvela

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
)

// validateGatewayURL rejects plaintext http:// gateway URLs unless the host is
// a recognized loopback address. Sending payments over plaintext to a remote
// host would leak the Payment-Signature header and expose the request body to
// any on-path observer.
func validateGatewayURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid gateway URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("gateway URL must use http or https, got %q", u.Scheme)
	}
	if u.Scheme == "http" {
		host := u.Hostname()
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return fmt.Errorf("gateway URL must use https:// for non-local endpoints, got %s://%s", u.Scheme, host)
		}
	}
	return nil
}

// SolvelaClient is the main client for interacting with the Solvela gateway.
type SolvelaClient struct {
	config       ClientConfig
	wallet       *Wallet
	signer       Signer
	transport    *Transport
	cache        *ResponseCache
	sessionStore *SessionStore
	sessionSalt  []byte
	lastBalance  *float64
}

// NewClient creates a new SolvelaClient with functional options.
//
// The gateway URL is validated: plaintext http:// is only allowed for the
// loopback hosts localhost, 127.0.0.1, and ::1. All other endpoints must use
// https://. Misconfigured URLs return a [ClientError]; the client itself is
// returned only on success.
func NewClient(wallet *Wallet, signer Signer, opts ...Option) (*SolvelaClient, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if err := validateGatewayURL(cfg.GatewayURL); err != nil {
		return nil, &ClientError{Message: err.Error()}
	}

	salt, err := newSessionSalt()
	if err != nil {
		return nil, &ClientError{Message: fmt.Sprintf("failed to seed session salt: %v", err)}
	}

	c := &SolvelaClient{
		config:      cfg,
		wallet:      wallet,
		signer:      signer,
		transport:   NewTransport(cfg.GatewayURL, cfg.Timeout),
		sessionSalt: salt,
	}
	if cfg.EnableCache {
		c.cache = NewResponseCache()
	}
	if cfg.EnableSessions {
		c.sessionStore = NewSessionStore(cfg.SessionTTL)
	}
	return c, nil
}

// Chat sends a non-streaming chat request with automatic payment, caching,
// session tracking, and quality checking.
func (c *SolvelaClient) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	model := request.Model

	// Step 1: Balance guard
	if c.lastBalance != nil && *c.lastBalance == 0 && c.config.FreeFallbackModel != "" {
		model = c.config.FreeFallbackModel
	}

	// Step 2: Session lookup
	var sessionID string
	if c.sessionStore != nil {
		sessionID = DeriveSessionIDWithSalt(c.sessionSalt, request.Messages)
		info := c.sessionStore.GetOrCreate(sessionID, model)
		if model == request.Model { // not overridden by balance guard
			model = info.Model
		}
	}

	// Step 3: Cache check (after model finalization)
	var cacheKey uint64
	if c.cache != nil {
		cacheKey = CacheKey(model, request.Messages)
		if cached, ok := c.cache.Get(cacheKey); ok {
			return &cached, nil
		}
	}

	// Build effective request
	effectiveReq := &ChatRequest{
		Model:       model,
		Messages:    request.Messages,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stream:      false,
		Tools:       request.Tools,
		ToolChoice:  request.ToolChoice,
	}

	// Step 4: Send request
	response, err := c.sendWithPayment(ctx, effectiveReq, nil)
	if err != nil {
		return nil, err
	}

	// Step 5: Quality check + retry
	if c.config.EnableQualityCheck {
		for i := 0; i < c.config.MaxQualityRetries; i++ {
			if len(response.Choices) == 0 {
				break
			}
			reason := CheckDegraded(response.Choices[0].Message.Content)
			if reason == "" {
				break
			}
			response, err = c.sendWithPayment(ctx, effectiveReq, map[string]string{
				"X-Solvela-Retry-Reason": "degraded",
			})
			if err != nil {
				return nil, err
			}
		}
	}

	// Step 6: Cache store
	if c.cache != nil {
		c.cache.Put(cacheKey, *response)
	}

	// Step 7: Session update
	if c.sessionStore != nil && sessionID != "" {
		hash := CacheKey(model, request.Messages)
		c.sessionStore.RecordRequest(sessionID, hash)
	}

	return response, nil
}

// ChatStream sends a streaming chat request.
//
// Payment handshake: the streaming endpoint still returns a 402 on the initial
// POST when payment is required, so this method runs the same handshake as
// [SolvelaClient.Chat] before opening the stream — a non-streaming request is
// issued first to discover the price, the configured [Signer] (if any) signs
// the payment, and the streaming request is then re-sent with the
// Payment-Signature header attached. If no [Signer] is configured and a 402 is
// returned, this method surfaces a [PaymentRequiredError] rather than silently
// streaming an unauthenticated request.
func (c *SolvelaClient) ChatStream(ctx context.Context, request *ChatRequest) (<-chan ChatChunkOrError, error) {
	model := request.Model

	// Step 1: Balance guard
	if c.lastBalance != nil && *c.lastBalance == 0 && c.config.FreeFallbackModel != "" {
		model = c.config.FreeFallbackModel
	}

	// Step 2: Session lookup
	var sessionID string
	if c.sessionStore != nil {
		sessionID = DeriveSessionIDWithSalt(c.sessionSalt, request.Messages)
		info := c.sessionStore.GetOrCreate(sessionID, model)
		if model == request.Model {
			model = info.Model
		}
	}

	effectiveReq := &ChatRequest{
		Model:    model,
		Messages: request.Messages,
		Stream:   true,
	}

	// Step 3: Resolve payment (if any) before opening the stream.
	sig, err := c.resolvePaymentSignature(ctx, effectiveReq)
	if err != nil {
		return nil, err
	}

	ch, err := c.transport.SendChatStream(ctx, effectiveReq, sig, nil)
	if err != nil {
		return nil, err
	}

	// Wrap channel to do session update after stream completes
	if c.sessionStore != nil && sessionID != "" {
		wrappedCh := make(chan ChatChunkOrError)
		go func() {
			defer close(wrappedCh)
			for item := range ch {
				wrappedCh <- item
			}
			hash := CacheKey(model, request.Messages)
			c.sessionStore.RecordRequest(sessionID, hash)
		}()
		return wrappedCh, nil
	}

	return ch, nil
}

// Models retrieves available models from the gateway.
func (c *SolvelaClient) Models(ctx context.Context) ([]ModelInfo, error) {
	return c.transport.FetchModels(ctx)
}

// LastKnownBalance returns the most recently known wallet balance, or nil.
func (c *SolvelaClient) LastKnownBalance() *float64 {
	return c.lastBalance
}

// String returns a debug-safe representation with redacted secrets.
func (c *SolvelaClient) String() string {
	return fmt.Sprintf("SolvelaClient(gateway=%s, wallet=REDACTED)", c.config.GatewayURL)
}

func (c *SolvelaClient) sendWithPayment(ctx context.Context, request *ChatRequest, extraHeaders map[string]string) (*ChatResponse, error) {
	// Force non-streaming for the price-discovery probe; we will stream at the
	// caller layer if needed. This is also the live request when the caller
	// invokes Chat (non-streaming).
	probeReq := *request
	probeReq.Stream = false

	result, err := c.transport.SendChat(ctx, &probeReq, "", extraHeaders)
	if err != nil {
		return nil, err
	}

	if result.PaymentRequired == nil {
		return result.Response, nil
	}

	sig, err := c.signPaymentRequired(ctx, result.PaymentRequired)
	if err != nil {
		return nil, err
	}

	result, err = c.transport.SendChat(ctx, &probeReq, sig, extraHeaders)
	if err != nil {
		return nil, err
	}
	if result.PaymentRequired != nil {
		return nil, &PaymentRejectedError{Reason: "payment rejected after signing"}
	}
	return result.Response, nil
}

// resolvePaymentSignature issues a non-streaming probe to discover whether the
// gateway requires payment for the given request, and signs the payment if so.
// Returns the encoded Payment-Signature header value, or "" if no payment is
// needed. The probe shares the same handshake logic as non-streaming Chat —
// the gateway returns 402 on the initial POST whether or not the response is
// streamed, so the same flow is safe to reuse.
func (c *SolvelaClient) resolvePaymentSignature(ctx context.Context, request *ChatRequest) (string, error) {
	probeReq := *request
	probeReq.Stream = false

	result, err := c.transport.SendChat(ctx, &probeReq, "", nil)
	if err != nil {
		return "", err
	}
	if result.PaymentRequired == nil {
		// No payment required; the streaming request can proceed unauthenticated.
		return "", nil
	}
	return c.signPaymentRequired(ctx, result.PaymentRequired)
}

// signPaymentRequired runs the validation + signer pipeline for a 402 response
// and returns the base64-encoded Payment-Signature header value.
func (c *SolvelaClient) signPaymentRequired(ctx context.Context, pr *PaymentRequired) (string, error) {
	if c.signer == nil {
		return "", &PaymentRequiredError{PaymentRequired: *pr}
	}
	accepted := c.findCompatibleScheme(pr)
	if accepted == nil {
		return "", &PaymentRequiredError{PaymentRequired: *pr}
	}
	if err := c.validatePayment(accepted); err != nil {
		return "", err
	}

	amountAtomic := parseAmount(accepted.Amount)
	payload, err := c.signer.SignPayment(ctx, amountAtomic, accepted.PayTo, pr.Resource, *accepted)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode payment payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(payloadJSON), nil
}

// isCompatibleAccept reports whether an accepted scheme entry is one we can
// safely sign and pay. We require:
//   - scheme is "exact" or "escrow"
//   - network is the Solana mainnet identifier we support
//   - asset is the canonical USDC mint (no surprise SPL tokens)
//
// Asset and network must be validated here to prevent a malicious gateway from
// tricking the SDK into signing a transfer of a different token, or paying on
// a different chain.
func isCompatibleAccept(a PaymentAccept, scheme string) bool {
	return a.Scheme == scheme && a.Network == SolanaNetwork && a.Asset == USDCMint
}

func (c *SolvelaClient) findCompatibleScheme(pr *PaymentRequired) *PaymentAccept {
	for i := range pr.Accepts {
		if isCompatibleAccept(pr.Accepts[i], "exact") {
			return &pr.Accepts[i]
		}
	}
	for i := range pr.Accepts {
		if isCompatibleAccept(pr.Accepts[i], "escrow") {
			return &pr.Accepts[i]
		}
	}
	return nil
}

func (c *SolvelaClient) validatePayment(accepted *PaymentAccept) error {
	if c.config.ExpectedRecipient != "" && accepted.PayTo != c.config.ExpectedRecipient {
		return &RecipientMismatchError{Expected: c.config.ExpectedRecipient, Actual: accepted.PayTo}
	}
	// Defense-in-depth: re-verify network and asset here in case a caller bypassed
	// findCompatibleScheme.
	if accepted.Network != SolanaNetwork {
		return &ClientError{Message: fmt.Sprintf("unsupported network: %q (expected %q)", accepted.Network, SolanaNetwork)}
	}
	if accepted.Asset != USDCMint {
		return &ClientError{Message: fmt.Sprintf("unsupported asset: %q (expected USDC mint %q)", accepted.Asset, USDCMint)}
	}
	// Fail closed when no max is configured. A nil cap means "unbounded", which
	// turns the SDK into a footgun if a misconfigured gateway returns an
	// inflated amount; better to refuse than to drain the wallet.
	if c.config.MaxPaymentAmount == nil {
		return &ClientError{Message: "client misconfigured: MaxPaymentAmount must be set (use WithMaxPaymentAmount or DefaultConfig)"}
	}
	amount := parseAmount(accepted.Amount)
	if amount > *c.config.MaxPaymentAmount {
		return &AmountExceedsMaxError{Amount: amount, MaxAmount: *c.config.MaxPaymentAmount}
	}
	return nil
}

func parseAmount(s string) uint64 {
	var n uint64
	fmt.Sscanf(s, "%d", &n)
	return n
}
