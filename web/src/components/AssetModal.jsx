import { useEffect, useRef, useState } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { AssetLogo } from './AssetLogo.jsx';
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

export function AssetModal({ asset, onClose, onSaved }) {
  const editing = !!asset;
  const [symbol, setSymbol] = useState(asset?.symbol || '');
  const [name, setName] = useState(asset?.name || '');
  const [type, setType] = useState(asset?.type || 'stock');
  const [currency, setCurrency] = useState(asset?.currency || 'USD');
  const [provider, setProvider] = useState(asset?.provider || 'yahoo');
  const [providerID, setProviderID] = useState(asset?.provider_id || '');
  const [logoURL, setLogoURL] = useState(asset?.logo_url || '');
  const [err, setErr] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [looking, setLooking] = useState(false);

  // Cash is just a balance in a currency — no ticker, no external price
  // source. The symbol/name are derived from the currency so the DB row
  // keeps a unique key without asking the user for one.
  const isCash = type === 'cash';

  // When the user picks a different asset type, snap the provider to
  // the one that actually serves prices for it — CoinGecko for crypto,
  // Yahoo for stocks/ETFs. The first render is skipped so editing an
  // existing row doesn't rewrite the stored provider behind the user's
  // back. Cash rows keep the empty provider the submit handler sends.
  const typeFirstRun = useRef(true);
  useEffect(() => {
    if (typeFirstRun.current) {
      typeFirstRun.current = false;
      return;
    }
    if (type === 'crypto' && provider !== 'coingecko') setProvider('coingecko');
    else if ((type === 'stock' || type === 'etf') && provider !== 'yahoo') setProvider('yahoo');
  }, [type]); // eslint-disable-line react-hooks/exhaustive-deps

  // Debounced provider lookup: whenever symbol or provider changes the
  // form re-queries and auto-fills name / currency / type / provider-id
  // / logo_url. On mount for an existing asset we suppress the initial
  // fire so the user's stored values aren't clobbered; subsequent
  // provider changes do fire, which is how editing a broken asset (e.g.
  // coingecko BTC with provider_id=BTC) self-heals to provider_id=bitcoin.
  const lookupSeq = useRef(0);
  const firstRun = useRef(true);
  useEffect(() => {
    const isFirst = firstRun.current;
    firstRun.current = false;
    if (isFirst && editing) return;
    if (isCash) {
      setLooking(false);
      return;
    }

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
        setLogoURL(info?.logo_url || '');
      } catch {
        // 404 / network hiccup — leave the user's current values alone.
      } finally {
        if (seq === lookupSeq.current) setLooking(false);
      }
    }, 400);
    return () => clearTimeout(t);
  }, [symbol, provider, editing, isCash]);

  const submit = async (e) => {
    e.preventDefault();
    setErr('');
    if (!isCash && (!symbol.trim() || !name.trim())) {
      setErr('Symbol and name are required.');
      return;
    }
    setSubmitting(true);
    try {
      const payload = isCash
        ? {
            symbol: `CASH-${currency}`,
            name: `${currency} Cash`,
            type,
            currency,
            provider: '',
            provider_id: '',
            logo_url: '',
          }
        : {
            symbol: symbol.trim().toUpperCase(),
            name: name.trim(),
            type,
            currency,
            provider,
            provider_id: providerID.trim(),
            logo_url: logoURL,
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

  // Preview of the logo the form will save — re-rendered live as the
  // lookup resolves or the user toggles between cash and non-cash. The
  // asset isn't in the DB yet so we pass the raw upstream URL via
  // `previewURL` instead of the usual proxy path.
  const previewAsset = isCash
    ? { type: 'cash', currency, symbol: `CASH-${currency}` }
    : { type, currency, symbol: symbol.trim().toUpperCase(), logo_url: logoURL };
  const previewLogoURL = !isCash ? logoURL : undefined;

  return (
    <div class="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
      <form class="modal" onSubmit={submit}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12 }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <AssetLogo asset={previewAsset} previewURL={previewLogoURL} size={40} />
            <div>
              <h2 class="modal-title">{editing ? 'Edit asset' : 'Add asset'}</h2>
              <div class="modal-sub">
                Ticker / name / currency. Provider info drives the price refresher.
              </div>
            </div>
          </div>
          <button type="button" class="icon-btn" onClick={onClose}><Icon name="close" /></button>
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
            <select class="select" value={currency} onChange={e => setCurrency(e.currentTarget.value)} disabled={editing && isCash}>
              {CURRENCIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
        </div>

        {!isCash && (
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
        )}

        {!isCash && (
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
        )}

        {err && <div style={{ color: 'var(--neg)', fontSize: 13, marginTop: 8 }}>{err}</div>}

        <div class="modal-actions">
          <button type="button" class="btn" onClick={onClose}>Cancel</button>
          <button type="submit" class="btn primary"
            disabled={submitting || (!isCash && (!symbol.trim() || !name.trim()))}>
            <Icon name="check" />
            {submitting ? 'Saving…' : editing ? 'Save changes' : 'Create asset'}
          </button>
        </div>
      </form>
    </div>
  );
}
