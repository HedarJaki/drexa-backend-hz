-- 000013_p2p_usd.down.sql
ALTER TABLE p2p_orders RENAME COLUMN total_usd TO total_idr;
