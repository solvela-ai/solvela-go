package rustyclaw

import (
	"encoding/json"
	"testing"
)

func TestChatMessageRoundtrip(t *testing.T) {
	name := "test-user"
	msg := ChatMessage{
		Role:    RoleUser,
		Content: "Hello, world!",
		Name:    &name,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ChatMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Role != msg.Role {
		t.Errorf("role: got %q, want %q", decoded.Role, msg.Role)
	}
	if decoded.Content != msg.Content {
		t.Errorf("content: got %q, want %q", decoded.Content, msg.Content)
	}
	if decoded.Name == nil || *decoded.Name != name {
		t.Errorf("name: got %v, want %q", decoded.Name, name)
	}
}

func TestChatMessageOmitempty(t *testing.T) {
	msg := ChatMessage{
		Role:    RoleAssistant,
		Content: "Hi",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	raw := string(data)
	// Optional fields should be omitted
	for _, field := range []string{"name", "tool_calls", "tool_call_id"} {
		if contains(raw, `"`+field+`"`) {
			t.Errorf("expected field %q to be omitted, got: %s", field, raw)
		}
	}
}

func TestChatResponseRoundtrip(t *testing.T) {
	finishReason := "stop"
	resp := ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "gpt-4",
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    RoleAssistant,
					Content: "Hello!",
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ChatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != resp.ID {
		t.Errorf("id: got %q, want %q", decoded.ID, resp.ID)
	}
	if decoded.Model != resp.Model {
		t.Errorf("model: got %q, want %q", decoded.Model, resp.Model)
	}
	if len(decoded.Choices) != 1 {
		t.Fatalf("choices len: got %d, want 1", len(decoded.Choices))
	}
	if decoded.Choices[0].Message.Content != "Hello!" {
		t.Errorf("content: got %q, want %q", decoded.Choices[0].Message.Content, "Hello!")
	}
	if decoded.Usage == nil || decoded.Usage.TotalTokens != 15 {
		t.Errorf("usage: got %v", decoded.Usage)
	}
}

func TestChatResponseOmitUsage(t *testing.T) {
	resp := ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "gpt-4",
		Choices: []ChatChoice{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if contains(string(data), `"usage"`) {
		t.Error("expected usage to be omitted when nil")
	}
}

func TestPaymentRequiredRoundtrip(t *testing.T) {
	pr := PaymentRequired{
		X402Version: 2,
		Resource:    Resource{URL: "/v1/chat/completions", Method: "POST"},
		Accepts: []PaymentAccept{
			{
				Scheme:            "exact",
				Network:           SolanaNetwork,
				Amount:            "1000",
				Asset:             USDCMint,
				PayTo:             "11111111111111111111111111111112",
				MaxTimeoutSeconds: 300,
			},
		},
		CostBreakdown: CostBreakdown{
			ProviderCost: "950",
			PlatformFee:  "50",
			Total:        "1000",
			Currency:     "USDC",
			FeePercent:   5,
		},
		Error: "Payment required",
	}

	data, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PaymentRequired
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.X402Version != 2 {
		t.Errorf("version: got %d, want 2", decoded.X402Version)
	}
	if decoded.CostBreakdown.Total != "1000" {
		t.Errorf("total: got %q, want %q", decoded.CostBreakdown.Total, "1000")
	}
	if decoded.Accepts[0].Scheme != "exact" {
		t.Errorf("scheme: got %q, want %q", decoded.Accepts[0].Scheme, "exact")
	}
}

func TestPaymentAcceptOmitEscrowProgramID(t *testing.T) {
	pa := PaymentAccept{
		Scheme:            "exact",
		Network:           SolanaNetwork,
		Amount:            "1000",
		Asset:             USDCMint,
		PayTo:             "test",
		MaxTimeoutSeconds: 300,
	}

	data, err := json.Marshal(pa)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if contains(string(data), `"escrow_program_id"`) {
		t.Error("expected escrow_program_id to be omitted when nil")
	}
}

func TestModelInfoRoundtrip(t *testing.T) {
	maxOut := 4096
	mi := ModelInfo{
		ID:                       "openai/gpt-4",
		Provider:                 "openai",
		ModelID:                  "gpt-4",
		DisplayName:              "GPT-4",
		InputCostPerMillion:      30.0,
		OutputCostPerMillion:     60.0,
		ContextWindow:            128000,
		SupportsStreaming:        true,
		SupportsTools:            true,
		SupportsVision:           true,
		Reasoning:                false,
		SupportsStructuredOutput: true,
		SupportsBatch:            false,
		MaxOutputTokens:          &maxOut,
	}

	data, err := json.Marshal(mi)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ModelInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != mi.ID {
		t.Errorf("id: got %q, want %q", decoded.ID, mi.ID)
	}
	if decoded.InputCostPerMillion != 30.0 {
		t.Errorf("input cost: got %f, want 30.0", decoded.InputCostPerMillion)
	}
	if decoded.MaxOutputTokens == nil || *decoded.MaxOutputTokens != 4096 {
		t.Errorf("max output tokens: got %v, want 4096", decoded.MaxOutputTokens)
	}
}

func TestModelInfoOmitMaxOutputTokens(t *testing.T) {
	mi := ModelInfo{
		ID:       "test",
		Provider: "test",
		ModelID:  "test",
	}

	data, err := json.Marshal(mi)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if contains(string(data), `"max_output_tokens"`) {
		t.Error("expected max_output_tokens to be omitted when nil")
	}
}

func TestConstants(t *testing.T) {
	if X402Version != 2 {
		t.Errorf("X402Version: got %d, want 2", X402Version)
	}
	if USDCMint != "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" {
		t.Errorf("USDCMint: got %q", USDCMint)
	}
	if SolanaNetwork != "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp" {
		t.Errorf("SolanaNetwork: got %q", SolanaNetwork)
	}
	if MaxTimeoutSeconds != 300 {
		t.Errorf("MaxTimeoutSeconds: got %d, want 300", MaxTimeoutSeconds)
	}
	if PlatformFeePercent != 5 {
		t.Errorf("PlatformFeePercent: got %d, want 5", PlatformFeePercent)
	}
}

func TestJSONSnakeCaseFields(t *testing.T) {
	resp := ChatResponse{
		ID:      "test",
		Object:  "chat.completion",
		Created: 123,
		Model:   "gpt-4",
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    RoleAssistant,
					Content: "hi",
				},
			},
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	raw := string(data)
	for _, field := range []string{"prompt_tokens", "completion_tokens", "total_tokens", "finish_reason"} {
		// finish_reason is omitempty nil, so skip that check
		if field == "finish_reason" {
			continue
		}
		if !contains(raw, `"`+field+`"`) {
			t.Errorf("expected snake_case field %q in JSON: %s", field, raw)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
