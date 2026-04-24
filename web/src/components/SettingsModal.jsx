import { useState } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { previewDateFormat } from '../format.js';
import { api } from '../api.js';

const CURRENCIES = ['USD', 'EUR', 'GBP', 'JPY', 'CHF', 'CAD', 'AUD'];

const AESTHETICS = [
  { id: 'technical', label: 'Technical', sub: 'Slate + electric blue' },
  { id: 'editorial', label: 'Editorial', sub: 'Neutral paper + red' },
  { id: 'forest',    label: 'Forest',    sub: 'Cool green + slate' },
];

// Preset list shown in the Date format dropdown. 'browser' stays the
// default for users who never open this modal — it keeps the old
// locale-aware behaviour. 'custom' opens a free-text box with tokens.
const DATE_PRESETS = [
  { id: 'browser',    label: 'Browser locale (default)' },
  { id: 'DD/MM/YYYY', label: 'DD/MM/YYYY' },
  { id: 'MM/DD/YYYY', label: 'MM/DD/YYYY' },
  { id: 'YYYY-MM-DD', label: 'YYYY-MM-DD' },
  { id: 'D MMM YYYY', label: 'D MMM YYYY' },
  { id: 'custom',     label: 'Custom…' },
];

// Any saved pattern that isn't in the preset list means the user picked
// Custom at some point — open the modal straight into that mode.
function presetIdFor(pattern) {
  if (!pattern) return 'browser';
  if (DATE_PRESETS.some(p => p.id === pattern)) return pattern;
  return 'custom';
}

export function SettingsModal({ user, aesthetic, setAesthetic, dateFormat, setDateFormat, onSaved, onClose }) {
  const [baseCur, setBaseCur] = useState(user.base_currency);
  const [presetId, setPresetId] = useState(() => presetIdFor(dateFormat));
  const [customPattern, setCustomPattern] = useState(() =>
    presetIdFor(dateFormat) === 'custom' ? dateFormat : 'DD.MM.YYYY');
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [saving, setSaving] = useState(false);

  const effectivePattern = presetId === 'custom' ? customPattern : presetId;

  const save = async (e) => {
    e.preventDefault();
    setMsg(''); setErr('');
    setSaving(true);
    try {
      // Base currency is the only server-side field here; aesthetic
      // and date format are already persisted to localStorage via
      // their setters the moment the user picks them.
      if (baseCur !== user.base_currency) {
        const updated = await api.updateMe({ base_currency: baseCur });
        onSaved(updated);
      }
      setDateFormat(effectivePattern || 'browser');
      setMsg('Saved.');
    } catch (e2) {
      setErr(e2.message || 'Failed to save settings.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <div class="modal" style={{ maxWidth: 480 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h2 class="modal-title">Settings</h2>
            <div class="modal-sub">How the app looks and how numbers and dates are displayed.</div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        <form onSubmit={save}>
          <div class="field">
            <label>Base currency</label>
            <select class="select" value={baseCur} onChange={e => setBaseCur(e.currentTarget.value)}>
              {CURRENCIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4 }}>
              Reporting currency used for totals and charts.
            </div>
          </div>

          <div style={{ height: 1, background: 'var(--border)', margin: '16px 0' }} />

          <h3 style={{ fontSize: 15, fontWeight: 600, margin: '0 0 10px' }}>Aesthetic</h3>
          <div style={{ display: 'grid', gap: 6, marginBottom: 4 }}>
            {AESTHETICS.map(opt => (
              <button key={opt.id} type="button" onClick={() => setAesthetic(opt.id)}
                style={{
                  textAlign: 'left', padding: '8px 10px',
                  border: `1px solid ${aesthetic === opt.id ? 'var(--terra)' : 'var(--border)'}`,
                  background: aesthetic === opt.id ? 'var(--terra-wash)' : 'var(--bg-sunken)',
                  borderRadius: 'var(--radius-sm)',
                  color: aesthetic === opt.id ? 'var(--terra)' : 'var(--text)',
                  display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                }}>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 500 }}>{opt.label}</div>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 1 }}>{opt.sub}</div>
                </div>
                {aesthetic === opt.id && <Icon name="check" size={14} />}
              </button>
            ))}
          </div>

          <div style={{ height: 1, background: 'var(--border)', margin: '16px 0' }} />

          <h3 style={{ fontSize: 15, fontWeight: 600, margin: '0 0 10px' }}>Date format</h3>
          <div class="field">
            <select class="select" value={presetId} onChange={e => setPresetId(e.currentTarget.value)}>
              {DATE_PRESETS.map(p => <option key={p.id} value={p.id}>{p.label}</option>)}
            </select>
          </div>
          {presetId === 'custom' && (
            <div class="field">
              <label>Custom pattern</label>
              <input class="input" value={customPattern}
                onInput={e => setCustomPattern(e.currentTarget.value)}
                placeholder="e.g. DD.MM.YYYY" />
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4, fontFamily: 'var(--font-mono)' }}>
                Tokens: YYYY · YY · MMMM · MMM · MM · M · DD · D
              </div>
            </div>
          )}
          <div style={{
            fontSize: 12, color: 'var(--text-muted)', marginTop: 4,
            fontFamily: 'var(--font-mono)',
          }}>
            Today shows as: {previewDateFormat(effectivePattern) || '—'}
          </div>

          {err && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 12 }}>{err}</div>}
          {msg && <div style={{ color: 'var(--pos)', fontSize: 13, marginTop: 12 }}>{msg}</div>}

          <div class="modal-actions" style={{ justifyContent: 'flex-end' }}>
            <button type="button" class="btn" onClick={onClose}>Close</button>
            <button type="submit" class="btn primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save settings'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
