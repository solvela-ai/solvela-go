// Manual smoke test — exercises the real wire contract before release.
//
// Run before tagging a release to verify the SDK still agrees with a live
// Solvela gateway. Catches wire-format drift that unit/integration tests
// cannot (header names, JSON field renames, accepts-array ordering, new
// required fields).
//
// Usage:
//
//	SOLVELA_GATEWAY_URL=https://staging.solvela.ai \
//	go run ./scripts/smoke
//
// Defaults to http://localhost:8402 if SOLVELA_GATEWAY_URL is unset.
//
// Exit codes:
//
//	0 — all assertions passed
//	1 — an assertion failed or the gateway is unreachable
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	solvela "github.com/solvela-ai/solvela-go"
)

func main() {
	os.Exit(run())
}

func run() int {
	gatewayURL := os.Getenv("SOLVELA_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8402"
	}
	fmt.Printf("Smoke test against: %s\n\n", gatewayURL)

	// No wallet, no signer: smoke probes the gateway's unsigned-request paths
	// (models registry + 402 negotiation), not payment settlement.
	client, err := solvela.NewClient(nil, nil, solvela.WithGatewayURL(gatewayURL))
	if err != nil {
		fmt.Printf("FAIL: client construction rejected gateway URL: %v\n", err)
		return 1
	}
	defer client.Close()

	ctx := context.Background()

	// 1. Models() reaches the gateway and parses the response.
	models, err := client.Models(ctx)
	if err != nil {
		fmt.Printf("FAIL: Models() returned error: %v\n", err)
		return 1
	}
	fmt.Printf("  Models()           -> %d model(s) returned\n", len(models))
	if len(models) == 0 {
		fmt.Println("FAIL: expected at least one model from the gateway")
		return 1
	}
	sample := models[0]
	fmt.Printf("  sample model       -> id=%q ctx=%d in=%g/M out=%g/M\n",
		sample.ID, sample.ContextWindow, sample.InputUsdcPerMillion, sample.OutputUsdcPerMillion)

	// Guard against silent ModelInfo wire drift: every prior shape change
	// surfaced as all-zero pricing / all-false capabilities. Assert at least
	// one model in the registry exposes streaming and paid input pricing.
	anyStreaming := false
	anyPriced := false
	for _, m := range models {
		if m.SupportsStreaming {
			anyStreaming = true
		}
		if m.InputUsdcPerMillion > 0 {
			anyPriced = true
		}
	}
	if !anyStreaming {
		fmt.Println("FAIL: no model reports SupportsStreaming=true — capabilities parsing may be drifted")
		return 1
	}
	if !anyPriced {
		fmt.Println("FAIL: no model reports InputUsdcPerMillion > 0 — pricing parsing may be drifted")
		return 1
	}
	if sample.DisplayName == "" || sample.Provider == "" {
		fmt.Println("FAIL: sample model missing DisplayName or Provider")
		return 1
	}

	// 2. Unsigned chat returns 402 with a parseable PaymentRequired body.
	req := &solvela.ChatRequest{
		Model: sample.ID,
		Messages: []solvela.ChatMessage{
			{Role: solvela.RoleUser, Content: "ping"},
		},
	}
	_, err = client.Chat(ctx, req)
	if err == nil {
		fmt.Println("FAIL: Chat() returned a response with no signer configured (expected 402)")
		return 1
	}
	var prErr *solvela.PaymentRequiredError
	if !errors.As(err, &prErr) {
		fmt.Printf("FAIL: Chat() raised %T (expected *PaymentRequiredError): %v\n", err, err)
		return 1
	}
	pr := prErr.PaymentRequired
	schemes := make([]string, 0, len(pr.Accepts))
	for _, a := range pr.Accepts {
		schemes = append(schemes, string(a.Scheme))
	}
	fmt.Printf("  Chat() unsigned    -> 402 OK (total=%s %s, schemes=%v)\n",
		pr.CostBreakdown.Total, pr.CostBreakdown.Currency, schemes)
	if len(pr.Accepts) == 0 {
		fmt.Println("FAIL: 402 response had empty accepts array")
		return 1
	}

	fmt.Println("\nSmoke test PASSED")
	return 0
}
