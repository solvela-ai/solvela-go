# solvela-go

Go SDK for [Solvela](https://solvela.ai) — a Solana-native AI agent payment gateway.

AI agents pay for LLM API calls with USDC-SPL on Solana via the x402 protocol. No API keys, no accounts, just wallets.

## Install

```bash
go get github.com/solvela-ai/solvela-go
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	solvela "github.com/solvela-ai/solvela-go"
)

func main() {
	wallet, _, err := solvela.CreateWallet()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Wallet:", wallet.Address())

	client := solvela.NewClient(wallet, nil,
		solvela.WithGatewayURL("https://api.solvela.ai"),
		solvela.WithCache(true),
		solvela.WithSessions(true),
	)

	resp, err := client.Chat(context.Background(), &solvela.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []solvela.ChatMessage{
			{Role: solvela.RoleUser, Content: "Hello!"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Choices[0].Message.Content)
}
```

## Status

The core SDK (transport, caching, sessions, quality checking, streaming, balance monitoring) is fully implemented and tested.

**`KeypairSigner` is not yet implemented.** The bundled `KeypairSigner` type returns an error when `SignPayment` is called — it is a placeholder, not a working signer. To make payments you have two options:

1. **Use a different SDK** — the [Python SDK](https://github.com/solvela-ai/solvela-python) and TypeScript SDK include working `KeypairSigner` implementations backed by their respective Solana libraries.
2. **Implement a custom `Signer`** — the `Signer` interface is pluggable. Provide your own implementation using `crypto/ed25519` (already in the Go standard library) and a Solana JSON-RPC client of your choice.

```go
type MySigner struct{ wallet *solvela.Wallet }

func (s *MySigner) SignPayment(ctx context.Context, amount uint64, recipient string, resource solvela.Resource, accepted solvela.PaymentAccept) (*solvela.PaymentPayload, error) {
    // build and sign a USDC-SPL transfer transaction, return PaymentPayload
}

client := solvela.NewClient(wallet, &MySigner{wallet: wallet}, ...)
```

## Features

- Automatic x402 payment flow (402 detection, signing, retry)
- Response caching with LRU eviction and dedup window
- Session tracking with three-strike model escalation
- Quality checking with automatic retry on degraded responses
- SSE streaming support
- Balance monitoring with low-balance callbacks
- Pluggable `Signer` interface for custom payment signing

## Configuration

```go
client := solvela.NewClient(wallet, signer,
	solvela.WithGatewayURL("https://api.solvela.ai"),
	solvela.WithTimeout(60 * time.Second),
	solvela.WithCache(true),
	solvela.WithSessions(true),
	solvela.WithQualityCheck(true),
	solvela.WithMaxQualityRetries(2),
	solvela.WithExpectedRecipient("expected-wallet-address"),
	solvela.WithMaxPaymentAmount(100000), // atomic USDC units
	solvela.WithFreeFallbackModel("gpt-4o-mini"),
)
```

## Testing

```bash
go test ./... -v -count=1

# Live tests (requires running gateway)
go test ./... -v -tags=live
```

## License

MIT
