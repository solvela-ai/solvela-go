//go:build live

package rustyclaw_test

import (
	"context"
	"os"
	"testing"
	"time"

	rustyclaw "github.com/solvela/sdk-go"
)

func TestLiveHealth(t *testing.T) {
	gatewayURL := os.Getenv("RCR_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8402"
	}

	transport := rustyclaw.NewTransport(gatewayURL, 10*time.Second)
	models, err := transport.FetchModels(context.Background())
	if err != nil {
		t.Fatalf("fetch models: %v", err)
	}
	if len(models) == 0 {
		t.Error("expected at least one model")
	}
	t.Logf("found %d models", len(models))
}

func TestLiveChat402(t *testing.T) {
	gatewayURL := os.Getenv("RCR_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8402"
	}

	wallet, _, err := rustyclaw.CreateWallet()
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	client := rustyclaw.NewClient(wallet, nil,
		rustyclaw.WithGatewayURL(gatewayURL),
		rustyclaw.WithTimeout(30*time.Second),
	)

	_, err = client.Chat(context.Background(), &rustyclaw.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []rustyclaw.ChatMessage{
			{Role: rustyclaw.RoleUser, Content: "Say hello in one word."},
		},
	})

	// Expect 402 since we have no payment signer
	if err == nil {
		t.Fatal("expected error (402 payment required)")
	}
	_, ok := err.(*rustyclaw.PaymentRequiredError)
	if !ok {
		t.Logf("got error type %T: %v", err, err)
	}
}
