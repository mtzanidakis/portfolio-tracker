import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { fmtMoney } from '../format.js';
import { api } from '../api.js';

// Modal for creating or editing a transaction. Pass `transaction` to
// enter edit mode. User is required so we know the base currency for
// the FX conversion step.
export function TxModal({ transaction, user, onClose, onSaved }) {
  const editing = !!transaction;

  const [side, setSide] = useState(transaction?.side || 'buy');
  const [sym, setSym] = useState(transaction?.asset_symbol || '');
  const [qty, setQty] = useState(transaction ? String(transaction.qty) : '');
  const [price, setPrice] = useState(transaction ? String(transaction.price) : '');
  const [fee, setFee] = useState(transaction ? String(transaction.fee || 0) : '0');
  const [date, setDate] = useState(
    transaction?.occurred_at
      ? transaction.occurred_at.slice(0, 10)
      : new Date().toISOString().slice(0, 10),
  );
  const [accountId, setAccountId] = useState(transaction?.account_id || 0);
  const [fxToBase, setFxToBase] = useState(
    transaction ? String(transaction.fx_to_base || 1) : '1',
  );
  const [fxAuto, setFxAuto] = useState(!editing); // default to auto for new tx
  const [fxLoading, setFxLoading] = useState(false);
  const [note, setNote] = useState(transaction?.note || '');
  const [assets, setAssets] = useState([]);
  const [accounts, setAccounts] = useState([]);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    Promise.all([api.assets(), api.accounts()]).then(([a, accs]) => {
      const nonCash = (a || []).filter(x => x.type !== 'cash');
      setAssets(nonCash);
      setAccounts(accs || []);
      if (!sym && nonCash.length) setSym(nonCash[0].symbol);
      if (!accountId && accs?.length) setAccountId(accs[0].id);
    }).catch(e => setError(e.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const account = accounts.find(a => a.id === accountId);
  const accountCurrency = account?.currency || 'USD';
  const baseCurrency = user?.base_currency || 'USD';
  const needsFx = accountCurrency !== baseCurrency;

  // Auto-calculate fx_to_base whenever account / base / date changes.
  // Skipped when not needed (same currency) or when the user has
  // toggled auto off.
  useEffect(() => {
    if (!needsFx) {
      setFxToBase('1');
      return;
    }
    if (!fxAuto) return;
    let cancelled = false;
    setFxLoading(true);
    api.fxRate(accountCurrency, baseCurrency, date)
      .then(r => { if (!cancelled) setFxToBase(String(r.rate)); })
      .catch(() => { /* keep current value on failure */ })
      .finally(() => { if (!cancelled) setFxLoading(false); });
    return () => { cancelled = true; };
  }, [accountCurrency, baseCurrency, date, needsFx, fxAuto]);

  const total = (parseFloat(qty) || 0) * (parseFloat(price) || 0);

  const submit = async (e) => {
    e.preventDefault();
    if (!sym || !accountId || !qty || !price) return;
    setSubmitting(true);
    setError('');
    try {
      const payload = {
        account_id: accountId,
        asset_symbol: sym,
        side,
        qty: parseFloat(qty),
        price: parseFloat(price),
        fee: parseFloat(fee) || 0,
        fx_to_base: needsFx ? (parseFloat(fxToBase) || 1) : 1,
        occurred_at: new Date(date + 'T12:00:00Z').toISOString(),
        note,
      };
      if (editing) {
        await api.updateTx(transaction.id, payload);
      } else {
        await api.createTx(payload);
      }
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
            <h2 class="modal-title">{editing ? 'Edit transaction' : 'Add transaction'}</h2>
            <div class="modal-sub">
              {editing ? 'Change any field and save.' : 'Record a buy or sell manually.'}
            </div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        <div class="seg" style={{ marginBottom: 14 }}>
          <button type="button" class={side === 'buy' ? 'active buy' : ''} onClick={() => setSide('buy')}>Buy</button>
          <button type="button" class={side === 'sell' ? 'active sell' : ''} onClick={() => setSide('sell')}>Sell</button>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Account</label>
            <select class="select" value={accountId} onChange={e => setAccountId(parseInt(e.currentTarget.value))}>
              {accounts.map(a => (
                <option key={a.id} value={a.id}>{a.name} ({a.currency})</option>
              ))}
            </select>
          </div>
          <div class="field">
            <label>Asset</label>
            <select class="select" value={sym} onChange={e => setSym(e.currentTarget.value)}>
              {assets.map(a => <option key={a.symbol} value={a.symbol}>{a.symbol} — {a.name}</option>)}
            </select>
          </div>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Quantity</label>
            <input class="input mono" type="number" step="any" placeholder="0.00"
              value={qty} onInput={e => setQty(e.currentTarget.value)} autoFocus />
          </div>
          <div class="field">
            <label>Price per unit ({accountCurrency})</label>
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
            <label>Fee ({accountCurrency})</label>
            <input class="input mono" type="number" step="any" value={fee}
              onInput={e => setFee(e.currentTarget.value)} />
          </div>
        </div>

        {needsFx && (
          <div class="field">
            <label style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>FX rate (1 {accountCurrency} = ? {baseCurrency}, locked at trade time)</span>
              <span style={{ fontSize: 11, color: 'var(--text-faint)', display: 'flex', alignItems: 'center', gap: 6 }}>
                {fxLoading && <span>fetching…</span>}
                <label style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer' }}>
                  <input type="checkbox" checked={fxAuto}
                    onChange={e => setFxAuto(e.currentTarget.checked)}
                    style={{ margin: 0 }} />
                  auto
                </label>
              </span>
            </label>
            <input class="input mono" type="number" step="any" value={fxToBase}
              disabled={fxAuto && fxLoading}
              onInput={e => { setFxToBase(e.currentTarget.value); setFxAuto(false); }} />
          </div>
        )}

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
              {fmtMoney(total, accountCurrency)}
            </div>
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', textAlign: 'right' }}>
            {qty && price
              ? `${qty} ${sym} @ ${fmtMoney(parseFloat(price), accountCurrency)}`
              : 'Enter quantity & price'}
          </div>
        </div>

        {error && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 8 }}>{error}</div>}

        <div class="modal-actions">
          <button type="button" class="btn" onClick={onClose}>Cancel</button>
          <button type="submit" class="btn primary" disabled={!qty || !price || submitting}>
            <Icon name="check" />
            {submitting
              ? 'Saving…'
              : editing ? 'Save changes'
              : side === 'buy' ? 'Record buy' : 'Record sell'}
          </button>
        </div>
      </form>
    </div>
  );
}
