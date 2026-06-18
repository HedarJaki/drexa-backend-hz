// Command escrow-deploy publishes the P2PEscrow contract using the backend's
// own escrow config (.env). It needs only a running EVM node + a funded signer
// key — no Hardhat/Foundry required.
//
//	go run ./cmd/escrow-deploy                 # arbiter = deployer (the signer)
//	go run ./cmd/escrow-deploy -arbiter 0xABC  # explicit arbiter address
//
// Then copy the printed ESCROW_CONTRACT_ADDRESS into .env and restart the server.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"drexa/internal/p2p/chain"
	"drexa/pkg/config"
)

func main() {
	arbiter := flag.String("arbiter", "", "arbiter address (defaults to the deployer/signer)")
	flag.Parse()

	cfg := config.Load()
	if cfg.Escrow.RPCURL == "" || cfg.Escrow.PrivateKey == "" {
		fmt.Fprintln(os.Stderr, "error: ESCROW_RPC_URL and ESCROW_PRIVATE_KEY must be set in .env")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("Deploying P2PEscrow to %s ...\n", cfg.Escrow.RPCURL)
	addr, tx, err := chain.Deploy(ctx, cfg.Escrow.RPCURL, cfg.Escrow.ChainID, cfg.Escrow.PrivateKey, *arbiter)
	if err != nil {
		fmt.Fprintln(os.Stderr, "deploy failed:", err)
		os.Exit(1)
	}

	fmt.Println("\nP2PEscrow deployed!")
	fmt.Println("  address:", addr)
	fmt.Println("  tx:     ", tx)
	fmt.Println("\nAdd this to your .env and restart the server:")
	fmt.Println("  ESCROW_CONTRACT_ADDRESS=" + addr)
}
