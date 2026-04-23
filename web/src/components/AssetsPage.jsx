import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AssetLogo } from './AssetLogo.jsx';
import { AssetModal } from './AssetModal.jsx';
import { AssetDetailsModal } from './AssetDetailsModal.jsx';
import { api } from '../api.js';

const TYPE_LABEL = { stock: 'Stock', etf: 'ETF', crypto: 'Crypto', cash: 'Cash' };

export function AssetsPage({ privacy, onOpenActivity }) {
  const [assets, setAssets] = useState([]);
  const [query, setQuery] = useState('');
  const [err, setErr] = useState(null);
  const [modalAsset, setModalAsset] = useState(null);
  const [detailsAsset, setDetailsAsset] = useState(null);
  const [showAdd, setShowAdd] = useState(false);

  const load = () => {
    api.assets().then(a => setAssets(a || [])).catch(e => setErr(e.message));
  };
  useEffect(load, []);

  if (err) return <div class="empty">Error: {err}</div>;

  const q = query.toLowerCase();
  const filtered = assets.filter(a =>
    q === '' ||
    a.symbol.toLowerCase().includes(q) ||
    (a.name || '').toLowerCase().includes(q)
  );

  const handleDelete = async (asset) => {
    if (!confirm(`Delete "${asset.symbol}"? All transactions for this asset will be removed as well.`)) return;
    try {
      await api.deleteAsset(asset.symbol);
      load();
    } catch (e) {
      alert(e.message || 'Failed to delete asset.');
    }
  };

  return (
    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Assets</div>
          <div style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 2 }}>
            {assets.length} tracked · shared across users
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <div class="search-wrap">
            <Icon name="search" />
            <input class="search" placeholder="Filter…"
              value={query} onInput={e => setQuery(e.currentTarget.value)}
              style={{ width: 160 }} />
          </div>
          <button class="btn primary" onClick={() => setShowAdd(true)}>
            <Icon name="plus" /> Add asset
          </button>
        </div>
      </div>

      <table class="table">
        <thead>
          <tr>
            <th>Symbol</th>
            <th>Name</th>
            <th>Type</th>
            <th>Currency</th>
            <th>Provider</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {filtered.length === 0 && (
            <tr><td colspan="6" class="empty">
              {assets.length === 0 ? 'No assets yet — click "Add asset".' : 'No matches.'}
            </td></tr>
          )}
          {filtered.map(a => {
            // Non-cash rows open the details modal on click; cash rows
            // are static (there's no market-price story for a cash
            // balance). Edit/Delete buttons stopPropagation so they
            // don't also trigger the row click.
            const clickable = a.type !== 'cash';
            const openDetails = () => { if (clickable) setDetailsAsset(a); };
            return (
              <tr key={a.symbol}
                onClick={clickable ? openDetails : undefined}
                onKeyDown={clickable ? (e) => {
                  if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openDetails(); }
                } : undefined}
                role={clickable ? 'button' : undefined}
                tabIndex={clickable ? 0 : undefined}
                style={clickable ? { cursor: 'pointer' } : undefined}>
                <td data-primary>
                  <div class="ticker">
                    <AssetLogo asset={a} size={26} />
                    <div class="ticker-meta">
                      <div class="ticker-sym" style={{ fontSize: 13 }}>{a.type === 'cash' ? a.currency : a.symbol}</div>
                    </div>
                  </div>
                </td>
                <td data-label="Name" style={{ fontSize: 13 }}>{a.name}</td>
                <td data-label="Type"><span class={`pill ${a.type}`}>{TYPE_LABEL[a.type] || a.type}</span></td>
                <td class="mono" data-label="Currency" style={{ fontSize: 13 }}>{a.currency}</td>
                <td data-label="Provider" style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                  {a.provider
                    ? <>{a.provider}{a.provider_id ? ` · ${a.provider_id}` : ''}</>
                    : '—'}
                </td>
                <td data-actions style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                  <button class="icon-btn" title="Edit"
                    onClick={(e) => { e.stopPropagation(); setModalAsset(a); }}>
                    <Icon name="edit" />
                  </button>
                  <button class="icon-btn" title="Delete"
                    onClick={(e) => { e.stopPropagation(); handleDelete(a); }}>
                    <Icon name="trash" />
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>

      {showAdd && (
        <AssetModal
          onClose={() => setShowAdd(false)}
          onSaved={() => { setShowAdd(false); load(); }}
        />
      )}
      {modalAsset && (
        <AssetModal
          asset={modalAsset}
          onClose={() => setModalAsset(null)}
          onSaved={() => { setModalAsset(null); load(); }}
        />
      )}
      {detailsAsset && (
        <AssetDetailsModal
          asset={detailsAsset}
          privacy={privacy}
          onClose={() => setDetailsAsset(null)}
          onShowActivities={(sym) => { if (onOpenActivity) onOpenActivity(sym); }}
        />
      )}
    </div>
  );
}
