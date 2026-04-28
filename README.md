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
