-- 000010_p2p_chain.up.sql
-- On-chain escrow: P2P escrow is now governed by the P2PEscrow smart contract.
-- Add chain bookkeeping columns and make the legacy internal escrow wallet optional.

-- Advertisements: seller's EVM payout address (refund destination).
ALTER TABLE p2p_advertisements
    ADD COLUMN IF NOT EXISTS seller_address TEXT NOT NULL DEFAULT '';

-- Orders: the on-chain escrow wallet is no longer required (escrow lives in the
-- contract). Make the legacy column nullable and add chain bookkeeping fields.
ALTER TABLE p2p_orders
    ALTER COLUMN escrow_wallet_id DROP NOT NULL;

ALTER TABLE p2p_orders
    ADD COLUMN IF NOT EXISTS buyer_address   TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seller_address  TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS on_chain_id     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS escrow_state    TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS create_tx_hash  TEXT,
    ADD COLUMN IF NOT EXISTS release_tx_hash TEXT,
    ADD COLUMN IF NOT EXISTS refund_tx_hash  TEXT,
    ADD COLUMN IF NOT EXISTS dispute_tx_hash TEXT;

CREATE INDEX IF NOT EXISTS idx_p2p_orders_on_chain_id ON p2p_orders(on_chain_id);
