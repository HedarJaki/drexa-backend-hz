# Drexa P2P On-Chain Escrow

The `P2PEscrow` smart contract is the source of truth for crypto held during a
P2P trade. The Go backend operates it as the **arbiter** (the platform signer):
it funds an escrow when an order is created, releases ETH to the buyer when the
seller confirms the fiat payment, and refunds the seller on cancel / expiry /
dispute resolution.

```
Seller's crypto ── createEscrow() ──▶ [ P2PEscrow holds ETH ]
                                              │
   buyer pays fiat off-chain                  │ release()  ─▶ buyer
   cancel / expiry                            │ refund()   ─▶ seller
   dispute                                    │ resolveDispute(toBuyer)
```

- Contract: [`contracts/P2PEscrow.sol`](contracts/P2PEscrow.sol)
- Go client: [`internal/p2p/chain`](../internal/p2p/chain) (go-ethereum)
- Compiled ABI/bytecode embedded at [`internal/p2p/chain/artifacts`](../internal/p2p/chain/artifacts)

## 1. Run a local EVM node

```bash
cd contracts
npm install            # first time only
npx hardhat node       # exposes http://127.0.0.1:8545, chainId 31337
```

Hardhat prints 20 funded test accounts. Account #0 is the default backend signer
(matches `ESCROW_PRIVATE_KEY` in `.env.example`):

```
Account #0: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
Private Key: 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
```

## 2. Deploy the contract

Pick **one** path.

**A. From Go (no Hardhat needed beyond the node):**

```bash
go run ./cmd/escrow-deploy
# → prints: ESCROW_CONTRACT_ADDRESS=0x...
```

**B. With Hardhat:**

```bash
cd contracts
npx hardhat run scripts/deploy.js --network localhost
```

Either way, copy the printed address into `.env`:

```
ESCROW_CONTRACT_ADDRESS=0x<deployed address>
```

## 3. Start the backend

```bash
go run ./cmd/server
```

On startup you should see `p2p escrow chain client connected`. If escrow env is
missing it logs a warning and P2P escrow endpoints return `503`.

## 4. Public testnet (Sepolia)

1. Get a Sepolia RPC URL (Infura/Alchemy) and a funded test key (Sepolia faucet).
2. In `.env`:
   ```
   ESCROW_RPC_URL=https://sepolia.infura.io/v3/<key>
   ESCROW_CHAIN_ID=11155111
   ESCROW_PRIVATE_KEY=0x<your funded sepolia key>
   ```
3. Deploy: `go run ./cmd/escrow-deploy` (or `npx hardhat run scripts/deploy.js --network sepolia`).
4. Set `ESCROW_CONTRACT_ADDRESS` and restart.

The signer must hold enough ETH to cover **escrow funding + gas** for every order,
because in this custodial model the platform funds escrows on sellers' behalf.

## Contract API (operator-only)

| Function | Effect |
|----------|--------|
| `createEscrow(orderId, buyer, seller)` payable | Lock `msg.value` for an order |
| `markPaid(orderId)` | Record buyer's fiat payment (informational) |
| `release(orderId)` | Send escrow to the buyer |
| `refund(orderId)` | Return escrow to the seller |
| `raiseDispute(orderId)` | Freeze escrow pending resolution |
| `resolveDispute(orderId, toBuyer)` | Send to buyer (`true`) or seller (`false`) |
| `getEscrow(orderId)` view | Read buyer/seller/amount/state |

`orderId` is `keccak256(<order UUID>)`, computed by the Go client.

> **Re-compiling:** if you change `P2PEscrow.sol`, run `npx hardhat compile`, then
> regenerate the embedded artifacts:
> ```bash
> node -e "const a=require('./contracts/artifacts/contracts/P2PEscrow.sol/P2PEscrow.json');const fs=require('fs');fs.writeFileSync('internal/p2p/chain/artifacts/P2PEscrow.abi.json',JSON.stringify(a.abi));fs.writeFileSync('internal/p2p/chain/artifacts/P2PEscrow.bin',a.bytecode.replace(/^0x/,''));"
> ```
> (run from the repo root).
