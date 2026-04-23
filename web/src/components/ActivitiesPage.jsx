import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AssetLogo } from './AssetLogo.jsx';
import { TxModal } from './TxModal.jsx';
import { fmtMoney, fmtNum, fmtDate } from '../format.js';
import { api } from '../api.js';

const SIDE_LABEL = {
  buy: 'Buy', sell: 'Sell',
  deposit: 'Deposit', withdraw: 'Withdraw', interest: 'Interest',
};
const TRADE_SIDES = new Set(['buy', 'sell']);
const CASH_SIDES = new Set(['deposit', 'withdraw', 'interest']);

export function ActivitiesPage({ privacy, currency, user }) {
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [rows, setRows] = useState([]);
  const [assets, setAssets] = useState([]);
  const [accounts, setAccounts] = useState([]);
  const [editTx, setEditTx] = useState(null);
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

  const inFilter = (side) => {
    switch (filter) {
      case 'all':    return true;
      case 'trades': return TRADE_SIDES.has(side);
      case 'cash':   return CASH_SIDES.has(side);
      default:       return side === filter;
    }
  };

  const filtered = rows.filter(tx =>
    inFilter(tx.side) &&
    (q === '' ||
      tx.asset_symbol.toLowerCase().includes(q) ||
      (assetMap[tx.asset_symbol]?.name || '').toLowerCase().includes(q))
  );

  const inBase = (tx) => tx.qty * tx.price * (tx.fx_to_base || 1);
  const totalBuys = rows.filter(a => a.side === 'buy').reduce((s, a) => s + inBase(a), 0);
  const totalSells = rows.filter(a => a.side === 'sell').reduce((s, a) => s + inBase(a), 0);
  const totalDeposits = rows.filter(a => a.side === 'deposit').reduce((s, a) => s + inBase(a), 0);
  const totalWithdraws = rows.filter(a => a.side === 'withdraw').reduce((s, a) => s + inBase(a), 0);
  const totalInterest = rows.filter(a => a.side === 'interest').reduce((s, a) => s + inBase(a), 0);
  const cashFlow = totalDeposits + totalInterest - totalWithdraws;
  const hasCashActivity = totalDeposits + totalWithdraws + totalInterest > 0;

  const handleDelete = async (tx) => {
    if (!confirm(`Delete this ${tx.side} of ${tx.qty} ${tx.asset_symbol}?`)) return;
    try {
      await api.deleteTx(tx.id);
      load();
    } catch (e) {
      alert(e.message || 'Failed to delete transaction.');
    }
  };

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
          {hasCashActivity && (
            <div style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>
              Cash flow:{' '}
              <span class="mono" style={{ color: cashFlow >= 0 ? 'var(--pos)' : 'var(--neg)' }}>
                {fmtMoney(cashFlow, currency, { sign: true })}
              </span>
              {' '}
              <span style={{ color: 'var(--text-faint)' }}>
                ({fmtMoney(totalDeposits, currency)} in · {fmtMoney(totalInterest, currency)} interest · {fmtMoney(totalWithdraws, currency)} out)
              </span>
            </div>
          )}
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
              <button class={filter === 'trades' ? 'active' : ''} onClick={() => setFilter('trades')}>Trades</button>
              <button class={filter === 'cash' ? 'active' : ''} onClick={() => setFilter('cash')}>Cash</button>
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
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(tx => {
              const asset = assetMap[tx.asset_symbol];
              const acc = accMap[tx.account_id];
              const isCashTx = CASH_SIDES.has(tx.side);
              const total = tx.qty * tx.price;
              const accCur = acc?.currency || asset?.currency || 'USD';
              const rowCur = isCashTx ? (asset?.currency || accCur) : accCur;
              return (
                <tr key={tx.id}>
                  <td class="mono" style={{ color: 'var(--text-muted)', fontSize: 12 }}>
                    {fmtDate(tx.occurred_at, { year: '2-digit', month: 'short', day: '2-digit' })}
                  </td>
                  <td><span class={`pill ${tx.side}`}>{SIDE_LABEL[tx.side] || tx.side}</span></td>
                  <td>
                    <div class="ticker">
                      <AssetLogo asset={asset || { symbol: tx.asset_symbol }} size={26} />
                      <div class="ticker-meta">
                        <div class="ticker-sym" style={{ fontSize: 13 }}>
                          {asset?.type === 'cash' ? asset.currency : tx.asset_symbol}
                        </div>
                        <div class="ticker-name">{asset?.name || ''}</div>
                      </div>
                    </div>
                  </td>
                  <td class="num" style={{ textAlign: 'right' }}>
                    {isCashTx ? fmtMoney(tx.qty, rowCur) : fmtNum(tx.qty, 4)}
                  </td>
                  <td class="num" style={{ textAlign: 'right', color: 'var(--text-muted)' }}>
                    {isCashTx ? '—' : fmtMoney(tx.price, accCur)}
                  </td>
                  <td class="num" style={{ textAlign: 'right' }}>
                    {privacy ? <span class="masked">{fmtMoney(total, rowCur)}</span> : fmtMoney(total, rowCur)}
                  </td>
                  <td style={{ fontSize: 12, color: 'var(--text-muted)' }}>{acc?.name}</td>
                  <td style={{ fontSize: 12, color: 'var(--text-faint)' }}>{tx.note || '—'}</td>
                  <td style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                    <button class="icon-btn" title="Edit" onClick={() => setEditTx(tx)}>
                      <Icon name="edit" />
                    </button>
                    <button class="icon-btn" title="Delete" onClick={() => handleDelete(tx)}>
                      <Icon name="trash" />
                    </button>
                  </td>
                </tr>
              );
            })}
            {filtered.length === 0 && (
              <tr><td colspan="9" class="empty">No transactions match your filter.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {editTx && (
        <TxModal
          transaction={editTx}
          user={user}
          onClose={() => setEditTx(null)}
          onSaved={() => { setEditTx(null); load(); }}
        />
      )}
    </>
  );
}
