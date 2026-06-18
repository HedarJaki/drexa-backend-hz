-- 000010_p2p_chain.down.sql
-- Revert on-chain escrow columns.

DROP INDEX IF EXISTS idx_p2p_orders_on_chain_id;

ALTER TABLE p2p_orders
    DROP COLUMN IF EXISTS buyer_address,
    DROP COLUMN IF EXISTS seller_address,
    DROP COLUMN IF EXISTS on_chain_id,
    DROP COLUMN IF EXISTS escrow_state,
    DROP COLUMN IF EXISTS create_tx_hash,
    DROP COLUMN IF EXISTS release_tx_hash,
    DROP COLUMN IF EXISTS refund_tx_hash,
    DROP COLUMN IF EXISTS dispute_tx_hash;

-- Note: escrow_wallet_id is left nullable on rollback (re-imposing NOT NULL
-- could fail if NULL rows exist). Restore manually if strictly required.

ALTER TABLE p2p_advertisements
    DROP COLUMN IF EXISTS seller_address;
