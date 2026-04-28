package solvela

import "context"

// Signer is a pluggable interface for signing payment transactions.
type Signer interface {
	SignPayment(ctx context.Context, amountAtomic uint64, recipient string, resource Resource, accepted PaymentAccept) (*PaymentPayload, error)
}

// KeypairSigner signs real Solana USDC-SPL transfer transactions.
type KeypairSigner struct {
	wallet *Wallet
	rpcURL string
}

// NewKeypairSigner creates a signer from a wallet and optional Solana RPC URL.
// If rpcURL is empty, defaults to mainnet-beta.
func NewKeypairSigner(wallet *Wallet, rpcURL string) *KeypairSigner {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	return &KeypairSigner{wallet: wallet, rpcURL: rpcURL}
}

// SignPayment is not yet implemented in solvela-go.
//
// Use the Python SDK (solvela-python) or TypeScript SDK (solvela-ts) for
// production payment signing, or implement a custom [Signer] using the
// standard library's crypto/ed25519 package and a Solana JSON-RPC client
// of your choice.
func (s *KeypairSigner) SignPayment(_ context.Context, _ uint64, _ string, _ Resource, _ PaymentAccept) (*PaymentPayload, error) {
	return nil, &SignerError{Message: "KeypairSigner.SignPayment is not yet implemented in solvela-go; use the Python or TS SDK, or implement a custom Signer"}
}
