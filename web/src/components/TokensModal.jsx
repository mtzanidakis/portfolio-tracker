import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { fmtDate } from '../format.js';
import { api } from '../api.js';

// Expiry presets, in days. 0 means "never expires" — the dropdown's
// default since most users (cron jobs, ptagent on a personal box) want a
// stable token. The shorter values exist for ad-hoc use (a CI bot for a
// release window, a teammate borrowing access for the day, …).
const EXPIRY_OPTIONS = [
  { label: 'Never',     days: 0 },
  { label: '7 days',    days: 7 },
  { label: '30 days',   days: 30 },
  { label: '90 days',   days: 90 },
  { label: '1 year',    days: 365 },
];

export function TokensModal({ onClose }) {
  const [tokens, setTokens] = useState([]);
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(true);

  // Creation flow state.
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [expiryDays, setExpiryDays] = useState(0);
  const [justCreated, setJustCreated] = useState(null); // {name, token}
  const [copied, setCopied] = useState(false);

  const load = async () => {
    setErr('');
    setLoading(true);
    try {
      setTokens(await api.listTokens() || []);
    } catch (e) {
      setErr(e.message || 'Failed to load tokens.');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const create = async (e) => {
    e.preventDefault();
    if (!newName) return;
    setErr('');
    setCreating(true);
    try {
      const expiresAt = expiryDays > 0
        ? new Date(Date.now() + expiryDays * 86400_000).toISOString()
        : null;
      const resp = await api.createToken(newName, expiresAt);
      setJustCreated({ name: resp.name, token: resp.token });
      setNewName('');
      setExpiryDays(0);
      await load();
    } catch (e) {
      setErr(e.message || 'Failed to create token.');
    } finally {
      setCreating(false);
    }
  };

  const revoke = async (id) => {
    if (!confirm('Revoke this token? This cannot be undone.')) return;
    try {
      await api.revokeToken(id);
      await load();
    } catch (e) {
      setErr(e.message || 'Failed to revoke token.');
    }
  };

  const remove = async (id) => {
    if (!confirm('Delete this token from the list? The row is hidden but kept for audit.')) return;
    try {
      await api.deleteToken(id);
      await load();
    } catch (e) {
      setErr(e.message || 'Failed to delete token.');
    }
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(justCreated.token);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { /* ignore */ }
  };

  const fmtMaybe = (s) => s ? fmtDate(s) : '—';

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <div class="modal" style={{ maxWidth: 640 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h2 class="modal-title">API tokens</h2>
            <div class="modal-sub">
              Tokens authenticate <code>ptagent</code> and other automation.
              Each token is shown once — copy it immediately.
            </div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        {justCreated && (
          <div style={{
            background: 'var(--terra-wash)',
            border: '1px solid var(--terra)',
            borderRadius: 'var(--radius-sm)',
            padding: 12, marginBottom: 14,
          }}>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 6 }}>
              New token "<strong>{justCreated.name}</strong>" — save this now:
            </div>
            <div class="mono" style={{
              fontSize: 12, wordBreak: 'break-all',
              background: 'var(--bg-sunken)', padding: '8px 10px',
              borderRadius: 4, userSelect: 'all',
            }}>{justCreated.token}</div>
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              <button class="btn" onClick={copy}>
                {copied ? 'Copied!' : 'Copy to clipboard'}
              </button>
              <button class="btn" onClick={() => setJustCreated(null)}>Done</button>
            </div>
          </div>
        )}

        <form onSubmit={create} style={{ display: 'flex', gap: 8, marginBottom: 14 }}>
          <input
            class="input"
            placeholder="Token name (e.g. 'laptop-cli')"
            value={newName} onInput={e => setNewName(e.currentTarget.value)}
            style={{ flex: 1 }}
          />
          <select class="input" aria-label="Token expiry"
            value={expiryDays}
            onChange={e => setExpiryDays(parseInt(e.currentTarget.value, 10) || 0)}
            style={{ width: 120 }}>
            {EXPIRY_OPTIONS.map(o => (
              <option key={o.days} value={o.days}>{o.label}</option>
            ))}
          </select>
          <button type="submit" class="btn primary" disabled={!newName || creating}>
            {creating ? 'Creating…' : 'Create token'}
          </button>
        </form>

        {err && <div style={{ color: 'var(--neg)', fontSize: 13, marginBottom: 10 }}>{err}</div>}

        <div style={{ border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)' }}>
          <table class="table" style={{ margin: 0 }}>
            <thead>
              <tr>
                <th>Name</th>
                <th>Created</th>
                <th>Last used</th>
                <th>Expires</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {loading && <tr><td colspan="6" class="empty">Loading…</td></tr>}
              {!loading && tokens.length === 0 && (
                <tr><td colspan="6" class="empty">No tokens yet.</td></tr>
              )}
              {tokens.map(t => {
                const expired = t.expires_at && new Date(t.expires_at) <= new Date();
                const status = t.revoked_at
                  ? { label: 'Revoked', color: 'var(--neg)' }
                  : expired
                    ? { label: 'Expired', color: 'var(--neg)' }
                    : { label: 'Active', color: 'var(--pos)' };
                const dimmed = t.revoked_at || expired;
                const canRevoke = !t.revoked_at && !expired;
                return (
                  <tr key={t.id} style={{ opacity: dimmed ? 0.5 : 1 }}>
                    <td>{t.name}</td>
                    <td style={{ fontSize: 12, color: 'var(--text-muted)' }}>{fmtMaybe(t.created_at)}</td>
                    <td style={{ fontSize: 12, color: 'var(--text-muted)' }}>{fmtMaybe(t.last_used_at)}</td>
                    <td style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                      {t.expires_at ? fmtDate(t.expires_at) : 'Never'}
                    </td>
                    <td>
                      <span style={{ color: status.color, fontSize: 12 }}>{status.label}</span>
                    </td>
                    <td style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                      {canRevoke && (
                        <button class="btn" onClick={() => revoke(t.id)}
                          style={{ fontSize: 12, padding: '4px 10px', marginRight: 6 }}>Revoke</button>
                      )}
                      <button class="icon-btn" title="Delete" aria-label="Delete"
                        onClick={() => remove(t.id)}>
                        <Icon name="trash" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
