import { useState, useEffect, useRef } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AssetLogo } from './AssetLogo.jsx';
import { TxModal } from './TxModal.jsx';
import { fmtMoney, fmtNum, fmtDate } from '../format.js';
import { api } from '../api.js';

const SIDE_LABEL = {
  buy: 'Buy', sell: 'Sell',
  deposit: 'Deposit', withdraw: 'Withdraw', interest: 'Interest',
};
const CASH_SIDES = new Set(['deposit', 'withdraw', 'interest']);

// Maps the UI tab to the `side=` query param the backend expects.
// Empty string → no side filter (show everything).
const FILTER_TO_SIDES = {
  all:    '',
  trades: 'buy,sell',
  cash:   'deposit,withdraw,interest',
};

const PAGE_SIZE = 50;
const Q_DEBOUNCE_MS = 300;

export function ActivitiesPage({ privacy, currency, user }) {
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  // Items are the paginated slice from the server. Aggregates for the
  // hero come from /api/v1/transactions/summary so we never load the
  // whole history just to sum it.
  const [items, setItems] = useState([]);
  const [cursor, setCursor] = useState('');
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [summary, setSummary] = useState(null);
  const [assets, setAssets] = useState([]);
  const [accounts, setAccounts] = useState([]);
  const [editTx, setEditTx] = useState(null);
  const [err, setErr] = useState(null);

  // Debounce the free-text input so each keystroke doesn't hammer the
  // backend. The debounced value drives the actual fetch.
  const [debouncedQ, setDebouncedQ] = useState('');
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQ(query.trim()), Q_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [query]);

  // Assets + accounts are lookup maps that don't change across pages;
  // the hero stats re-pull on every commit via reloadStats (see below).
  const loadLookups = async () => {
    const [a, accs] = await Promise.all([api.assets(), api.accounts()]);
    setAssets(a || []);
    setAccounts(accs || []);
  };

  const reloadSummary = async () => {
    try {
      setSummary(await api.txSummary());
    } catch (e) {
      setErr(e.message);
    }
  };

  // A request-sequence ref guards against stale responses: filter /
  // query changes fire overlapping fetches and we only want the
  // latest one to land.
  const seq = useRef(0);

  const fetchPage = async ({ cursor = '', reset = false } = {}) => {
    const mySeq = ++seq.current;
    if (reset) setLoading(true); else setLoadingMore(true);
    try {
      const { items: page, nextCursor } = await api.transactionsPage({
        q: debouncedQ,
        side: FILTER_TO_SIDES[filter] || '',
        cursor,
        limit: PAGE_SIZE,
      });
      if (seq.current !== mySeq) return;
      setItems(prev => reset ? (page || []) : [...prev, ...(page || [])]);
      setCursor(nextCursor);
      setErr(null);
    } catch (e) {
      if (seq.current === mySeq) setErr(e.message);
    } finally {
      if (seq.current === mySeq) {
        setLoading(false);
        setLoadingMore(false);
      }
    }
  };

  // Initial load: lookup tables + first page + aggregate stats.
  useEffect(() => {
    loadLookups();
    reloadSummary();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Any time the filter or debounced query changes, reset to page 1.
  useEffect(() => {
    fetchPage({ reset: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter, debouncedQ]);

  if (err) return <div class="empty">Error: {err}</div>;

  const assetMap = Object.fromEntries((assets || []).map(a => [a.symbol, a]));
  const accMap = Object.fromEntries((accounts || []).map(a => [a.id, a]));

  // Aggregates come from the dedicated summary endpoint so the hero
  // never depends on having every transaction loaded client-side.
  const totalBuys      = summary?.total_buys      ?? 0;
  const totalSells     = summary?.total_sells     ?? 0;
  const totalDeposits  = summary?.total_deposits  ?? 0;
  const totalWithdraws = summary?.total_withdraws ?? 0;
  const totalInterest  = summary?.total_interest  ?? 0;
  const cashFlow = totalDeposits + totalInterest - totalWithdraws;
  const hasCashActivity = totalDeposits + totalWithdraws + totalInterest > 0;
  const buyCount  = summary?.buy_count  ?? 0;
  const sellCount = summary?.sell_count ?? 0;
  const txCount   = summary?.count         ?? 0;
  const assetCnt  = summary?.asset_count   ?? 0;
  const acctCnt   = summary?.account_count ?? 0;

  const handleDelete = async (tx) => {
    if (!confirm(`Delete this ${tx.side} of ${tx.qty} ${tx.asset_symbol}?`)) return;
    try {
      await api.deleteTx(tx.id);
      await Promise.all([fetchPage({ reset: true }), reloadSummary()]);
    } catch (e) {
      alert(e.message || 'Failed to delete transaction.');
    }
  };

  const onSavedTx = async () => {
    setEditTx(null);
    await Promise.all([fetchPage({ reset: true }), reloadSummary()]);
  };

  return (
    <>
      <div class="hero">
        <div class="hero-main">
          <div class="hero-label">Activities</div>
          <div class="hero-value">
            {txCount} <span style={{ fontSize: 18, color: 'var(--text-muted)' }}>transactions</span>
          </div>
          <div style={{ marginTop: 10, fontSize: 13, color: 'var(--text-muted)' }}>
            Across {assetCnt} assets in {acctCnt} accounts
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
            <div class="stat-sub">{buyCount} buy orders</div>
          </div>
          <div class="stat">
            <div>
              <div class="stat-label">Total realized</div>
              <div class="stat-value neg">
                {privacy ? <span class="masked">{fmtMoney(totalSells, currency)}</span> : fmtMoney(totalSells, currency)}
              </div>
            </div>
            <div class="stat-sub">{sellCount} sell orders</div>
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
              <th style={{ textAlign: 'right' }}>Fee</th>
              <th style={{ textAlign: 'right' }}>Total</th>
              <th>Account</th>
              <th>Note</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {items.map(tx => {
              const asset = assetMap[tx.asset_symbol];
              const acc = accMap[tx.account_id];
              const isCashTx = CASH_SIDES.has(tx.side);
              const total = tx.qty * tx.price;
              const accCur = acc?.currency || asset?.currency || 'USD';
              const rowCur = isCashTx ? (asset?.currency || accCur) : accCur;
              return (
                <tr key={tx.id}>
                  <td class="mono" data-label="Date" style={{ color: 'var(--text-muted)', fontSize: 12 }}>
                    {fmtDate(tx.occurred_at)}
                  </td>
                  <td data-label="Side"><span class={`pill ${tx.side}`}>{SIDE_LABEL[tx.side] || tx.side}</span></td>
                  <td data-primary>
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
                  <td class="num" data-label="Quantity" style={{ textAlign: 'right' }}>
                    {isCashTx ? fmtMoney(tx.qty, rowCur) : fmtNum(tx.qty, 4)}
                  </td>
                  <td class="num" data-label="Price" style={{ textAlign: 'right', color: 'var(--text-muted)' }}>
                    {isCashTx ? '—' : fmtMoney(tx.price, accCur)}
                  </td>
                  <td class="num" data-label="Fee" style={{ textAlign: 'right', color: 'var(--text-muted)' }}>
                    {tx.fee ? fmtMoney(tx.fee, rowCur) : '—'}
                  </td>
                  <td class="num" data-label="Total" style={{ textAlign: 'right' }}>
                    {privacy ? <span class="masked">{fmtMoney(total, rowCur)}</span> : fmtMoney(total, rowCur)}
                  </td>
                  <td data-label="Account" style={{ fontSize: 12, color: 'var(--text-muted)' }}>{acc?.name}</td>
                  <td data-label="Note" style={{ fontSize: 12, color: 'var(--text-faint)' }}>{tx.note || '—'}</td>
                  <td data-actions style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
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
            {!loading && items.length === 0 && (
              <tr><td colspan="10" class="empty">No transactions match your filter.</td></tr>
            )}
            {loading && items.length === 0 && (
              <tr><td colspan="10" class="empty">Loading…</td></tr>
            )}
          </tbody>
        </table>

        {cursor && !loading && (
          <div style={{ display: 'flex', justifyContent: 'center', marginTop: 16 }}>
            <button class="btn" onClick={() => fetchPage({ cursor })} disabled={loadingMore}>
              {loadingMore ? 'Loading…' : 'Load more'}
            </button>
          </div>
        )}
      </div>

      {editTx && (
        <TxModal
          transaction={editTx}
          user={user}
          onClose={() => setEditTx(null)}
          onSaved={onSavedTx}
        />
      )}
    </>
  );
}
