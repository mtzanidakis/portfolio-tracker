import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { fmtMoney } from '../format.js';
import { api } from '../api.js';

export function AddModal({ onClose, onSaved }) {
  const [side, setSide] = useState('buy');
  const [sym, setSym] = useState('');
  const [qty, setQty] = useState('');
  const [price, setPrice] = useState('');
  const [fee, setFee] = useState('0');
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10));
  const [accountId, setAccountId] = useState(0);
  const [fxToBase, setFxToBase] = useState('1');
  const [note, setNote] = useState('');
  const [assets, setAssets] = useState([]);
  const [accounts, setAccounts] = useState([]);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    Promise.all([api.assets(), api.accounts()]).then(([a, accs]) => {
      const nonCash = (a || []).filter(x => x.type !== 'cash');
      setAssets(nonCash);
      setAccounts(accs || []);
      if (nonCash.length) setSym(nonCash[0].symbol);
      if (accs?.length) setAccountId(accs[0].id);
    }).catch(e => setError(e.message));
  }, []);

  const asset = assets.find(a => a.symbol === sym);
  const assetCurrency = asset?.currency || 'USD';
  const total = (parseFloat(qty) || 0) * (parseFloat(price) || 0);

  const submit = async (e) => {
    e.preventDefault();
    if (!sym || !accountId || !qty || !price) return;
    setSubmitting(true);
    setError('');
    try {
      await api.createTx({
        account_id: accountId,
        asset_symbol: sym,
        side,
        qty: parseFloat(qty),
        price: parseFloat(price),
        fee: parseFloat(fee) || 0,
        fx_to_base: parseFloat(fxToBase) || 1,
        occurred_at: new Date(date + 'T12:00:00Z').toISOString(),
        note,
      });
      if (onSaved) onSaved();
      onClose();
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <form class="modal" onSubmit={submit}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h2 class="modal-title">Add transaction</h2>
            <div class="modal-sub">Record a buy or sell manually.</div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        <div class="seg" style={{ marginBottom: 14 }}>
          <button type="button" class={side === 'buy' ? 'active buy' : ''} onClick={() => setSide('buy')}>Buy</button>
          <button type="button" class={side === 'sell' ? 'active sell' : ''} onClick={() => setSide('sell')}>Sell</button>
        </div>

        <div class="field">
          <label>Asset</label>
          <select class="select" value={sym} onChange={e => setSym(e.currentTarget.value)}>
            {assets.map(a => <option key={a.symbol} value={a.symbol}>{a.symbol} — {a.name}</option>)}
          </select>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Quantity</label>
            <input class="input mono" type="number" step="any" placeholder="0.00"
              value={qty} onInput={e => setQty(e.currentTarget.value)} autoFocus />
          </div>
          <div class="field">
            <label>Price per unit ({assetCurrency})</label>
            <input class="input mono" type="number" step="any" placeholder="0.00"
              value={price} onInput={e => setPrice(e.currentTarget.value)} />
          </div>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Date</label>
            <input class="input mono" type="date" value={date} onInput={e => setDate(e.currentTarget.value)} />
          </div>
          <div class="field">
            <label>Account</label>
            <select class="select" value={accountId} onChange={e => setAccountId(parseInt(e.currentTarget.value))}>
              {accounts.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
            </select>
          </div>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Fee ({assetCurrency})</label>
            <input class="input mono" type="number" step="any" value={fee}
              onInput={e => setFee(e.currentTarget.value)} />
          </div>
          <div class="field">
            <label>FX rate ({assetCurrency} → base)</label>
            <input class="input mono" type="number" step="any" value={fxToBase}
              onInput={e => setFxToBase(e.currentTarget.value)} />
          </div>
        </div>

        <div class="field">
          <label>Note</label>
          <input class="input" type="text" value={note} onInput={e => setNote(e.currentTarget.value)} />
        </div>

        <div style={{
          background: 'var(--bg-sunken)', border: '1px solid var(--border)',
          borderRadius: 'var(--radius-sm)', padding: '12px 14px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 6,
        }}>
          <div>
            <div style={{ fontSize: 11, color: 'var(--text-faint)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>Total</div>
            <div class="mono" style={{ fontSize: 20, fontWeight: 500, marginTop: 2 }}>
              {fmtMoney(total, assetCurrency)}
            </div>
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', textAlign: 'right' }}>
            {qty && price ? `${qty} ${sym} @ ${fmtMoney(parseFloat(price), assetCurrency)}` : 'Enter quantity & price'}
          </div>
        </div>

        {error && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 8 }}>{error}</div>}

        <div class="modal-actions">
          <button type="button" class="btn" onClick={onClose}>Cancel</button>
          <button type="submit" class="btn primary" disabled={!qty || !price || submitting}>
            <Icon name="check" /> {submitting ? 'Saving…' : side === 'buy' ? 'Record buy' : 'Record sell'}
          </button>
        </div>
      </form>
    </div>
  );
}
