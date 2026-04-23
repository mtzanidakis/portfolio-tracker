import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AccountModal } from './AccountModal.jsx';
import { AccountCardMenu } from './AccountCardMenu.jsx';
import { fmtMoney } from '../format.js';
import { api } from '../api.js';

export function AccountsPage() {
  const [accounts, setAccounts] = useState([]);
  const [transactions, setTransactions] = useState([]);
  // `holdings` is the cross-account aggregate from /api/v1/holdings,
  // keyed by symbol. We use each holding's per-unit native price
  // (ValueNative / Qty) to price the per-account open position.
  const [holdings, setHoldings] = useState([]);
  const [err, setErr] = useState(null);
  const [modalAccount, setModalAccount] = useState(null);
  const [showAdd, setShowAdd] = useState(false);
  const [menuFor, setMenuFor] = useState(null);

  const load = async () => {
    try {
      const [accs, txs, h] = await Promise.all([
        api.accounts(), api.transactions(), api.holdings(),
      ]);
      setAccounts(accs || []);
      setTransactions(txs || []);
      setHoldings(h || []);
    } catch (e) {
      setErr(e.message);
    }
  };
  useEffect(() => { load(); }, []);

  // Per-unit native-currency price for each symbol — derived from the
  // aggregate holdings so we don't need a second prices endpoint.
  // `PriceStale` propagates so the card can signal a '?' when the
  // refresher hasn't populated prices yet.
  const priceMap = Object.fromEntries((holdings || []).map(h => [
    h.Symbol,
    {
      price: h.Qty > 0 ? (h.ValueNative / h.Qty) : 0,
      stale: !!h.PriceStale,
    },
  ]));

  if (err) return <div class="empty">Error: {err}</div>;

  // Per-account stats computed in the account's currency at book value.
  // Trades and cash operations live on separate ledgers:
  //   openCost    = running cost basis of still-open positions
  //                 (same average-cost method as portfolio.Holdings)
  //   realized    = lifetime realised PnL across closed trades in this
  //                 account — the missing piece that made a fully-sold
  //                 loser look like "net invested $11.84"
  //   cashBalance = deposit + interest − withdraw
  // Transactions are replayed chronologically per symbol.
  const statsFor = (accountId) => {
    const txs = transactions.filter(t => t.account_id === accountId);
    const sorted = [...txs].sort((a, b) => {
      const da = new Date(a.occurred_at).getTime();
      const db = new Date(b.occurred_at).getTime();
      if (da !== db) return da - db;
      return (a.id || 0) - (b.id || 0);
    });
    const state = new Map();
    let realized = 0;
    let cashBalance = 0;
    let hasTrade = false;
    let hasCash = false;
    for (const t of sorted) {
      const gross = t.qty * t.price;
      const fee = t.fee || 0;
      switch (t.side) {
        case 'buy': {
          hasTrade = true;
          const cur = state.get(t.asset_symbol) || { qty: 0, cost: 0 };
          cur.qty += t.qty;
          cur.cost += gross + fee;
          state.set(t.asset_symbol, cur);
          break;
        }
        case 'sell': {
          hasTrade = true;
          const cur = state.get(t.asset_symbol) || { qty: 0, cost: 0 };
          const avg = cur.qty > 0 ? cur.cost / cur.qty : 0;
          const proceeds = gross - fee;
          const costRemoved = avg * t.qty;
          realized += proceeds - costRemoved;
          cur.qty -= t.qty;
          cur.cost -= costRemoved;
          if (cur.qty < 1e-9) { cur.qty = 0; cur.cost = 0; }
          state.set(t.asset_symbol, cur);
          break;
        }
        case 'deposit':
        case 'interest': cashBalance += gross; hasCash = true; break;
        case 'withdraw': cashBalance -= gross; hasCash = true; break;
      }
    }
    let openCost = 0;
    let openValue = 0;
    let valueStale = false;
    let hasOpen = false;
    for (const [sym, { qty, cost }] of state.entries()) {
      openCost += cost;
      if (qty > 0) {
        hasOpen = true;
        const p = priceMap[sym];
        if (!p || p.stale || !p.price) {
          valueStale = true;
          openValue += cost; // fall back to cost so the card still totals
        } else {
          openValue += qty * p.price;
        }
      }
    }
    const unrealized = hasOpen ? openValue - openCost : 0;
    return {
      count: txs.length,
      openCost, openValue, unrealized, valueStale, hasOpen,
      realized, cashBalance, hasTrade, hasCash,
    };
  };

  const handleDelete = async (acc) => {
    if (!confirm(`Delete "${acc.name}"? Accounts referenced by transactions cannot be deleted.`)) return;
    try {
      await api.deleteAccount(acc.id);
      load();
    } catch (e) {
      alert(e.message || 'Failed to delete account.');
    }
  };

  return (
    <>
      <div class="acc-grid">
        {accounts.map(a => {
          const {
            count, openCost, openValue, unrealized, valueStale, hasOpen,
            realized, cashBalance, hasTrade, hasCash,
          } = statsFor(a.id);
          return (
            <div key={a.id} class="acc-card">
              <div class="acc-head" style={{ position: 'relative' }}>
                <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                  <div class="acc-badge" style={{ background: a.color || '#c8502a' }}>{a.short || '??'}</div>
                  <div>
                    <div class="acc-name">{a.name}</div>
                    <div class="acc-type">{a.type}</div>
                  </div>
                </div>
                <button
                  type="button" class="icon-btn"
                  onClick={(e) => { e.stopPropagation(); setMenuFor(menuFor === a.id ? null : a.id); }}
                  aria-haspopup="menu" aria-expanded={menuFor === a.id}>
                  <Icon name="more" />
                </button>
                {menuFor === a.id && (
                  <AccountCardMenu
                    onEdit={() => setModalAccount(a)}
                    onDelete={() => handleDelete(a)}
                    onClose={() => setMenuFor(null)}
                  />
                )}
              </div>

              {hasCash && (
                <div>
                  <div class="stat-label">Cash balance · {a.currency}</div>
                  <div class="acc-value">{fmtMoney(cashBalance, a.currency)}</div>
                </div>
              )}
              {hasTrade && hasOpen && (() => {
                const unrealPct = openCost > 0 ? (unrealized / openCost) * 100 : 0;
                const unrealColor = unrealized >= 0 ? 'var(--pos)' : 'var(--neg)';
                return (
                  <div>
                    <div class="stat-label">
                      Current value · {a.currency}
                      {valueStale && (
                        <span title="Price data missing; falling back to cost"
                          style={{ marginLeft: 6, color: 'var(--neg)' }}>⚠</span>
                      )}
                    </div>
                    <div class="acc-value">{fmtMoney(openValue, a.currency)}</div>
                    <div style={{
                      fontSize: 12, fontFamily: 'var(--font-mono)', marginTop: 4,
                      color: 'var(--text-muted)',
                    }}>
                      Cost {fmtMoney(openCost, a.currency)}
                      {' · '}
                      <span style={{ color: unrealColor }}>
                        {fmtMoney(unrealized, a.currency, { sign: true })} ({unrealPct.toFixed(2)}%)
                      </span>
                    </div>
                    {realized !== 0 && (
                      <div style={{
                        fontSize: 12, fontFamily: 'var(--font-mono)', marginTop: 2,
                        color: realized >= 0 ? 'var(--pos)' : 'var(--neg)',
                      }}>
                        Realized {fmtMoney(realized, a.currency, { sign: true })}
                      </div>
                    )}
                  </div>
                );
              })()}
              {hasTrade && !hasOpen && (
                <div>
                  <div class="stat-label">Cost basis · {a.currency}</div>
                  <div class="acc-value">{fmtMoney(0, a.currency)}</div>
                  {realized !== 0 && (
                    <div style={{
                      fontSize: 12, fontFamily: 'var(--font-mono)', marginTop: 4,
                      color: realized >= 0 ? 'var(--pos)' : 'var(--neg)',
                    }}>
                      Realized {fmtMoney(realized, a.currency, { sign: true })}
                    </div>
                  )}
                </div>
              )}
              {!hasCash && !hasTrade && (
                <div>
                  <div class="stat-label">Cost basis · {a.currency}</div>
                  <div class="acc-value">{fmtMoney(0, a.currency)}</div>
                </div>
              )}

              <div class="acc-footer">
                <span>{count} transaction{count === 1 ? '' : 's'}</span>
              </div>
            </div>
          );
        })}

        <button
          type="button"
          onClick={() => setShowAdd(true)}
          class="acc-card"
          style={{
            borderStyle: 'dashed', display: 'flex', flexDirection: 'column',
            alignItems: 'center', justifyContent: 'center', gap: 10,
            color: 'var(--text-muted)', minHeight: 220, cursor: 'pointer',
            background: 'transparent',
          }}>
          <div style={{
            width: 40, height: 40, borderRadius: 10,
            background: 'var(--bg-sunken)', display: 'grid', placeItems: 'center',
            color: 'var(--terra)',
          }}>
            <Icon name="plus" size={20} />
          </div>
          <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--text)' }}>Add an account</div>
          <div style={{ fontSize: 12, textAlign: 'center', maxWidth: 220 }}>
            Labels only — brokerage, exchange, wallet, cash.
          </div>
        </button>
      </div>

      {showAdd && (
        <AccountModal
          onClose={() => setShowAdd(false)}
          onSaved={() => { setShowAdd(false); load(); }}
        />
      )}
      {modalAccount && (
        <AccountModal
          account={modalAccount}
          onClose={() => setModalAccount(null)}
          onSaved={() => { setModalAccount(null); load(); }}
        />
      )}
    </>
  );
}
