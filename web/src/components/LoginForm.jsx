import { useState } from 'preact/hooks';
import { api } from '../api.js';

const MIN_PW = 8;

export function LoginForm({ onLoggedIn }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e) => {
    e.preventDefault();
    if (!email || password.length < MIN_PW) return;
    setError('');
    setSubmitting(true);
    try {
      const user = await api.login(email, password);
      onLoggedIn(user);
    } catch (err) {
      setError('Invalid email or password.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div class="modal-backdrop" style={{ position: 'fixed', inset: 0 }}>
      <form class="modal" onSubmit={submit}>
        <h2 class="modal-title">Portfolio Tracker</h2>
        <div class="modal-sub">Sign in to continue.</div>

        <div class="field">
          <label>Email</label>
          <input class="input" type="email" autoFocus autocomplete="username"
            value={email} onInput={e => setEmail(e.currentTarget.value)} />
        </div>
        <div class="field">
          <label>Password</label>
          <input class="input" type="password" autocomplete="current-password"
            value={password} onInput={e => setPassword(e.currentTarget.value)} />
        </div>

        {error && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 8 }}>{error}</div>}

        <div class="modal-actions">
          <button type="submit" class="btn primary"
            disabled={!email || password.length < MIN_PW || submitting}>
            {submitting ? 'Signing in…' : 'Sign in'}
          </button>
        </div>

        <div style={{ fontSize: 12, color: 'var(--text-faint)', marginTop: 16, textAlign: 'center' }}>
          No account yet? Ask your admin to run <code>ptadmin user add</code>.
        </div>
      </form>
    </div>
  );
}
