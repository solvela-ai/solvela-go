package rustyclaw

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// RustyClawClient is the main client for interacting with the RustyClaw gateway.
type RustyClawClient struct {
	config       ClientConfig
	wallet       *Wallet
	signer       Signer
	transport    *Transport
	cache        *ResponseCache
	sessionStore *SessionStore
	lastBalance  *float64
}

// NewClient creates a new RustyClawClient with functional options.
func NewClient(wallet *Wallet, signer Signer, opts ...Option) *RustyClawClient {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	c := &RustyClawClient{
		config:    cfg,
		wallet:    wallet,
		signer:    signer,
		transport: NewTransport(cfg.GatewayURL, cfg.Timeout),
	}
	if cfg.EnableCache {
		c.cache = NewResponseCache()
	}
	if cfg.EnableSessions {
		c.sessionStore = NewSessionStore(cfg.SessionTTL)
	}
	return c
}

// Chat sends a non-streaming chat request with automatic payment, caching,
// session tracking, and quality checking.
func (c *RustyClawClient) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	model := request.Model

	// Step 1: Balance guard
	if c.lastBalance != nil && *c.lastBalance == 0 && c.config.FreeFallbackModel != "" {
		model = c.config.FreeFallbackModel
	}

	// Step 2: Session lookup
	var sessionID string
	if c.sessionStore != nil {
		sessionID = DeriveSessionID(request.Messages)
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
				"X-RCR-Retry-Reason": "degraded",
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
func (c *RustyClawClient) ChatStream(ctx context.Context, request *ChatRequest) (<-chan ChatChunkOrError, error) {
	model := request.Model

	// Step 1: Balance guard
	if c.lastBalance != nil && *c.lastBalance == 0 && c.config.FreeFallbackModel != "" {
		model = c.config.FreeFallbackModel
	}

	// Step 2: Session lookup
	var sessionID string
	if c.sessionStore != nil {
		sessionID = DeriveSessionID(request.Messages)
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

	ch, err := c.transport.SendChatStream(ctx, effectiveReq, "", nil)
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
func (c *RustyClawClient) Models(ctx context.Context) ([]ModelInfo, error) {
	return c.transport.FetchModels(ctx)
}

// LastKnownBalance returns the most recently known wallet balance, or nil.
func (c *RustyClawClient) LastKnownBalance() *float64 {
	return c.lastBalance
}

// String returns a debug-safe representation with redacted secrets.
func (c *RustyClawClient) String() string {
	return fmt.Sprintf("RustyClawClient(gateway=%s, wallet=REDACTED)", c.config.GatewayURL)
}

func (c *RustyClawClient) sendWithPayment(ctx context.Context, request *ChatRequest, extraHeaders map[string]string) (*ChatResponse, error) {
	result, err := c.transport.SendChat(ctx, request, "", extraHeaders)
	if err != nil {
		return nil, err
	}

	if result.PaymentRequired != nil {
		if c.signer == nil {
			return nil, &PaymentRequiredError{PaymentRequired: *result.PaymentRequired}
		}

		accepted := c.findCompatibleScheme(result.PaymentRequired)
		if accepted == nil {
			return nil, &PaymentRequiredError{PaymentRequired: *result.PaymentRequired}
		}

		if err := c.validatePayment(accepted); err != nil {
			return nil, err
		}

		// Sign payment
		amountAtomic := parseAmount(accepted.Amount)
		payload, err := c.signer.SignPayment(ctx, amountAtomic, accepted.PayTo, result.PaymentRequired.Resource, *accepted)
		if err != nil {
			return nil, err
		}

		payloadJSON, _ := json.Marshal(payload)
		sig := base64.StdEncoding.EncodeToString(payloadJSON)

		result, err = c.transport.SendChat(ctx, request, sig, extraHeaders)
		if err != nil {
			return nil, err
		}
		if result.PaymentRequired != nil {
			return nil, &PaymentRejectedError{Reason: "payment rejected after signing"}
		}
	}

	return result.Response, nil
}

func (c *RustyClawClient) findCompatibleScheme(pr *PaymentRequired) *PaymentAccept {
	for i := range pr.Accepts {
		if pr.Accepts[i].Scheme == "exact" && pr.Accepts[i].Network == SolanaNetwork {
			return &pr.Accepts[i]
		}
	}
	for i := range pr.Accepts {
		if pr.Accepts[i].Scheme == "escrow" && pr.Accepts[i].Network == SolanaNetwork {
			return &pr.Accepts[i]
		}
	}
	return nil
}

func (c *RustyClawClient) validatePayment(accepted *PaymentAccept) error {
	if c.config.ExpectedRecipient != "" && accepted.PayTo != c.config.ExpectedRecipient {
		return &RecipientMismatchError{Expected: c.config.ExpectedRecipient, Actual: accepted.PayTo}
	}
	if c.config.MaxPaymentAmount != nil {
		amount := parseAmount(accepted.Amount)
		if amount > *c.config.MaxPaymentAmount {
			return &AmountExceedsMaxError{Amount: amount, MaxAmount: *c.config.MaxPaymentAmount}
		}
	}
	return nil
}

func parseAmount(s string) uint64 {
	var n uint64
	fmt.Sscanf(s, "%d", &n)
	return n
}
