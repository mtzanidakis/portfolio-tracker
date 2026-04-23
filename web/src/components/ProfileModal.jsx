import { useState } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { api } from '../api.js';

const MIN_PW = 8;
const CURRENCIES = ['USD', 'EUR', 'GBP', 'JPY', 'CHF', 'CAD', 'AUD'];
const AESTHETICS = [
  { id: 'technical', label: 'Technical', sub: 'Slate + electric blue' },
  { id: 'editorial', label: 'Editorial', sub: 'Neutral paper + red' },
  { id: 'forest',    label: 'Forest',    sub: 'Cool green + slate' },
];

export function ProfileModal({ user, aesthetic, setAesthetic, onClose, onSaved }) {
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email);
  const [baseCur, setBaseCur] = useState(user.base_currency);
  const [profileMsg, setProfileMsg] = useState('');
  const [profileErr, setProfileErr] = useState('');
  const [savingProfile, setSavingProfile] = useState(false);

  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');
  const [pwMsg, setPwMsg] = useState('');
  const [pwErr, setPwErr] = useState('');
  const [changingPw, setChangingPw] = useState(false);

  const saveProfile = async (e) => {
    e.preventDefault();
    setProfileMsg(''); setProfileErr('');
    setSavingProfile(true);
    try {
      const patch = {};
      if (name !== user.name) patch.name = name;
      if (email !== user.email) patch.email = email;
      if (baseCur !== user.base_currency) patch.base_currency = baseCur;
      if (Object.keys(patch).length === 0) {
        setProfileMsg('No changes.');
        return;
      }
      const updated = await api.updateMe(patch);
      setProfileMsg('Saved.');
      onSaved(updated);
    } catch (err) {
      setProfileErr(err.message || 'Failed to save profile.');
    } finally {
      setSavingProfile(false);
    }
  };

  const changePassword = async (e) => {
    e.preventDefault();
    setPwMsg(''); setPwErr('');
    if (newPw.length < MIN_PW) {
      setPwErr(`New password must be at least ${MIN_PW} characters.`);
      return;
    }
    if (newPw !== confirmPw) {
      setPwErr('Passwords do not match.');
      return;
    }
    setChangingPw(true);
    try {
      await api.changePassword(currentPw, newPw);
      setCurrentPw(''); setNewPw(''); setConfirmPw('');
      setPwMsg('Password changed. Other devices have been signed out.');
    } catch (err) {
      if (err.status === 401) setPwErr('Current password is incorrect.');
      else setPwErr(err.message || 'Failed to change password.');
    } finally {
      setChangingPw(false);
    }
  };

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <div class="modal" style={{ maxWidth: 460 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h2 class="modal-title">Profile</h2>
            <div class="modal-sub">Update your display name, email, reporting currency, or password.</div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        <form onSubmit={saveProfile}>
          <div class="field">
            <label>Name</label>
            <input class="input" value={name} onInput={e => setName(e.currentTarget.value)} />
          </div>
          <div class="field">
            <label>Email</label>
            <input class="input" type="email" value={email} onInput={e => setEmail(e.currentTarget.value)} />
          </div>
          <div class="field">
            <label>Base currency</label>
            <select class="select" value={baseCur} onChange={e => setBaseCur(e.currentTarget.value)}>
              {CURRENCIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          {profileErr && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 4 }}>{profileErr}</div>}
          {profileMsg && <div style={{ color: 'var(--pos)', fontSize: 13, marginTop: 4 }}>{profileMsg}</div>}
          <div class="modal-actions" style={{ justifyContent: 'flex-end' }}>
            <button type="submit" class="btn primary" disabled={savingProfile}>
              {savingProfile ? 'Saving…' : 'Save profile'}
            </button>
          </div>
        </form>

        <div style={{ height: 1, background: 'var(--border)', margin: '16px 0' }} />

        <h3 style={{ fontSize: 15, fontWeight: 600, margin: '0 0 10px' }}>Aesthetic</h3>
        <div style={{ display: 'grid', gap: 6, marginBottom: 18 }}>
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

        <h3 style={{ fontSize: 15, fontWeight: 600, margin: 0 }}>Change password</h3>
        <form onSubmit={changePassword}>
          <div class="field">
            <label>Current password</label>
            <input class="input" type="password" autocomplete="current-password"
              value={currentPw} onInput={e => setCurrentPw(e.currentTarget.value)} />
          </div>
          <div class="field">
            <label>New password (min {MIN_PW})</label>
            <input class="input" type="password" autocomplete="new-password"
              value={newPw} onInput={e => setNewPw(e.currentTarget.value)} />
          </div>
          <div class="field">
            <label>Confirm new password</label>
            <input class="input" type="password" autocomplete="new-password"
              value={confirmPw} onInput={e => setConfirmPw(e.currentTarget.value)} />
          </div>
          {pwErr && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 4 }}>{pwErr}</div>}
          {pwMsg && <div style={{ color: 'var(--pos)', fontSize: 13, marginTop: 4 }}>{pwMsg}</div>}
          <div class="modal-actions" style={{ justifyContent: 'flex-end' }}>
            <button type="submit" class="btn"
              disabled={!currentPw || newPw.length < MIN_PW || newPw !== confirmPw || changingPw}>
              {changingPw ? 'Updating…' : 'Change password'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
