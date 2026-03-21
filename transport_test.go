package rustyclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTransportSendChat200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type header")
		}

		resp := ChatResponse{
			ID:    "chatcmpl-test",
			Model: "gpt-4",
			Choices: []ChatChoice{
				{Index: 0, Message: ChatMessage{Role: RoleAssistant, Content: "Hello!"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	req := &ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}}}

	result, err := transport.SendChat(context.Background(), req, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response == nil {
		t.Fatal("expected response, got nil")
	}
	if result.Response.ID != "chatcmpl-test" {
		t.Errorf("id: got %q, want %q", result.Response.ID, "chatcmpl-test")
	}
	if result.PaymentRequired != nil {
		t.Error("expected no payment required")
	}
}

func TestTransportSendChat402(t *testing.T) {
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

	transport := NewTransport(server.URL, 10*time.Second)
	req := &ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}}}

	result, err := transport.SendChat(context.Background(), req, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PaymentRequired == nil {
		t.Fatal("expected payment required")
	}
	if result.PaymentRequired.CostBreakdown.Total != "1000" {
		t.Errorf("total: got %q, want %q", result.PaymentRequired.CostBreakdown.Total, "1000")
	}
}

func TestTransportSendChat500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	req := &ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}}}

	_, err := transport.SendChat(context.Background(), req, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	gatewayErr, ok := err.(*GatewayError)
	if !ok {
		t.Fatalf("expected GatewayError, got %T", err)
	}
	if gatewayErr.Status != 500 {
		t.Errorf("status: got %d, want 500", gatewayErr.Status)
	}
}

func TestTransportSendChatPaymentHeader(t *testing.T) {
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("Payment-Signature")
		resp := ChatResponse{ID: "paid"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	req := &ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}}}

	_, err := transport.SendChat(context.Background(), req, "test-sig-123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedSig != "test-sig-123" {
		t.Errorf("sig: got %q, want %q", receivedSig, "test-sig-123")
	}
}

func TestTransportSendChatExtraHeaders(t *testing.T) {
	var receivedCustom string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCustom = r.Header.Get("X-Custom")
		json.NewEncoder(w).Encode(ChatResponse{ID: "test"})
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	req := &ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}}}
	headers := map[string]string{"X-Custom": "custom-value"}

	_, err := transport.SendChat(context.Background(), req, "", headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedCustom != "custom-value" {
		t.Errorf("custom header: got %q, want %q", receivedCustom, "custom-value")
	}
}

func TestTransportSendChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		content := "Hello"
		chunk := ChatChunk{
			ID:    "chunk-1",
			Model: "gpt-4",
			Choices: []ChatChunkChoice{
				{Index: 0, Delta: ChatDelta{Content: &content}},
			},
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	req := &ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}}}

	ch, err := transport.SendChatStream(context.Background(), req, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chunks []ChatChunk
	for item := range ch {
		if item.Err != nil {
			t.Fatalf("chunk error: %v", item.Err)
		}
		chunks = append(chunks, *item.Chunk)
	}

	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].ID != "chunk-1" {
		t.Errorf("chunk id: got %q, want %q", chunks[0].ID, "chunk-1")
	}
}

func TestTransportSendChatStream402(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(PaymentRequired{X402Version: 2})
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	req := &ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: RoleUser, Content: "Hi"}}}

	_, err := transport.SendChatStream(context.Background(), req, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	_, ok := err.(*PaymentRequiredError)
	if !ok {
		t.Fatalf("expected PaymentRequiredError, got %T: %v", err, err)
	}
}

func TestTransportFetchModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		result := struct {
			Data []ModelInfo `json:"data"`
		}{
			Data: []ModelInfo{
				{ID: "gpt-4", Provider: "openai", DisplayName: "GPT-4"},
				{ID: "claude-3", Provider: "anthropic", DisplayName: "Claude 3"},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	models, err := transport.FetchModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].ID != "gpt-4" {
		t.Errorf("model[0].ID: got %q, want %q", models[0].ID, "gpt-4")
	}
}

func TestTransportFetchModels500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	transport := NewTransport(server.URL, 10*time.Second)
	_, err := transport.FetchModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	gatewayErr, ok := err.(*GatewayError)
	if !ok {
		t.Fatalf("expected GatewayError, got %T", err)
	}
	if gatewayErr.Status != 500 {
		t.Errorf("status: got %d, want 500", gatewayErr.Status)
	}
}

func TestTransportBaseURLTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ChatResponse{ID: "test"})
	}))
	defer server.Close()

	transport := NewTransport(server.URL+"/", 10*time.Second)
	if transport.baseURL != server.URL {
		t.Errorf("trailing slash not trimmed: got %q", transport.baseURL)
	}
}
