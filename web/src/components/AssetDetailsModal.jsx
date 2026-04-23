import { useEffect, useState } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AssetLogo } from './AssetLogo.jsx';
import { fmtMoney, fmtNum, fmtDate, fmtPct } from '../format.js';
import { api } from '../api.js';

// AssetDetailsModal summarises a non-cash asset: current market price,
// lifetime investment, realised + unrealized PnL, first-activity date
// and the range of buy prices. A "Show activities" action hands the
// symbol back to App so it can route to Activities with this asset
// pinned in the filter. Data is loaded fresh on open — small payloads
// relative to the full asset page, and keeps the modal self-contained.
export function AssetDetailsModal({ asset, onClose, onShowActivities }) {
  const [priceInfo, setPriceInfo] = useState(null);
  const [txs, setTxs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [p, ts] = await Promise.all([
          api.assetPrice(asset.symbol),
          api.transactions('?symbol=' + encodeURIComponent(asset.symbol)),
        ]);
        if (cancelled) return;
        setPriceInfo(p);
        setTxs(ts || []);
      } catch (e) {
        if (!cancelled) setErr(e.message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [asset.symbol]);

  const cur = asset.currency || 'USD';

  // Derive everything in the asset's native currency so the numbers
  // line up with the user's entered tx prices. Average-cost method
  // mirrors portfolio.Holdings on the backend.
  const buys = txs.filter(t => t.side === 'buy');
  const firstTx = txs.length
    ? txs.reduce((a, b) => new Date(a.occurred_at) < new Date(b.occurred_at) ? a : b)
    : null;
  const investmentSum = buys.reduce((s, t) => s + t.qty * t.price + (t.fee || 0), 0);
  const buyPrices = buys.map(t => t.price).filter(p => p > 0);
  const minBuy = buyPrices.length ? Math.min(...buyPrices) : null;
  const maxBuy = buyPrices.length ? Math.max(...buyPrices) : null;

  // Replay chronologically for running qty + cost (used for current
  // value and unrealized PnL) and for realised PnL on sells.
  let realized = 0;
  let openQty = 0;
  let openCost = 0;
  {
    const sorted = [...txs].sort((a, b) => {
      const da = new Date(a.occurred_at).getTime();
      const db = new Date(b.occurred_at).getTime();
      return da !== db ? da - db : (a.id || 0) - (b.id || 0);
    });
    for (const t of sorted) {
      if (t.side === 'buy') {
        openQty += t.qty;
        openCost += t.qty * t.price + (t.fee || 0);
      } else if (t.side === 'sell') {
        const avg = openQty > 0 ? openCost / openQty : 0;
        const proceeds = t.qty * t.price - (t.fee || 0);
        const costRemoved = avg * t.qty;
        realized += proceeds - costRemoved;
        openQty -= t.qty;
        openCost -= costRemoved;
        if (openQty < 1e-9) { openQty = 0; openCost = 0; }
      }
    }
  }

  const priceStale   = !priceInfo || priceInfo.stale;
  const currentPrice = priceInfo?.price || 0;
  const currentQty   = openQty;
  const currentValue = currentQty * currentPrice;
  const unrealized   = currentQty > 0 ? currentValue - openCost : 0;
  const totalPnL     = unrealized + realized;
  const pnlPct       = investmentSum > 0 ? (totalPnL / investmentSum) * 100 : 0;

  const Stat = ({ label, value, sub, color }) => (
    <div style={{ padding: '10px 0' }}>
      <div style={{ fontSize: 11, color: 'var(--text-faint)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
        {label}
      </div>
      <div style={{ fontFamily: 'var(--font-mono)', fontSize: 18, fontWeight: 500, marginTop: 4, color: color || 'var(--text)' }}>
        {value}
      </div>
      {sub && (
        <div style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginTop: 2 }}>
          {sub}
        </div>
      )}
    </div>
  );

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <div class="modal" style={{ width: 560 }}>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', minWidth: 0 }}>
            <AssetLogo asset={asset} size={44} />
            <div style={{ minWidth: 0 }}>
              <h2 class="modal-title">{asset.symbol}</h2>
              <div class="modal-sub" style={{ marginBottom: 0 }}>{asset.name}</div>
            </div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        {err && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 12 }}>{err}</div>}
        {loading && <div class="empty" style={{ padding: 24 }}>Loading…</div>}

        {!loading && !err && (
          <div style={{
            marginTop: 14,
            display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0 20px',
            borderTop: '1px solid var(--border)',
          }}>
            <Stat
              label="Current price"
              value={priceStale ? '—' : fmtMoney(currentPrice, cur)}
              sub={priceStale ? 'price unavailable' : null}
            />
            <Stat
              label="Quantity"
              value={fmtNum(currentQty, 4)}
            />
            <Stat
              label="Current value"
              value={fmtMoney(currentValue, cur)}
            />
            <Stat
              label="Investment sum"
              value={fmtMoney(investmentSum, cur)}
              sub={buys.length ? `across ${buys.length} buy${buys.length === 1 ? '' : 's'}` : 'no buys yet'}
            />
            <Stat
              label="Total PnL"
              value={fmtMoney(totalPnL, cur, { sign: true }) + ' · ' + fmtPct(pnlPct)}
              sub={
                <>
                  Unrealized {fmtMoney(unrealized, cur, { sign: true })}
                  {' · '}
                  Realized {fmtMoney(realized, cur, { sign: true })}
                </>
              }
              color={totalPnL >= 0 ? 'var(--pos)' : 'var(--neg)'}
            />
            <Stat
              label="Activities"
              value={txs.length.toString()}
              sub={firstTx ? `since ${fmtDate(firstTx.occurred_at)}` : '—'}
            />
            <div style={{ gridColumn: '1 / -1' }}>
              <Stat
                label="Buy price range"
                value={
                  minBuy !== null
                    ? `${fmtMoney(minBuy, cur)} — ${fmtMoney(maxBuy, cur)}`
                    : '—'
                }
              />
            </div>
          </div>
        )}

        <div class="modal-actions">
          <button type="button" class="btn" onClick={onClose}>Close</button>
          <button type="button" class="btn primary"
            onClick={() => { onShowActivities(asset.symbol); onClose(); }}>
            Show activities →
          </button>
        </div>
      </div>
    </div>
  );
}
