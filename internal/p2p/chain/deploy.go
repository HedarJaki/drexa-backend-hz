package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Deploy publishes a fresh P2PEscrow contract and returns its address. The
// arbiter is set to arbiterAddr, or to the deployer's own address when empty.
// This lets the project deploy with only Go + a running node (no Hardhat needed).
func Deploy(ctx context.Context, rpcURL string, chainID int64, privKeyHex, arbiterAddr string) (contractAddr string, txHash string, err error) {
	parsedABI, err := abi.JSON(strings.NewReader(escrowABIJSON))
	if err != nil {
		return "", "", fmt.Errorf("chain: parse abi: %w", err)
	}
	bytecode, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(escrowBinHex), "0x"))
	if err != nil {
		return "", "", fmt.Errorf("chain: decode bytecode: %w", err)
	}

	key, err := parseKey(privKeyHex)
	if err != nil {
		return "", "", err
	}

	backend, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return "", "", fmt.Errorf("chain: dial %s: %w", rpcURL, err)
	}
	defer backend.Close()

	id := big.NewInt(chainID)
	if chainID == 0 {
		id, err = backend.ChainID(ctx)
		if err != nil {
			return "", "", fmt.Errorf("chain: detect chain id: %w", err)
		}
	}

	opts, err := bind.NewKeyedTransactorWithChainID(key.priv, id)
	if err != nil {
		return "", "", fmt.Errorf("chain: build tx opts: %w", err)
	}
	opts.Context = ctx

	arbiter := key.addr
	if arbiterAddr != "" {
		if !common.IsHexAddress(arbiterAddr) {
			return "", "", fmt.Errorf("chain: invalid arbiter address %q", arbiterAddr)
		}
		arbiter = common.HexToAddress(arbiterAddr)
	}

	addr, tx, _, err := bind.DeployContract(opts, parsedABI, bytecode, backend, arbiter)
	if err != nil {
		return "", "", fmt.Errorf("chain: deploy: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, backend, tx)
	if err != nil {
		return addr.Hex(), tx.Hash().Hex(), fmt.Errorf("chain: deploy wait mined: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return addr.Hex(), tx.Hash().Hex(), fmt.Errorf("chain: deploy reverted (tx %s)", tx.Hash().Hex())
	}
	return addr.Hex(), tx.Hash().Hex(), nil
}
