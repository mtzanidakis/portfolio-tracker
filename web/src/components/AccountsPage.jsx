import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AccountModal } from './AccountModal.jsx';
import { AccountCardMenu } from './AccountCardMenu.jsx';
import { fmtMoney } from '../format.js';
import { api } from '../api.js';

export function AccountsPage() {
  const [accounts, setAccounts] = useState([]);
  const [transactions, setTransactions] = useState([]);
  const [err, setErr] = useState(null);
  const [modalAccount, setModalAccount] = useState(null);
  const [showAdd, setShowAdd] = useState(false);
  const [menuFor, setMenuFor] = useState(null);

  const load = async () => {
    try {
      const [accs, txs] = await Promise.all([api.accounts(), api.transactions()]);
      setAccounts(accs || []);
      setTransactions(txs || []);
    } catch (e) {
      setErr(e.message);
    }
  };
  useEffect(() => { load(); }, []);

  if (err) return <div class="empty">Error: {err}</div>;

  // Per-account stats computed in the account's currency at book value.
  // Trades and cash operations live on separate ledgers: the same card
  // can show both when an account hosts a mix (e.g., a brokerage that
  // also holds idle cash).
  //   invested    = buy(qty*price + fee) − sell(qty*price)
  //   cashBalance = deposit + interest − withdraw
  const statsFor = (accountId) => {
    const txs = transactions.filter(t => t.account_id === accountId);
    let invested = 0;
    let cashBalance = 0;
    let hasTrade = false;
    let hasCash = false;
    for (const t of txs) {
      const gross = t.qty * t.price;
      const fee = t.fee || 0;
      switch (t.side) {
        case 'buy':      invested += gross + fee; hasTrade = true; break;
        case 'sell':     invested -= gross;       hasTrade = true; break;
        case 'deposit':
        case 'interest': cashBalance += gross;    hasCash = true;  break;
        case 'withdraw': cashBalance -= gross;    hasCash = true;  break;
      }
    }
    return { count: txs.length, invested, cashBalance, hasTrade, hasCash };
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
          const { count, invested, cashBalance, hasTrade, hasCash } = statsFor(a.id);
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
              {hasTrade && (
                <div>
                  <div class="stat-label">Net invested · {a.currency}</div>
                  <div class="acc-value">{fmtMoney(invested, a.currency)}</div>
                </div>
              )}
              {!hasCash && !hasTrade && (
                <div>
                  <div class="stat-label">Net invested · {a.currency}</div>
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
