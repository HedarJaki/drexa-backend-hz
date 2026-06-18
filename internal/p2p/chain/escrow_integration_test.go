package chain

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Hardhat deterministic dev accounts (no real funds).
const (
	hardhatAcct0Key = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" // signer/arbiter
	hardhatAcct1    = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"                         // buyer
	hardhatAcct2    = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"                         // seller
)

// TestEscrowLifecycle exercises deploy → createEscrow → getEscrow → release
// against a real EVM node. Skipped unless ESCROW_IT_RPC is set, e.g.:
//
//	ESCROW_IT_RPC=http://127.0.0.1:8545 go test ./internal/p2p/chain/ -run Lifecycle -v
func TestEscrowLifecycle(t *testing.T) {
	rpc := os.Getenv("ESCROW_IT_RPC")
	if rpc == "" {
		t.Skip("set ESCROW_IT_RPC to a running EVM node to run this integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	addr, _, err := Deploy(ctx, rpc, 0, hardhatAcct0Key, "")
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	t.Logf("deployed P2PEscrow at %s", addr)

	cl, err := New(ctx, Config{RPCURL: rpc, ContractAddress: addr, PrivateKey: hardhatAcct0Key})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	orderID := uuid.NewString()
	amount := EthToWei(0.1)

	if _, err := cl.CreateEscrow(ctx, orderID, hardhatAcct1, hardhatAcct2, amount); err != nil {
		t.Fatalf("createEscrow: %v", err)
	}

	info, err := cl.GetEscrow(ctx, orderID)
	if err != nil {
		t.Fatalf("getEscrow: %v", err)
	}
	if info.State != StateFunded {
		t.Fatalf("expected state funded, got %s", info.State)
	}
	if info.Amount.Cmp(amount) != 0 {
		t.Fatalf("expected amount %s, got %s", amount, info.Amount)
	}
	t.Logf("escrow funded: buyer=%s seller=%s amount=%s", info.Buyer, info.Seller, info.Amount)

	if _, err := cl.Release(ctx, orderID); err != nil {
		t.Fatalf("release: %v", err)
	}

	info, err = cl.GetEscrow(ctx, orderID)
	if err != nil {
		t.Fatalf("getEscrow after release: %v", err)
	}
	if info.State != StateReleased {
		t.Fatalf("expected state released, got %s", info.State)
	}
	t.Logf("escrow released ✓ (state=%s)", info.State)
}
