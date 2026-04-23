import { useState, useEffect } from 'preact/hooks';

// Currency glyphs for cash-type assets. The keys match domain.Currency
// codes; anything missing falls back to the code itself.
const CUR_SYMBOL = {
  USD: '$', EUR: '€', GBP: '£', JPY: '¥',
  CHF: 'Fr', CAD: 'C$', AUD: 'A$',
};

// AssetLogo renders a 32×32 (by default) circular badge for an asset.
// - cash       → currency glyph in a neutral circle
// - logo_url   → <img> fetched from the resolved provider URL
// - otherwise  → the asset's first two letters, for a graceful fallback
// The component swaps to the initials fallback if the image 404s, and
// resets that state whenever the logo URL changes so a re-lookup
// (e.g. after editing the asset) is not stuck on the previous failure.
export function AssetLogo({ asset, size = 32 }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => { setFailed(false); }, [asset?.logo_url]);

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

  if (asset.logo_url && !failed) {
    return (
      <img class="asset-logo" alt={asset.symbol || ''}
        src={asset.logo_url}
        loading="lazy"
        referrerpolicy="no-referrer"
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
