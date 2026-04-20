import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { api } from '../api.js';

export function AccountsPage() {
  const [accounts, setAccounts] = useState([]);
  const [err, setErr] = useState(null);

  useEffect(() => {
    api.accounts().then(a => setAccounts(a || [])).catch(e => setErr(e.message));
  }, []);

  if (err) return <div class="empty">Error: {err}</div>;

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
            <div class="acc-head">
              <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                <div class="acc-badge" style={{ background: a.color || '#c8502a' }}>{a.short || '??'}</div>
                <div>
                  <div class="acc-name">{a.name}</div>
                  <div class="acc-type">{a.type}</div>
                </div>
              </div>
              <button class="icon-btn"><Icon name="more" /></button>
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

        <button class="acc-card" style={{
          borderStyle: 'dashed', display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center', gap: 10,
          color: 'var(--text-muted)', minHeight: 220,
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
    </>
  );
}
