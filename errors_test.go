package solvela

import (
	"errors"
	"testing"
)

func TestClientError(t *testing.T) {
	err := &ClientError{Message: "something went wrong"}
	if err.Error() != "something went wrong" {
		t.Errorf("got %q", err.Error())
	}
}

func TestWalletError(t *testing.T) {
	err := &WalletError{Message: "invalid key"}
	if err.Error() != "wallet error: invalid key" {
		t.Errorf("got %q", err.Error())
	}
}

func TestSignerError(t *testing.T) {
	err := &SignerError{Message: "sign failed"}
	if err.Error() != "signer error: sign failed" {
		t.Errorf("got %q", err.Error())
	}
}

func TestInsufficientBalanceError(t *testing.T) {
	err := &InsufficientBalanceError{Have: 100, Need: 500}
	if err.Error() != "insufficient balance: have 100, need 500" {
		t.Errorf("got %q", err.Error())
	}
}

func TestGatewayError(t *testing.T) {
	err := &GatewayError{Status: 500, Message: "internal error"}
	if err.Error() != "gateway error 500: internal error" {
		t.Errorf("got %q", err.Error())
	}
}

func TestPaymentRequiredError(t *testing.T) {
	err := &PaymentRequiredError{
		PaymentRequired: PaymentRequired{
			CostBreakdown: CostBreakdown{Total: "1000"},
		},
	}
	if err.Error() != "payment required: 1000 USDC" {
		t.Errorf("got %q", err.Error())
	}
}

func TestPaymentRejectedError(t *testing.T) {
	err := &PaymentRejectedError{Reason: "invalid signature"}
	if err.Error() != "payment rejected: invalid signature" {
		t.Errorf("got %q", err.Error())
	}
}

func TestRecipientMismatchError(t *testing.T) {
	err := &RecipientMismatchError{Expected: "abc", Actual: "xyz"}
	if err.Error() != "recipient mismatch: expected abc, got xyz" {
		t.Errorf("got %q", err.Error())
	}
}

func TestAmountExceedsMaxError(t *testing.T) {
	err := &AmountExceedsMaxError{Amount: 1000, MaxAmount: 500}
	if err.Error() != "amount 1000 exceeds max 500" {
		t.Errorf("got %q", err.Error())
	}
}

func TestTimeoutError(t *testing.T) {
	err := &TimeoutError{TimeoutSecs: 30.0}
	if err.Error() != "request timed out after 30.0s" {
		t.Errorf("got %q", err.Error())
	}
}

func TestErrorTypeAssertions(t *testing.T) {
	var err error

	err = &ClientError{Message: "test"}
	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Error("expected ClientError type assertion to succeed")
	}

	err = &GatewayError{Status: 402, Message: "payment required"}
	var gwErr *GatewayError
	if !errors.As(err, &gwErr) {
		t.Error("expected GatewayError type assertion to succeed")
	}
	if gwErr.Status != 402 {
		t.Errorf("status: got %d, want 402", gwErr.Status)
	}

	err = &InsufficientBalanceError{Have: 0, Need: 100}
	var balErr *InsufficientBalanceError
	if !errors.As(err, &balErr) {
		t.Error("expected InsufficientBalanceError type assertion to succeed")
	}
}
