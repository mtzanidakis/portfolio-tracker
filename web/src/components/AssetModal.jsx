import { useEffect, useRef, useState } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { api } from '../api.js';

const CURRENCIES = ['USD', 'EUR', 'GBP', 'JPY', 'CHF', 'CAD', 'AUD'];
const TYPES = [
  { value: 'stock',  label: 'Stock' },
  { value: 'etf',    label: 'ETF' },
  { value: 'crypto', label: 'Crypto' },
  { value: 'cash',   label: 'Cash' },
];
const PROVIDERS = [
  { id: '',          label: '(none)' },
  { id: 'yahoo',     label: 'Yahoo Finance' },
  { id: 'coingecko', label: 'CoinGecko' },
];
const COLOURS = ['#c8502a', '#d4953d', '#a8572e', '#7a8c6f', '#b8632e', '#c9a87c'];

export function AssetModal({ asset, onClose, onSaved }) {
  const editing = !!asset;
  const [symbol, setSymbol] = useState(asset?.symbol || '');
  const [name, setName] = useState(asset?.name || '');
  const [type, setType] = useState(asset?.type || 'stock');
  const [currency, setCurrency] = useState(asset?.currency || 'USD');
  const [color, setColor] = useState(asset?.color || COLOURS[0]);
  const [provider, setProvider] = useState(asset?.provider || 'yahoo');
  const [providerID, setProviderID] = useState(asset?.provider_id || '');
  const [err, setErr] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [looking, setLooking] = useState(false);

  // Debounced provider lookup: whenever symbol or provider changes the
  // form re-queries and auto-fills name / currency / type / provider-id.
  // On mount for an existing asset we suppress the initial fire so the
  // user's stored values aren't clobbered; subsequent provider changes
  // do fire, which is how editing a broken asset (e.g. coingecko BTC
  // with provider_id=BTC) self-heals to provider_id=bitcoin.
  const lookupSeq = useRef(0);
  const firstRun = useRef(true);
  useEffect(() => {
    const isFirst = firstRun.current;
    firstRun.current = false;
    if (isFirst && editing) return;

    const sym = symbol.trim();
    if (!sym || !provider) {
      setLooking(false);
      return;
    }
    const seq = ++lookupSeq.current;
    setLooking(true);
    const t = setTimeout(async () => {
      try {
        const info = await api.lookupAsset(sym, provider);
        if (seq !== lookupSeq.current) return;
        if (info?.name) setName(info.name);
        if (info?.currency) setCurrency(info.currency);
        if (info?.type) setType(info.type);
        if (info?.provider_id) setProviderID(info.provider_id);
      } catch {
        // 404 / network hiccup — leave the user's current values alone.
      } finally {
        if (seq === lookupSeq.current) setLooking(false);
      }
    }, 400);
    return () => clearTimeout(t);
  }, [symbol, provider, editing]);

  const submit = async (e) => {
    e.preventDefault();
    setErr('');
    if (!symbol.trim() || !name.trim()) {
      setErr('Symbol and name are required.');
      return;
    }
    setSubmitting(true);
    try {
      const payload = {
        symbol: symbol.trim().toUpperCase(),
        name: name.trim(),
        type,
        currency,
        color,
        provider,
        provider_id: providerID.trim(),
      };
      // POST is upsert, so we use it for both create and edit.
      const saved = await api.upsertAsset(payload);
      onSaved(saved);
      onClose();
    } catch (e) {
      setErr(e.message || 'Failed to save asset.');
    } finally {
      setSubmitting(false);
    }
  };

  const providerHint = provider === 'coingecko'
    ? 'e.g. "bitcoin", "ethereum" (CoinGecko coin id)'
    : provider === 'yahoo'
    ? 'optional; defaults to the symbol (e.g. AAPL, VOO)'
    : 'no external provider; leave blank';

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <form class="modal" onSubmit={submit}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h2 class="modal-title">{editing ? 'Edit asset' : 'Add asset'}</h2>
            <div class="modal-sub">
              Ticker / name / currency. Provider info drives the price refresher.
            </div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Symbol</label>
            <input class="input mono" autoFocus
              placeholder="AAPL, BTC, VOO"
              value={symbol}
              disabled={editing}
              onInput={e => setSymbol(e.currentTarget.value.toUpperCase())} />
          </div>
          <div class="field">
            <label>Name {looking && <span style={{ fontSize: 11, color: 'var(--muted)' }}>· looking up…</span>}</label>
            <input class="input"
              placeholder="Apple Inc."
              value={name} onInput={e => setName(e.currentTarget.value)} />
          </div>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Type</label>
            <select class="select" value={type} onChange={e => setType(e.currentTarget.value)}>
              {TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
          </div>
          <div class="field">
            <label>Native currency</label>
            <select class="select" value={currency} onChange={e => setCurrency(e.currentTarget.value)}>
              {CURRENCIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
        </div>

        <div class="row-2">
          <div class="field">
            <label>Provider</label>
            <select class="select" value={provider} onChange={e => setProvider(e.currentTarget.value)}>
              {PROVIDERS.map(p => <option key={p.id} value={p.id}>{p.label}</option>)}
            </select>
          </div>
          <div class="field">
            <label>Provider ID</label>
            <input class="input mono"
              placeholder={providerHint}
              value={providerID}
              onInput={e => setProviderID(e.currentTarget.value)} />
          </div>
        </div>

        <div class="field">
          <label>Color</label>
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center' }}>
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

        {err && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 8 }}>{err}</div>}

        <div class="modal-actions">
          <button type="button" class="btn" onClick={onClose}>Cancel</button>
          <button type="submit" class="btn primary"
            disabled={!symbol.trim() || !name.trim() || submitting}>
            <Icon name="check" />
            {submitting ? 'Saving…' : editing ? 'Save changes' : 'Create asset'}
          </button>
        </div>
      </form>
    </div>
  );
}
