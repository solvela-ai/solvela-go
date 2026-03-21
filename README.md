# rustyclaw-go

Go SDK for [RustyClawRouter](https://github.com/RustyClawRouter/RustyClawRouter) — a Solana-native AI agent payment gateway.

AI agents pay for LLM API calls with USDC-SPL on Solana via the x402 protocol. No API keys, no accounts, just wallets.

## Install

```bash
go get github.com/RustyClawRouter/rustyclaw-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    rcr "github.com/RustyClawRouter/rustyclaw-go"
)

func main() {
    wallet, _, err := rcr.CreateWallet()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Wallet:", wallet.Address())

    client := rcr.NewClient(wallet, nil,
        rcr.WithGatewayURL("http://localhost:8402"),
        rcr.WithCache(true),
        rcr.WithSessions(true),
    )

    resp, err := client.Chat(context.Background(), &rcr.ChatRequest{
        Model: "gpt-4o-mini",
        Messages: []rcr.ChatMessage{
            {Role: rcr.RoleUser, Content: "Hello!"},
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
client := rcr.NewClient(wallet, signer,
    rcr.WithGatewayURL("https://gateway.example.com"),
    rcr.WithTimeout(60 * time.Second),
    rcr.WithCache(true),
    rcr.WithSessions(true),
    rcr.WithQualityCheck(true),
    rcr.WithMaxQualityRetries(2),
    rcr.WithExpectedRecipient("expected-wallet-address"),
    rcr.WithMaxPaymentAmount(100000), // atomic USDC units
    rcr.WithFreeFallbackModel("gpt-4o-mini"),
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
