import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { fmtMoney, fmtNum, fmtDate } from '../format.js';
import { api } from '../api.js';

export function ActivitiesPage({ privacy, currency, openModal }) {
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [rows, setRows] = useState([]);
  const [assets, setAssets] = useState([]);
  const [accounts, setAccounts] = useState([]);
  const [err, setErr] = useState(null);

  async function load() {
    try {
      const [tx, a, accs] = await Promise.all([api.transactions(), api.assets(), api.accounts()]);
      setRows(tx || []);
      setAssets(a || []);
      setAccounts(accs || []);
    } catch (e) {
      setErr(e.message);
    }
  }
  useEffect(() => { load(); }, []);

  if (err) return <div class="empty">Error: {err}</div>;

  const assetMap = Object.fromEntries((assets || []).map(a => [a.symbol, a]));
  const accMap = Object.fromEntries((accounts || []).map(a => [a.id, a]));
  const q = query.toLowerCase();

  const filtered = rows.filter(tx =>
    (filter === 'all' ? true : tx.side === filter) &&
    (q === '' ||
      tx.asset_symbol.toLowerCase().includes(q) ||
      (assetMap[tx.asset_symbol]?.name || '').toLowerCase().includes(q))
  );

  const totalBuys = rows.filter(a => a.side === 'buy').reduce((s, a) => s + a.qty * a.price, 0);
  const totalSells = rows.filter(a => a.side === 'sell').reduce((s, a) => s + a.qty * a.price, 0);

  return (
    <>
      <div class="hero">
        <div class="hero-main">
          <div class="hero-label">Activities</div>
          <div class="hero-value">
            {rows.length} <span style={{ fontSize: 18, color: 'var(--text-muted)' }}>transactions</span>
          </div>
          <div style={{ marginTop: 10, fontSize: 13, color: 'var(--text-muted)' }}>
            Across {new Set(rows.map(a => a.asset_symbol)).size} assets in {new Set(rows.map(a => a.account_id)).size} accounts
          </div>
        </div>
        <div class="hero-side">
          <div class="stat">
            <div>
              <div class="stat-label">Total invested</div>
              <div class="stat-value pos">
                {privacy ? <span class="masked">{fmtMoney(totalBuys, currency)}</span> : fmtMoney(totalBuys, currency)}
              </div>
            </div>
            <div class="stat-sub">{rows.filter(a => a.side === 'buy').length} buy orders</div>
          </div>
          <div class="stat">
            <div>
              <div class="stat-label">Total realized</div>
              <div class="stat-value neg">
                {privacy ? <span class="masked">{fmtMoney(totalSells, currency)}</span> : fmtMoney(totalSells, currency)}
              </div>
            </div>
            <div class="stat-sub">{rows.filter(a => a.side === 'sell').length} sell orders</div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <div class="card-title">Transaction history</div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <div class="timeframe">
              <button class={filter === 'all' ? 'active' : ''} onClick={() => setFilter('all')}>All</button>
              <button class={filter === 'buy' ? 'active' : ''} onClick={() => setFilter('buy')}>Buys</button>
              <button class={filter === 'sell' ? 'active' : ''} onClick={() => setFilter('sell')}>Sells</button>
            </div>
            <div class="search-wrap">
              <Icon name="search" />
              <input class="search" placeholder="Filter…" value={query}
                onInput={e => setQuery(e.currentTarget.value)} style={{ width: 160 }} />
            </div>
          </div>
        </div>

        <table class="table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Side</th>
              <th>Asset</th>
              <th style={{ textAlign: 'right' }}>Quantity</th>
              <th style={{ textAlign: 'right' }}>Price</th>
              <th style={{ textAlign: 'right' }}>Total</th>
              <th>Account</th>
              <th>Note</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(tx => {
              const asset = assetMap[tx.asset_symbol];
              const acc = accMap[tx.account_id];
              const total = tx.qty * tx.price;
              const assetCur = asset?.currency || 'USD';
              return (
                <tr key={tx.id}>
                  <td class="mono" style={{ color: 'var(--text-muted)', fontSize: 12 }}>
                    {fmtDate(tx.occurred_at, { year: '2-digit', month: 'short', day: '2-digit' })}
                  </td>
                  <td><span class={`pill ${tx.side}`}>{tx.side === 'buy' ? 'Buy' : 'Sell'}</span></td>
                  <td>
                    <div class="ticker">
                      <div class="ticker-icon" style={{ background: '#999', width: 26, height: 26, fontSize: 10 }}>
                        {tx.asset_symbol.slice(0, 2)}
                      </div>
                      <div class="ticker-meta">
                        <div class="ticker-sym" style={{ fontSize: 13 }}>{tx.asset_symbol}</div>
                        <div class="ticker-name">{asset?.name || ''}</div>
                      </div>
                    </div>
                  </td>
                  <td class="num" style={{ textAlign: 'right' }}>{fmtNum(tx.qty, 4)}</td>
                  <td class="num" style={{ textAlign: 'right', color: 'var(--text-muted)' }}>
                    {fmtMoney(tx.price, assetCur)}
                  </td>
                  <td class="num" style={{ textAlign: 'right' }}>
                    {privacy ? <span class="masked">{fmtMoney(total, assetCur)}</span> : fmtMoney(total, assetCur)}
                  </td>
                  <td style={{ fontSize: 12, color: 'var(--text-muted)' }}>{acc?.name}</td>
                  <td style={{ fontSize: 12, color: 'var(--text-faint)' }}>{tx.note || '—'}</td>
                </tr>
              );
            })}
            {filtered.length === 0 && (
              <tr><td colspan="8" class="empty">No transactions match your filter.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
