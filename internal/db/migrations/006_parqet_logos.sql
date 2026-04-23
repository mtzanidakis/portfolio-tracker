-- Re-point existing stock/ETF logos at Parqet (the Yahoo-based
-- Clearbit lookup used previously is defunct). Crypto rows stay as-is
-- because CoinGecko serves its own logos.
UPDATE assets
   SET logo_url = 'https://assets.parqet.com/logos/symbol/' || symbol
 WHERE type IN ('stock','etf')
   AND provider = 'yahoo';

-- Drop any cached blobs for those rows so the proxy re-fetches from
-- the new upstream on next render.
DELETE FROM asset_logos
 WHERE symbol IN (
       SELECT symbol FROM assets WHERE type IN ('stock','etf') AND provider = 'yahoo'
);
