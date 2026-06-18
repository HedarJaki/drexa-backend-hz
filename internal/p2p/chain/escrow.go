// Package chain is the on-chain integration for P2P escrow. It wraps the
// P2PEscrow Solidity contract via go-ethereum, exposing a small string-based
// API so the P2P usecase layer never touches go-ethereum types directly.
//
// The platform backend is the contract's sole operator (the "arbiter"): it
// funds escrows on a seller's behalf and releases/refunds them, matching the
// custodial model where users never hold on-chain keys.
package chain

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

//go:embed artifacts/P2PEscrow.abi.json
var escrowABIJSON string

//go:embed artifacts/P2PEscrow.bin
var escrowBinHex string

// ErrNotConfigured is returned by the disabled client when escrow env is unset.
var ErrNotConfigured = errors.New("p2p escrow chain client not configured")

// EscrowState mirrors the on-chain P2PEscrow.State enum.
type EscrowState uint8

const (
	StateNone EscrowState = iota
	StateFunded
	StatePaid
	StateReleased
	StateRefunded
	StateDisputed
)

func (s EscrowState) String() string {
	switch s {
	case StateNone:
		return "none"
	case StateFunded:
		return "funded"
	case StatePaid:
		return "paid"
	case StateReleased:
		return "released"
	case StateRefunded:
		return "refunded"
	case StateDisputed:
		return "disputed"
	default:
		return "unknown"
	}
}

// EscrowInfo is the decoded result of an on-chain getEscrow call.
type EscrowInfo struct {
	Buyer     string
	Seller    string
	Amount    *big.Int // wei
	State     EscrowState
	CreatedAt uint64
}

// EscrowClient operates the on-chain P2P escrow. Order IDs are app-level UUID
// strings; the client hashes them to the contract's bytes32 key internally.
// Addresses are hex strings; amounts are in wei.
type EscrowClient interface {
	// Enabled reports whether a live chain client is configured.
	Enabled() bool
	// OrderHash returns the on-chain bytes32 key (hex) for an order UUID — useful
	// to persist alongside the order for traceability.
	OrderHash(orderID string) string

	CreateEscrow(ctx context.Context, orderID, buyerAddr, sellerAddr string, amountWei *big.Int) (txHash string, err error)
	MarkPaid(ctx context.Context, orderID string) (txHash string, err error)
	Release(ctx context.Context, orderID string) (txHash string, err error)
	Refund(ctx context.Context, orderID string) (txHash string, err error)
	RaiseDispute(ctx context.Context, orderID string) (txHash string, err error)
	ResolveDispute(ctx context.Context, orderID string, toBuyer bool) (txHash string, err error)
	GetEscrow(ctx context.Context, orderID string) (EscrowInfo, error)
}

// Config holds the connection + signer parameters for the escrow client.
type Config struct {
	RPCURL          string
	ChainID         int64
	ContractAddress string
	// PrivateKey is the platform signer (the contract's arbiter). Hex, 0x optional.
	// SECURITY: only used to sign escrow transactions; never log or expose it.
	PrivateKey string
}

type ethEscrowClient struct {
	backend  *ethclient.Client
	contract *bind.BoundContract
	abi      abi.ABI
	chainID  *big.Int
	key      ethKey
	address  common.Address // contract address

	mu sync.Mutex // serialize transactions to avoid nonce races
}

// New builds a live escrow client. Returns ErrNotConfigured if required fields
// are empty so callers can fall back to NewDisabled.
func New(ctx context.Context, cfg Config) (EscrowClient, error) {
	if cfg.RPCURL == "" || cfg.ContractAddress == "" || cfg.PrivateKey == "" {
		return nil, ErrNotConfigured
	}
	if !common.IsHexAddress(cfg.ContractAddress) {
		return nil, fmt.Errorf("chain: invalid contract address %q", cfg.ContractAddress)
	}

	parsedABI, err := abi.JSON(strings.NewReader(escrowABIJSON))
	if err != nil {
		return nil, fmt.Errorf("chain: parse abi: %w", err)
	}

	key, err := parseKey(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}

	backend, err := ethclient.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("chain: dial %s: %w", cfg.RPCURL, err)
	}

	chainID := big.NewInt(cfg.ChainID)
	if cfg.ChainID == 0 {
		// Auto-detect when not pinned (handy for local nodes / Sepolia).
		id, err := backend.ChainID(ctx)
		if err != nil {
			backend.Close()
			return nil, fmt.Errorf("chain: detect chain id: %w", err)
		}
		chainID = id
	}

	addr := common.HexToAddress(cfg.ContractAddress)
	contract := bind.NewBoundContract(addr, parsedABI, backend, backend, backend)

	return &ethEscrowClient{
		backend:  backend,
		contract: contract,
		abi:      parsedABI,
		chainID:  chainID,
		key:      key,
		address:  addr,
	}, nil
}

func (c *ethEscrowClient) Enabled() bool { return true }

func (c *ethEscrowClient) OrderHash(orderID string) string {
	h := orderHash(orderID)
	return hexutil.Encode(h[:])
}

func (c *ethEscrowClient) CreateEscrow(ctx context.Context, orderID, buyerAddr, sellerAddr string, amountWei *big.Int) (string, error) {
	if !common.IsHexAddress(buyerAddr) {
		return "", fmt.Errorf("chain: invalid buyer address %q", buyerAddr)
	}
	if !common.IsHexAddress(sellerAddr) {
		return "", fmt.Errorf("chain: invalid seller address %q", sellerAddr)
	}
	if amountWei == nil || amountWei.Sign() <= 0 {
		return "", errors.New("chain: escrow amount must be > 0")
	}
	return c.send(ctx, amountWei, "createEscrow",
		orderHash(orderID), common.HexToAddress(buyerAddr), common.HexToAddress(sellerAddr))
}

func (c *ethEscrowClient) MarkPaid(ctx context.Context, orderID string) (string, error) {
	return c.send(ctx, nil, "markPaid", orderHash(orderID))
}

func (c *ethEscrowClient) Release(ctx context.Context, orderID string) (string, error) {
	return c.send(ctx, nil, "release", orderHash(orderID))
}

func (c *ethEscrowClient) Refund(ctx context.Context, orderID string) (string, error) {
	return c.send(ctx, nil, "refund", orderHash(orderID))
}

func (c *ethEscrowClient) RaiseDispute(ctx context.Context, orderID string) (string, error) {
	return c.send(ctx, nil, "raiseDispute", orderHash(orderID))
}

func (c *ethEscrowClient) ResolveDispute(ctx context.Context, orderID string, toBuyer bool) (string, error) {
	return c.send(ctx, nil, "resolveDispute", orderHash(orderID), toBuyer)
}

func (c *ethEscrowClient) GetEscrow(ctx context.Context, orderID string) (EscrowInfo, error) {
	var out []interface{}
	err := c.contract.Call(&bind.CallOpts{Context: ctx}, &out, "getEscrow", orderHash(orderID))
	if err != nil {
		return EscrowInfo{}, fmt.Errorf("chain: getEscrow: %w", err)
	}
	if len(out) != 5 {
		return EscrowInfo{}, fmt.Errorf("chain: getEscrow returned %d values", len(out))
	}
	info := EscrowInfo{
		Buyer:     out[0].(common.Address).Hex(),
		Seller:    out[1].(common.Address).Hex(),
		Amount:    out[2].(*big.Int),
		State:     EscrowState(out[3].(uint8)),
		CreatedAt: out[4].(uint64),
	}
	return info, nil
}

// send signs and submits a transaction, then waits for it to be mined and
// checks that it did not revert. value is the attached ETH (nil for non-payable).
func (c *ethEscrowClient) send(ctx context.Context, value *big.Int, method string, args ...interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	opts, err := bind.NewKeyedTransactorWithChainID(c.key.priv, c.chainID)
	if err != nil {
		return "", fmt.Errorf("chain: build tx opts: %w", err)
	}
	opts.Context = ctx
	if value != nil {
		opts.Value = value
	}

	tx, err := c.contract.Transact(opts, method, args...)
	if err != nil {
		return "", fmt.Errorf("chain: %s: %w", method, err)
	}

	receipt, err := bind.WaitMined(ctx, c.backend, tx)
	if err != nil {
		return tx.Hash().Hex(), fmt.Errorf("chain: %s wait mined: %w", method, err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return tx.Hash().Hex(), fmt.Errorf("chain: %s reverted (tx %s)", method, tx.Hash().Hex())
	}
	return tx.Hash().Hex(), nil
}

// orderHash maps an app-level order UUID to the contract's bytes32 key.
func orderHash(orderID string) [32]byte {
	return crypto.Keccak256Hash([]byte(orderID))
}
