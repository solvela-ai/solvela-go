package solvela

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewClientCreation(t *testing.T) {
	wallet, _, err := CreateWallet()
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	client, err := NewClient(wallet, nil,
		WithGatewayURL("https://example.com"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if client.config.GatewayURL != "https://example.com" {
		t.Errorf("gateway: got %q, want %q", client.config.GatewayURL, "https://example.com")
	}
	if client.config.Timeout != 30*time.Second {
		t.Errorf("timeout: got %v, want 30s", client.config.Timeout)
	}
	if client.cache != nil {
		t.Error("cache should be nil by default")
	}
	if client.sessionStore != nil {
		t.Error("session store should be nil by default")
	}
}

func TestNewClientWithCache(t *testing.T) {
	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil, WithCache(true))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if client.cache == nil {
		t.Error("cache should be enabled")
	}
}

func TestNewClientWithSessions(t *testing.T) {
	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil, WithSessions(true))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if client.sessionStore == nil {
		t.Error("session store should be enabled")
	}
}

func TestNewClientRejectsPlaintextRemote(t *testing.T) {
	wallet, _, _ := CreateWallet()
	if _, err := NewClient(wallet, nil, WithGatewayURL("http://api.solvela.ai")); err == nil {
		t.Fatal("expected error rejecting plaintext remote URL")
	}
}

func TestNewClientAllowsPlaintextLocalhost(t *testing.T) {
	wallet, _, _ := CreateWallet()
	for _, host := range []string{"http://localhost:8402", "http://127.0.0.1:8402", "http://[::1]:8402"} {
		if _, err := NewClient(wallet, nil, WithGatewayURL(host)); err != nil {
			t.Errorf("loopback URL %q should be allowed: %v", host, err)
		}
	}
}

func TestClientChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			ID:    "chatcmpl-test",
			Model: "gpt-4",
			Choices: []ChatChoice{
				{Index: 0, Message: ChatMessage{Role: RoleAssistant, Content: "Hello!"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil, WithGatewayURL(server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "chatcmpl-test" {
		t.Errorf("id: got %q, want %q", resp.ID, "chatcmpl-test")
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "Hello!")
	}
}

func TestClientChatWithCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := ChatResponse{
			ID:    "chatcmpl-cached",
			Model: "gpt-4",
			Choices: []ChatChoice{
				{Index: 0, Message: ChatMessage{Role: RoleAssistant, Content: "Cached response."}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil,
		WithGatewayURL(server.URL),
		WithCache(true),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req := &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hello"}},
	}

	// First call
	resp1, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	// Second call (should be cached)
	resp2, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d (cache should have prevented second call)", callCount)
	}
	if resp1.ID != resp2.ID {
		t.Errorf("cached response should match: %q != %q", resp1.ID, resp2.ID)
	}
}

func TestClientChat402WithoutSigner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := PaymentRequired{
			X402Version: 2,
			Error:       "payment required",
			CostBreakdown: CostBreakdown{
				Total:    "1000",
				Currency: "USDC",
			},
			Accepts: []PaymentAccept{
				{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "1000", PayTo: "recipient123"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil, WithGatewayURL(server.URL)) // no signer
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	_, ok := err.(*PaymentRequiredError)
	if !ok {
		t.Fatalf("expected PaymentRequiredError, got %T: %v", err, err)
	}
}

func TestClientLastKnownBalance(t *testing.T) {
	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, ok := client.LastKnownBalance(); ok {
		t.Error("expected unset balance initially")
	}

	// Drive through the same code path used by the BalanceMonitor callback
	// so the test exercises the real getter/setter pair, not a direct field
	// assignment that would tautologically pass.
	client.setLastBalance(42.5)

	got, ok := client.LastKnownBalance()
	if !ok {
		t.Fatal("expected balance to be set")
	}
	if got != 42.5 {
		t.Errorf("balance: got %v, want 42.5", got)
	}
}

// TestChatCallsRecordRequestExactlyOnce asserts that one call to Chat
// produces exactly one RecordRequest invocation. With the previous bug,
// GetOrCreate also counted, so three Chat calls escalated immediately;
// after the fix three Chat calls plus the matching three RecordRequest
// increments are needed.
func TestChatCallsRecordRequestExactlyOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ChatResponse{
			ID:    "chat-1",
			Model: "gpt-4",
			Choices: []ChatChoice{
				{Index: 0, Message: ChatMessage{Role: RoleAssistant, Content: "ok"}},
			},
		})
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil,
		WithGatewayURL(server.URL),
		WithSessions(true),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req := &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	}
	for i := 0; i < 3; i++ {
		if _, err := client.Chat(context.Background(), req); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	// After 3 Chat calls with identical messages, three RecordRequests
	// should have fired (one per call). The session is escalated only when
	// the threshold is crossed by RecordRequest, never by GetOrCreate.
	sessionID := DeriveSessionIDWithSalt(client.sessionSalt, req.Messages)
	client.sessionStore.mu.Lock()
	entry, ok := client.sessionStore.sessions[sessionID]
	client.sessionStore.mu.Unlock()
	if !ok {
		t.Fatal("session not stored")
	}
	if got := len(entry.recentHashes); got != 3 {
		t.Errorf("recentHashes len: got %d, want 3", got)
	}
	if !entry.escalated {
		t.Error("expected escalation after 3 identical RecordRequests")
	}
}

// TestChatBalanceGuardSubstitutesFreeModel exercises the wired-up
// BalanceMonitor path: when the monitor reports a zero balance the next
// Chat call must substitute the configured free fallback model.
func TestChatBalanceGuardSubstitutesFreeModel(t *testing.T) {
	var seenModels []string
	var modelMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		modelMu.Lock()
		seenModels = append(seenModels, req.Model)
		modelMu.Unlock()
		json.NewEncoder(w).Encode(ChatResponse{
			ID:    "chat-free",
			Model: req.Model,
			Choices: []ChatChoice{
				{Index: 0, Message: ChatMessage{Role: RoleAssistant, Content: "ok"}},
			},
		})
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()

	// Fetcher returns 0.0 — drives the guard.
	fetcher := func() (float64, error) { return 0.0, nil }

	client, err := NewClient(wallet, nil,
		WithGatewayURL(server.URL),
		WithFreeFallbackModel("free-model"),
		WithBalanceMonitor(20*time.Millisecond, fetcher),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	// Wait for the first poll to land. The monitor polls immediately on
	// Start, but it runs on a separate goroutine so we need to give it a
	// moment.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if bal, ok := client.LastKnownBalance(); ok && bal == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if bal, ok := client.LastKnownBalance(); !ok || bal != 0 {
		t.Fatalf("monitor never reported balance=0 (got=%v ok=%v)", bal, ok)
	}

	if _, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "premium-model",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	}); err != nil {
		t.Fatalf("chat: %v", err)
	}

	modelMu.Lock()
	defer modelMu.Unlock()
	if len(seenModels) != 1 {
		t.Fatalf("server saw %d requests, want 1: %v", len(seenModels), seenModels)
	}
	if seenModels[0] != "free-model" {
		t.Errorf("balance guard did not substitute free model: server saw %q, want %q", seenModels[0], "free-model")
	}
}

// TestChatBalanceGuardDormantWithoutMonitor confirms that without
// WithBalanceMonitor the guard is dormant and the requested model passes
// through verbatim, even with a free fallback configured.
func TestChatBalanceGuardDormantWithoutMonitor(t *testing.T) {
	var seenModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		seenModel = req.Model
		json.NewEncoder(w).Encode(ChatResponse{
			ID:      "x",
			Model:   req.Model,
			Choices: []ChatChoice{{Index: 0, Message: ChatMessage{Role: RoleAssistant, Content: "ok"}}},
		})
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil,
		WithGatewayURL(server.URL),
		WithFreeFallbackModel("free-model"),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	if _, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "premium-model",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	}); err != nil {
		t.Fatalf("chat: %v", err)
	}
	if seenModel != "premium-model" {
		t.Errorf("expected guard dormant; server saw %q, want %q", seenModel, "premium-model")
	}
}

// fakeStreamSigner returns a minimal valid PaymentPayload so ChatStream
// tests can exercise the streaming path without needing real Solana RPC
// connectivity. Used to drive the 402 → sign → re-issue handshake.
type fakeStreamSigner struct{}

func (fakeStreamSigner) SignPayment(_ context.Context, _ uint64, payTo string, resource Resource, accepted PaymentAccept) (*PaymentPayload, error) {
	return &PaymentPayload{
		X402Version: X402Version,
		Resource:    resource,
		Accepted:    accepted,
		Payload:     SolanaPayload{Transaction: "fake-tx"},
	}, nil
}

// TestChatStreamCancelDoesNotLeakGoroutines exercises the SSE goroutine
// fix: the consumer cancels the parent context after reading a single
// chunk and the upstream goroutine plus body must close, not block on a
// channel send forever.
//
// Uses a 402 probe response followed by a successful streaming response so
// the actual transport.SendChatStream goroutine is the one being tested
// for cleanup.
func TestChatStreamCancelDoesNotLeakGoroutines(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		// Non-stream probe: return 402 to force the SDK to sign and re-issue
		// as a streaming request, so the SSE goroutine fix is actually
		// exercised.
		if !req.Stream {
			pr := PaymentRequired{
				X402Version:   X402Version,
				CostBreakdown: CostBreakdown{Total: "100"},
				Resource:      Resource{URL: serverURL + "/v1/chat/completions", Method: "POST"},
				Accepts: []PaymentAccept{
					{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "100", PayTo: "recipient"},
				},
			}
			w.WriteHeader(402)
			json.NewEncoder(w).Encode(pr)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("ResponseWriter does not support flushing")
			return
		}
		ctx := r.Context()
		for i := 0; i < 1000; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			content := "x"
			role := RoleAssistant
			chunk := ChatChunk{
				ID:      "chunk",
				Model:   "gpt-4",
				Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{Role: &role, Content: &content}}},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, fakeStreamSigner{},
		WithGatewayURL(server.URL),
		WithMaxPaymentAmount(1000),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Baseline goroutine count.
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := client.ChatStream(ctx, &ChatRequest{
			Model:    "gpt-4",
			Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
		})
		if err != nil {
			cancel()
			t.Fatalf("ChatStream: %v", err)
		}
		// Read exactly one chunk, then cancel and stop reading.
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatal("never received first chunk")
		}
		cancel()
	}

	// Allow goroutines to unwind. If the fix is missing, the upstream
	// goroutines stay parked on `ch <- chunk` forever and the count keeps
	// climbing.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= baseline+2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	final := runtime.NumGoroutine()
	if final > baseline+2 {
		t.Errorf("goroutine leak suspected: baseline=%d, final=%d (5 abandoned streams)", baseline, final)
	}
}

func TestClientStringRedacts(t *testing.T) {
	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil, WithGatewayURL("https://gateway.example.com"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	s := client.String()
	if !strings.Contains(s, "REDACTED") {
		t.Error("String() should contain REDACTED")
	}
	if !strings.Contains(s, "gateway.example.com") {
		t.Error("String() should contain gateway URL")
	}
	if strings.Contains(s, wallet.Address()) {
		t.Error("String() should not contain wallet address")
	}
}

func TestClientModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := struct {
			Data []ModelInfo `json:"data"`
		}{
			Data: []ModelInfo{
				{ID: "gpt-4", Provider: "openai"},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	client, err := NewClient(wallet, nil, WithGatewayURL(server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("got %d models, want 1", len(models))
	}
	if models[0].ID != "gpt-4" {
		t.Errorf("model id: got %q, want %q", models[0].ID, "gpt-4")
	}
}

func TestClientRecipientMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := PaymentRequired{
			X402Version:   2,
			CostBreakdown: CostBreakdown{Total: "1000"},
			Accepts: []PaymentAccept{
				{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "1000", PayTo: "wrong-recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client, err := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithExpectedRecipient("expected-recipient"),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	_, ok := err.(*RecipientMismatchError)
	if !ok {
		t.Fatalf("expected RecipientMismatchError, got %T: %v", err, err)
	}
}

func TestClientAmountExceedsMax(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := PaymentRequired{
			X402Version:   2,
			CostBreakdown: CostBreakdown{Total: "999999"},
			Accepts: []PaymentAccept{
				{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "999999", PayTo: "recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client, err := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithMaxPaymentAmount(1000),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	_, ok := err.(*AmountExceedsMaxError)
	if !ok {
		t.Fatalf("expected AmountExceedsMaxError, got %T: %v", err, err)
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "valid", input: "1000", want: 1000, wantErr: false},
		{name: "valid_large", input: "999999", want: 999999, wantErr: false},
		{name: "valid_with_whitespace", input: " 42 ", want: 42, wantErr: false},
		{name: "zero_rejected", input: "0", wantErr: true},
		{name: "empty_rejected", input: "", wantErr: true},
		{name: "whitespace_only_rejected", input: "   ", wantErr: true},
		{name: "non_numeric_rejected", input: "abc", wantErr: true},
		{name: "negative_rejected", input: "-1", wantErr: true},
		{name: "decimal_rejected", input: "1.5", wantErr: true},
		{name: "overflow_rejected", input: "99999999999999999999999999", wantErr: true},
		{name: "trailing_garbage_rejected", input: "100abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAmount(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseAmount(%q): expected error, got %d", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseAmount(%q): unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseAmount(%q): got %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestClientRejectsZeroAmountAttack verifies that a malicious gateway
// returning amount="0" is rejected during validation rather than passing
// through to the signer with a zero amount.
func TestClientRejectsZeroAmountAttack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := PaymentRequired{
			X402Version:   2,
			CostBreakdown: CostBreakdown{Total: "0"},
			Accepts: []PaymentAccept{
				{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "0", PayTo: "recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client, err := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithMaxPaymentAmount(1000),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for zero-amount payment")
	}
	ce, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("expected ClientError, got %T: %v", err, err)
	}
	if !strings.Contains(ce.Message, "invalid payment amount") {
		t.Errorf("unexpected ClientError message: %q", ce.Message)
	}
}

// TestClientRejectsNonNumericAmount verifies that non-numeric amounts are
// rejected — previously fmt.Sscanf would treat them as zero and silently
// pass the cap check.
func TestClientRejectsNonNumericAmount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := PaymentRequired{
			X402Version:   2,
			CostBreakdown: CostBreakdown{Total: "junk"},
			Accepts: []PaymentAccept{
				{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "notanumber", PayTo: "recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client, err := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithMaxPaymentAmount(1000),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for non-numeric amount")
	}
	if _, ok := err.(*ClientError); !ok {
		t.Fatalf("expected ClientError, got %T: %v", err, err)
	}
}

// TestClientRejectsForeignResourceURL verifies the SDK refuses to sign a
// payment when the 402 Resource.URL points at an origin different from the
// configured gateway. A rogue gateway must not be able to bind a foreign URL
// into the signed payment metadata.
func TestClientRejectsForeignResourceURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := PaymentRequired{
			X402Version:   2,
			CostBreakdown: CostBreakdown{Total: "100"},
			Resource:      Resource{URL: "https://attacker.example.com/v1/chat/completions", Method: "POST"},
			Accepts: []PaymentAccept{
				{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "100", PayTo: "recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client, err := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithMaxPaymentAmount(1000),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for foreign resource URL")
	}
	ce, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("expected ClientError, got %T: %v", err, err)
	}
	if !strings.Contains(ce.Message, "resource URL origin does not match") {
		t.Errorf("unexpected ClientError message: %q", ce.Message)
	}
}

// TestClientAllowsMatchingResourceURL verifies the origin check passes when
// the Resource.URL matches the configured gateway origin.
func TestClientAllowsMatchingResourceURL(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := PaymentRequired{
			X402Version:   2,
			CostBreakdown: CostBreakdown{Total: "100"},
			Resource:      Resource{URL: serverURL + "/v1/chat/completions", Method: "POST"},
			Accepts: []PaymentAccept{
				{Scheme: "exact", Network: SolanaNetwork, Asset: USDCMint, Amount: "100", PayTo: "recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()
	serverURL = server.URL

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client, err := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithMaxPaymentAmount(1000),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// We expect this path to pass the origin check and reach SignPayment
	// (which the stub signer may fail). Either way, we must NOT see a
	// "resource URL origin does not match" ClientError — that would mean
	// the origin check rejected a legitimate matching URL.
	_, err = client.Chat(context.Background(), &ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}},
	})
	if err != nil {
		if ce, ok := err.(*ClientError); ok {
			if strings.Contains(ce.Message, "resource URL origin does not match") {
				t.Fatalf("origin check incorrectly rejected matching URL: %v", err)
			}
		}
	}
}
