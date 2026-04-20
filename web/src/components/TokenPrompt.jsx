import { useState } from 'preact/hooks';
import { setToken } from '../api.js';

export function TokenPrompt({ onSaved }) {
  const [value, setValue] = useState('');
  return (
    <div class="modal-backdrop" style={{ position: 'fixed', inset: 0 }}>
      <form class="modal" onSubmit={e => { e.preventDefault(); if (value) { setToken(value.trim()); onSaved(); } }}>
        <h2 class="modal-title">Portfolio Tracker</h2>
        <div class="modal-sub">
          Paste an API token to continue. Create one with <code>ptadmin token create</code>.
        </div>
        <div class="field">
          <label>API token</label>
          <input class="input mono" type="password" autoFocus value={value}
            onInput={e => setValue(e.currentTarget.value)} placeholder="paste token here" />
        </div>
        <div class="modal-actions">
          <button type="submit" class="btn primary" disabled={!value}>Save token</button>
        </div>
      </form>
    </div>
  );
}
