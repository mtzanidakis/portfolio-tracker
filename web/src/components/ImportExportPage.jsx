import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { api } from '../api.js';

// Sources the wizard can import from. Extend this array when a new
// parser lands on the backend — the value goes straight into the
// /api/v1/import/{source}/analyze path.
const SOURCES = [
  { id: 'ghostfolio', label: 'Ghostfolio', hint: 'JSON export from Settings → Import/Export' },
];

// Steps in the wizard. We keep them as a linear flow — back/next
// buttons are enough, no URL routing.
const STEPS = ['upload', 'review', 'confirm', 'done'];

export function ImportExportPage() {
  const [source, setSource] = useState('ghostfolio');
  const [step, setStep] = useState('upload');
  const [plan, setPlan] = useState(null);          // normalised ImportPlan
  const [result, setResult] = useState(null);      // apply result
  const [existingAccounts, setExistingAccounts] = useState([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');

  // The review step needs the user's current accounts to populate the
  // per-row "Map to existing ▼" dropdown. We load them when entering
  // review so the dropdown is always fresh.
  useEffect(() => {
    if (step !== 'review') return;
    api.accounts().then(setExistingAccounts).catch(() => setExistingAccounts([]));
  }, [step]);

  const reset = () => {
    setPlan(null); setResult(null); setErr('');
    setStep('upload');
  };

  const onFile = async (file) => {
    setErr(''); setBusy(true);
    try {
      if (file.size > 10 * 1024 * 1024) {
        throw new Error('File too large (max 10 MB).');
      }
      const text = await file.text();
      let parsed;
      try { parsed = JSON.parse(text); }
      catch { throw new Error('Not a valid JSON file.'); }
      const p = await api.importAnalyze(source, parsed);
      setPlan(p);
      setStep('review');
    } catch (e) {
      setErr(e.message || 'Analyze failed.');
    } finally {
      setBusy(false);
    }
  };

  // Selected/MapToID live inside the plan itself — mutating in place
  // keeps /apply round-tripping trivial. We clone the plan's top-level
  // arrays on updates so Preact re-renders.
  const patchAccount = (i, patch) => {
    const next = { ...plan, accounts: [...plan.accounts] };
    next.accounts[i] = { ...next.accounts[i], ...patch };
    setPlan(next);
  };
  const patchAsset = (i, patch) => {
    const next = { ...plan, assets: [...plan.assets] };
    next.assets[i] = { ...next.assets[i], ...patch };
    setPlan(next);
  };

  const apply = async () => {
    setErr(''); setBusy(true);
    try {
      const r = await api.importApply(plan);
      setResult(r);
      setStep('done');
    } catch (e) {
      setErr(e.message || 'Import failed.');
    } finally {
      setBusy(false);
    }
  };

  const effectivePlan = plan && {
    accounts: plan.accounts.filter(a => a.selected),
    assets:   plan.assets.filter(a => a.selected),
    txs:      plan.transactions,
  };

  return (
    <>
      <div class="card">
        <div class="card-header">
          <div>
            <div class="card-title">Import</div>
            <div style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 2 }}>
              Bring accounts, assets and transactions over from another portfolio tracker.
            </div>
          </div>
          <Stepper step={step} />
        </div>

        {err && (
          <div style={{ background: 'var(--neg-wash)', color: 'var(--neg)', padding: '8px 12px',
                        borderRadius: 'var(--radius-sm)', fontSize: 13, margin: '0 0 12px' }}>
            {err}
          </div>
        )}

        {step === 'upload' && (
          <UploadStep
            source={source} setSource={setSource}
            onFile={onFile} busy={busy}
          />
        )}

        {step === 'review' && plan && (
          <ReviewStep
            plan={plan}
            existingAccounts={existingAccounts}
            patchAccount={patchAccount}
            patchAsset={patchAsset}
            onBack={reset}
            onNext={() => setStep('confirm')}
          />
        )}

        {step === 'confirm' && plan && (
          <ConfirmStep
            plan={plan}
            effective={effectivePlan}
            onBack={() => setStep('review')}
            onApply={apply}
            busy={busy}
          />
        )}

        {step === 'done' && result && (
          <DoneStep result={result} onReset={reset} />
        )}
      </div>

      <div class="card mt-16">
        <div class="card-header">
          <div>
            <div class="card-title">Export</div>
            <div style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 2 }}>
              Download your data. Files are generated on the fly, nothing is cached server-side.
            </div>
          </div>
        </div>
        <div style={{ display: 'grid', gap: 10, maxWidth: 560 }}>
          <ExportRow
            title="Full portfolio backup (JSON)"
            sub="Accounts, assets and transactions in one self-describing file."
            href={api.exportURL('json')}
            label="Download JSON" />
          <ExportRow
            title="Transactions only (CSV)"
            sub="One row per transaction, spreadsheet-friendly."
            href={api.exportURL('csv')}
            label="Download CSV" />
        </div>
      </div>
    </>
  );
}

function Stepper({ step }) {
  return (
    <div style={{ display: 'flex', gap: 6, alignItems: 'center', fontSize: 11,
                  color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
      {STEPS.map((s, i) => (
        <span key={s} style={{
          padding: '3px 8px', borderRadius: 999,
          border: '1px solid var(--border)',
          background: step === s ? 'var(--terra-wash)' : 'transparent',
          color: step === s ? 'var(--terra)' : 'var(--text-muted)',
          textTransform: 'uppercase', letterSpacing: '0.08em',
        }}>{i + 1}. {s}</span>
      ))}
    </div>
  );
}

function UploadStep({ source, setSource, onFile, busy }) {
  const hint = SOURCES.find(s => s.id === source)?.hint || '';
  const [fileName, setFileName] = useState('');
  return (
    <>
      <div class="field" style={{ maxWidth: 360 }}>
        <label>Source software</label>
        <select class="select" value={source} onChange={e => setSource(e.currentTarget.value)}>
          {SOURCES.map(s => <option key={s.id} value={s.id}>{s.label}</option>)}
        </select>
        {hint && <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4 }}>{hint}</div>}
      </div>
      <div class="field" style={{ maxWidth: 360 }}>
        <label>Export file</label>
        <label class="file-input">
          <span class="fi-btn">
            <Icon name="arrowUp" size={14} />
            Choose file
          </span>
          <span class="fi-name">{fileName || 'No file chosen'}</span>
          <input type="file" accept=".json,application/json"
            disabled={busy}
            onChange={e => {
              const f = e.currentTarget.files?.[0];
              if (!f) return;
              setFileName(f.name);
              onFile(f);
            }} />
        </label>
        {busy && <div style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>Analyzing…</div>}
      </div>
    </>
  );
}

function ReviewStep({ plan, existingAccounts, patchAccount, patchAsset, onBack, onNext }) {
  const allSelectedAccounts = plan.accounts.every(a => a.selected);
  const allSelectedAssets = plan.assets.every(a => a.selected);

  return (
    <>
      {plan.warnings?.length > 0 && (
        <div style={{
          background: 'var(--bg-sunken)', padding: '10px 12px',
          borderRadius: 'var(--radius-sm)', border: '1px solid var(--border)',
          marginBottom: 14,
        }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-muted)',
                         textTransform: 'uppercase', letterSpacing: '0.08em' }}>
            Warnings
          </div>
          <ul style={{ margin: '6px 0 0', paddingLeft: 18, fontSize: 13 }}>
            {plan.warnings.map((w, i) => <li key={i}>{w}</li>)}
          </ul>
        </div>
      )}

      <h3 style={{ fontSize: 14, fontWeight: 600, margin: '4px 0 8px' }}>
        Accounts ({plan.accounts.length})
      </h3>
      <table class="table">
        <thead>
          <tr>
            <th style={{ width: 32 }}>
              <input type="checkbox" checked={allSelectedAccounts}
                onChange={e => plan.accounts.forEach((_, i) => patchAccount(i, { selected: e.currentTarget.checked }))} />
            </th>
            <th>Name</th>
            <th>Currency</th>
            <th style={{ textAlign: 'right' }}>Activities</th>
            <th>Map to existing</th>
          </tr>
        </thead>
        <tbody>
          {plan.accounts.length === 0 && (
            <tr><td colspan="5" class="empty">No accounts with activities in this export.</td></tr>
          )}
          {plan.accounts.map((a, i) => (
            <tr key={a.source_id}>
              <td><input type="checkbox" checked={a.selected}
                onChange={e => patchAccount(i, { selected: e.currentTarget.checked })} /></td>
              <td data-primary>{a.name}</td>
              <td class="mono">{a.currency}</td>
              <td class="num" style={{ textAlign: 'right' }}>{a.tx_count}</td>
              <td>
                <select class="select" value={a.map_to_id || ''}
                  onChange={e => patchAccount(i, { map_to_id: Number(e.currentTarget.value) || 0 })}
                  style={{ maxWidth: 220 }}>
                  <option value="">Create new</option>
                  {existingAccounts.map(ex => (
                    <option key={ex.id} value={ex.id}>
                      {ex.name} · {ex.currency}{ex.id === a.existing_id ? '  (name match)' : ''}
                    </option>
                  ))}
                </select>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <h3 style={{ fontSize: 14, fontWeight: 600, margin: '20px 0 8px' }}>
        Assets ({plan.assets.length})
      </h3>
      <table class="table">
        <thead>
          <tr>
            <th style={{ width: 32 }}>
              <input type="checkbox" checked={allSelectedAssets}
                onChange={e => plan.assets.forEach((_, i) => patchAsset(i, { selected: e.currentTarget.checked }))} />
            </th>
            <th>Symbol</th>
            <th>Name</th>
            <th>Type</th>
            <th>Currency</th>
            <th>Provider</th>
            <th style={{ textAlign: 'right' }}>Activities</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {plan.assets.length === 0 && (
            <tr><td colspan="8" class="empty">No assets with activities in this export.</td></tr>
          )}
          {plan.assets.map((a, i) => (
            <tr key={a.source_id}>
              <td><input type="checkbox" checked={a.selected}
                onChange={e => patchAsset(i, { selected: e.currentTarget.checked })} /></td>
              <td class="mono" data-primary>{a.symbol}</td>
              <td>{a.name}</td>
              <td><span class={`pill ${a.type}`}>{a.type}</span></td>
              <td class="mono">{a.currency}</td>
              <td style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                {a.provider || '—'}
              </td>
              <td class="num" style={{ textAlign: 'right' }}>{a.tx_count}</td>
              <td style={{ fontSize: 12 }}>
                {a.existing_match
                  ? <span style={{ color: 'var(--text-muted)' }}>Already exists — will reuse</span>
                  : <span style={{ color: 'var(--text)' }}>New</span>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <div class="modal-actions" style={{ justifyContent: 'space-between', marginTop: 20 }}>
        <button type="button" class="btn" onClick={onBack}>Back</button>
        <button type="button" class="btn primary" onClick={onNext}>Next</button>
      </div>
    </>
  );
}

function ConfirmStep({ plan, effective, onBack, onApply, busy }) {
  const txPlanned = effective.accounts.length > 0 && effective.assets.length > 0
    ? plan.transactions.length : 0;
  return (
    <div>
      <div style={{ fontSize: 14, marginBottom: 12 }}>
        Ready to import. This will:
      </div>
      <ul style={{ fontSize: 14, lineHeight: 1.7, paddingLeft: 22 }}>
        <li>
          Create <strong>{effective.accounts.filter(a => !a.map_to_id).length}</strong> account(s),
          reuse <strong>{effective.accounts.filter(a => a.map_to_id).length}</strong>.
        </li>
        <li>
          Create <strong>{effective.assets.filter(a => !a.existing_match).length}</strong> asset(s),
          reuse <strong>{effective.assets.filter(a => a.existing_match).length}</strong>.
        </li>
        <li>
          Insert <strong>{txPlanned}</strong> transaction(s). FX rates will be fetched for those in
          a non-base currency.
        </li>
      </ul>
      <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 10 }}>
        Applied atomically — if anything fails, nothing is written.
      </div>
      <div class="modal-actions" style={{ justifyContent: 'space-between', marginTop: 20 }}>
        <button type="button" class="btn" onClick={onBack} disabled={busy}>Back</button>
        <button type="button" class="btn primary" onClick={onApply} disabled={busy}>
          {busy ? 'Importing…' : 'Import'}
        </button>
      </div>
    </div>
  );
}

function DoneStep({ result, onReset }) {
  return (
    <div>
      <div style={{
        background: 'var(--pos-wash)', color: 'var(--pos)', padding: '10px 12px',
        borderRadius: 'var(--radius-sm)', fontSize: 13, marginBottom: 12,
      }}>
        Import complete.
      </div>
      <ul style={{ fontSize: 14, lineHeight: 1.7, paddingLeft: 22 }}>
        <li>{result.accounts_created} account(s) created, {result.accounts_reused} reused.</li>
        <li>{result.assets_created} asset(s) created, {result.assets_reused} reused.</li>
        <li>{result.transactions_created} transaction(s) inserted.</li>
      </ul>
      {result.warnings?.length > 0 && (
        <ul style={{ fontSize: 12, color: 'var(--text-muted)', paddingLeft: 22, marginTop: 8 }}>
          {result.warnings.map((w, i) => <li key={i}>{w}</li>)}
        </ul>
      )}
      <div class="modal-actions" style={{ justifyContent: 'flex-end', marginTop: 20 }}>
        <button type="button" class="btn primary" onClick={onReset}>Import more</button>
      </div>
    </div>
  );
}

function ExportRow({ title, sub, href, label }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      gap: 12, padding: 12, border: '1px solid var(--border)',
      borderRadius: 'var(--radius-sm)', background: 'var(--bg-sunken)',
    }}>
      <div>
        <div style={{ fontSize: 14, fontWeight: 500 }}>{title}</div>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 2 }}>{sub}</div>
      </div>
      <a class="btn primary" href={href} download
        style={{ minWidth: 160, justifyContent: 'center' }}>
        <Icon name="arrowDown" size={14} /> {label}
      </a>
    </div>
  );
}
