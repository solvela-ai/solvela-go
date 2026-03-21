package rustyclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientCreation(t *testing.T) {
	wallet, _, err := CreateWallet()
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	client := NewClient(wallet, nil,
		WithGatewayURL("https://example.com"),
		WithTimeout(30*time.Second),
	)

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
	client := NewClient(wallet, nil, WithCache(true))
	if client.cache == nil {
		t.Error("cache should be enabled")
	}
}

func TestNewClientWithSessions(t *testing.T) {
	wallet, _, _ := CreateWallet()
	client := NewClient(wallet, nil, WithSessions(true))
	if client.sessionStore == nil {
		t.Error("session store should be enabled")
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
	client := NewClient(wallet, nil, WithGatewayURL(server.URL))

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
	client := NewClient(wallet, nil,
		WithGatewayURL(server.URL),
		WithCache(true),
	)

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
				{Scheme: "exact", Network: SolanaNetwork, Amount: "1000", PayTo: "recipient123"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	client := NewClient(wallet, nil, WithGatewayURL(server.URL)) // no signer

	_, err := client.Chat(context.Background(), &ChatRequest{
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
	client := NewClient(wallet, nil)

	if client.LastKnownBalance() != nil {
		t.Error("expected nil balance initially")
	}

	balance := 42.5
	client.lastBalance = &balance

	got := client.LastKnownBalance()
	if got == nil || *got != 42.5 {
		t.Errorf("balance: got %v, want 42.5", got)
	}
}

func TestClientStringRedacts(t *testing.T) {
	wallet, _, _ := CreateWallet()
	client := NewClient(wallet, nil, WithGatewayURL("https://gateway.example.com"))

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
	client := NewClient(wallet, nil, WithGatewayURL(server.URL))

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
				{Scheme: "exact", Network: SolanaNetwork, Amount: "1000", PayTo: "wrong-recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithExpectedRecipient("expected-recipient"),
	)

	_, err := client.Chat(context.Background(), &ChatRequest{
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
				{Scheme: "exact", Network: SolanaNetwork, Amount: "999999", PayTo: "recipient"},
			},
		}
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(pr)
	}))
	defer server.Close()

	wallet, _, _ := CreateWallet()
	signer := NewKeypairSigner(wallet, "")
	client := NewClient(wallet, signer,
		WithGatewayURL(server.URL),
		WithMaxPaymentAmount(1000),
	)

	_, err := client.Chat(context.Background(), &ChatRequest{
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
		input string
		want  uint64
	}{
		{"1000", 1000},
		{"0", 0},
		{"999999", 999999},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseAmount(tt.input)
		if got != tt.want {
			t.Errorf("parseAmount(%q): got %d, want %d", tt.input, got, tt.want)
		}
	}
}
