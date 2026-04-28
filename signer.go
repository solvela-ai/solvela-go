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

// SignPayment creates a signed payment transaction.
func (s *KeypairSigner) SignPayment(ctx context.Context, amountAtomic uint64, recipient string, resource Resource, accepted PaymentAccept) (*PaymentPayload, error) {
	// Full SPL token signing requires github.com/gagliardetto/solana-go.
	// For now, return a stub error indicating implementation needed.
	return nil, &SignerError{Message: "full SPL token signing not yet implemented — needs github.com/gagliardetto/solana-go"}
}
