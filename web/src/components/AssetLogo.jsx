import { useState, useEffect } from 'preact/hooks';

// Currency glyphs for cash-type assets. The keys match domain.Currency
// codes; anything missing falls back to the code itself.
const CUR_SYMBOL = {
  USD: '$', EUR: '€', GBP: '£', JPY: '¥',
  CHF: 'Fr', CAD: 'C$', AUD: 'A$',
};

// AssetLogo renders a 32×32 (by default) circular badge for an asset.
// - cash       → currency glyph in a neutral circle
// - logo_url   → <img> served by our /api/v1/assets/{sym}/logo proxy,
//                which fetches the provider URL once and caches the
//                bytes in sqlite — the browser never talks to Clearbit
//                or CoinGecko directly.
// - otherwise  → the asset's first two letters, for a graceful fallback
// The component swaps to the initials fallback if the image 404s, and
// resets that state whenever the asset symbol or URL changes so a
// re-lookup (e.g. after editing the asset) is not stuck on the previous
// failure.
export function AssetLogo({ asset, size = 32, previewURL }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => { setFailed(false); }, [asset?.symbol, asset?.logo_url, previewURL]);

  const base = {
    width: size, height: size,
    borderRadius: '50%',
    display: 'grid', placeItems: 'center',
    overflow: 'hidden',
    flexShrink: 0,
  };

  if (!asset) {
    return <div class="asset-logo" style={{ ...base, background: 'var(--bg-sunken)' }} />;
  }

  if (asset.type === 'cash') {
    const glyph = CUR_SYMBOL[asset.currency] || asset.currency || '¤';
    return (
      <div class="asset-logo cash" style={{
        ...base,
        background: 'var(--bg-sunken)',
        color: 'var(--text)',
        fontWeight: 600,
        fontSize: Math.round(size * 0.44),
        fontFamily: 'var(--font-mono, inherit)',
      }}>{glyph}</div>
    );
  }

  // In the Add-asset preview the row isn't in the DB yet, so the proxy
  // would 404. The caller passes `previewURL` to have us hit the raw
  // provider URL directly just for that transient preview.
  const src = previewURL
    || (asset.logo_url && asset.symbol
        ? `/api/v1/assets/${encodeURIComponent(asset.symbol)}/logo`
        : '');
  if (src && !failed) {
    // Strip Referer on the transient preview so Clearbit/CoinGecko
    // don't see the page URL; the proxy path is same-origin and safe.
    const referrerPolicy = previewURL ? 'no-referrer' : undefined;
    return (
      <img class="asset-logo" alt={asset.symbol || ''}
        src={src}
        loading="lazy"
        referrerpolicy={referrerPolicy}
        onError={() => setFailed(true)}
        style={{ ...base, background: 'var(--bg-sunken)', objectFit: 'cover' }} />
    );
  }

  const initials = (asset.symbol || '').replace(/^CASH-/, '').slice(0, 2).toUpperCase();
  return (
    <div class="asset-logo fallback" style={{
      ...base,
      background: 'var(--bg-sunken)',
      color: 'var(--text-muted)',
      fontWeight: 600,
      fontSize: Math.max(10, Math.round(size * 0.36)),
    }}>{initials}</div>
  );
}
