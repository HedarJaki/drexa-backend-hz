package chain

import (
	"context"
	"math/big"
)

// disabledClient is used when escrow chain config is absent. Every operation
// fails with ErrNotConfigured so P2P endpoints degrade clearly instead of
// silently skipping the on-chain escrow.
type disabledClient struct{}

// NewDisabled returns an EscrowClient that reports Enabled()==false and errors
// on every on-chain operation.
func NewDisabled() EscrowClient { return disabledClient{} }

func (disabledClient) Enabled() bool { return false }

func (disabledClient) OrderHash(string) string { return "" }

func (disabledClient) CreateEscrow(context.Context, string, string, string, *big.Int) (string, error) {
	return "", ErrNotConfigured
}

func (disabledClient) MarkPaid(context.Context, string) (string, error) {
	return "", ErrNotConfigured
}

func (disabledClient) Release(context.Context, string) (string, error) {
	return "", ErrNotConfigured
}

func (disabledClient) Refund(context.Context, string) (string, error) {
	return "", ErrNotConfigured
}

func (disabledClient) RaiseDispute(context.Context, string) (string, error) {
	return "", ErrNotConfigured
}

func (disabledClient) ResolveDispute(context.Context, string, bool) (string, error) {
	return "", ErrNotConfigured
}

func (disabledClient) GetEscrow(context.Context, string) (EscrowInfo, error) {
	return EscrowInfo{}, ErrNotConfigured
}
