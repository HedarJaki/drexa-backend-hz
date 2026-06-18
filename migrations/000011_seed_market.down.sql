-- 000011_seed_market.down.sql

DELETE FROM trading_pairs WHERE pair_id IN ('BTC_USD', 'ETH_USD');
DELETE FROM coins WHERE coin_id IN ('BTC', 'USD', 'ETH');
