import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AccountModal } from './AccountModal.jsx';
import { AccountCardMenu } from './AccountCardMenu.jsx';
import { api } from '../api.js';

export function AccountsPage() {
  const [accounts, setAccounts] = useState([]);
  const [err, setErr] = useState(null);
  const [modalAccount, setModalAccount] = useState(null);
  const [showAdd, setShowAdd] = useState(false);
  const [menuFor, setMenuFor] = useState(null);

  const load = () => {
    api.accounts().then(a => setAccounts(a || [])).catch(e => setErr(e.message));
  };
  useEffect(load, []);

  if (err) return <div class="empty">Error: {err}</div>;

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
      <div class="hero">
        <div class="hero-main">
          <div class="hero-label">Accounts</div>
          <div class="hero-value">{accounts.length}</div>
          <div style={{ marginTop: 10, fontSize: 13, color: 'var(--text-muted)' }}>
            {accounts.filter(a => a.connected).length} connected
          </div>
        </div>
      </div>

      <div class="acc-grid">
        {accounts.map(a => (
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

            <div>
              <div class="stat-label">Currency</div>
              <div class="acc-value">{a.currency}</div>
            </div>

            <div class="acc-footer">
              <span><span class="status-dot" /> {a.connected ? 'Connected' : 'Offline'}</span>
            </div>
          </div>
        ))}

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
