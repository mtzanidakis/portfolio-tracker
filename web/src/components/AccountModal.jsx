import { useState } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { api } from '../api.js';

const CURRENCIES = ['USD', 'EUR', 'GBP', 'JPY', 'CHF', 'CAD', 'AUD'];
const TYPES = [
  'Taxable Brokerage',
  'Retirement',
  'Crypto Exchange',
  'Self-custody',
  'Cash / Savings',
  'Other',
];

// deterministic default colour palette — mirrors the mock's earthy tones.
const COLOURS = ['#c8502a', '#d4953d', '#a8572e', '#7a8c6f', '#b8632e', '#c9a87c'];

export function AccountModal({ onClose, onSaved }) {
  const [name, setName] = useState('');
  const [type, setType] = useState(TYPES[0]);
  const [short, setShort] = useState('');
  const [color, setColor] = useState(COLOURS[0]);
  const [currency, setCurrency] = useState('USD');
  const [connected, setConnected] = useState(true);
  const [err, setErr] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const autoShort = name.trim().split(/\s+/).map(w => w[0]).join('').slice(0, 2).toUpperCase();

  const submit = async (e) => {
    e.preventDefault();
    setErr('');
    if (!name.trim()) {
      setErr('Name is required.');
      return;
    }
    setSubmitting(true);
    try {
      const created = await api.createAccount({
        name: name.trim(),
        type,
        short: (short || autoShort || '??').slice(0, 3).toUpperCase(),
        color,
        currency,
        connected,
      });
      onSaved(created);
      onClose();
    } catch (e) {
      setErr(e.message || 'Failed to create account.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <form class="modal" onSubmit={submit}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h2 class="modal-title">Add account</h2>
            <div class="modal-sub">Accounts are labels — brokerage, exchange, wallet, cash.</div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        <div class="field">
          <label>Name</label>
          <input class="input" autoFocus
            placeholder="e.g. Sunset Exchange"
            value={name} onInput={e => setName(e.currentTarget.value)} />
        </div>

        <div class="row-2">
          <div class="field">
            <label>Type</label>
            <select class="select" value={type} onChange={e => setType(e.currentTarget.value)}>
              {TYPES.map(t => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div class="field">
            <label>Currency</label>
            <select class="select" value={currency} onChange={e => setCurrency(e.currentTarget.value)}>
              {CURRENCIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Short label (1–3 chars)</label>
            <input class="input mono" maxlength={3}
              placeholder={autoShort || 'EB'}
              value={short} onInput={e => setShort(e.currentTarget.value.toUpperCase())} />
          </div>
          <div class="field">
            <label>Color</label>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center', height: 38 }}>
              {COLOURS.map(c => (
                <button key={c} type="button"
                  onClick={() => setColor(c)}
                  aria-label={`color ${c}`}
                  style={{
                    width: 26, height: 26, borderRadius: '50%',
                    background: c, border: color === c ? '2px solid var(--text)' : '1px solid var(--border)',
                    cursor: 'pointer', padding: 0,
                  }} />
              ))}
            </div>
          </div>
        </div>

        <div class="tweak-row" style={{ marginTop: 4 }}>
          <span>Connected</span>
          <button type="button" class={`switch ${connected ? 'on' : ''}`}
            onClick={() => setConnected(c => !c)} />
        </div>

        {err && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 8 }}>{err}</div>}

        <div class="modal-actions">
          <button type="button" class="btn" onClick={onClose}>Cancel</button>
          <button type="submit" class="btn primary" disabled={!name.trim() || submitting}>
            <Icon name="check" /> {submitting ? 'Saving…' : 'Create account'}
          </button>
        </div>
      </form>
    </div>
  );
}
