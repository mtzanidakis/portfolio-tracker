import { useState, useEffect } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { PerformanceChart } from './Chart.jsx';
import { fmtMoney, fmtPct } from '../format.js';
import { api } from '../api.js';

const TFS = ['1D', '1W', '1M', '3M', '6M', '1Y', 'ALL'];

export function PerformancePage({ privacy, currency }) {
  const [tf, setTf] = useState('6M');
  const [perf, setPerf] = useState(null);
  const [holdings, setHoldings] = useState([]);
  const [err, setErr] = useState(null);

  useEffect(() => {
    setErr(null);
    api.performance(tf).then(setPerf).catch(e => setErr(e.message));
  }, [tf]);

  useEffect(() => {
    api.holdings().then(setHoldings).catch(e => setErr(e.message));
  }, []);

  if (err) return <div class="empty">Error: {err}</div>;
  if (!perf) return <div class="empty">Loading…</div>;

  const movers = [...(holdings || [])]
    .filter(h => h.Qty > 0)
    .sort((a, b) => (b.PnLPctBase || 0) - (a.PnLPctBase || 0));

  const series = (perf.series || []).map(p => ({ d: p.at, v: p.value }));

  return (
    <>
      <div class="hero">
        <div class="hero-main">
          <div class="hero-label">Total portfolio value</div>
          <div class="hero-value">
            {privacy ? <span class="masked">{fmtMoney(perf.total, currency)}</span> : fmtMoney(perf.total, currency)}
          </div>
          <div class={`hero-delta ${perf.pnl < 0 ? 'neg' : ''}`}>
            <Icon name={perf.pnl >= 0 ? 'arrowUp' : 'arrowDown'} size={12} />
            {fmtMoney(perf.pnl, currency, { sign: true })} · {fmtPct(perf.pnl_pct)}
          </div>
          <div style={{ marginTop: 6, fontSize: 12, color: 'var(--text-faint)' }}>all time</div>
        </div>
        <div class="hero-side">
          <div class="stat">
            <div>
              <div class="stat-label">Cost basis</div>
              <div class="stat-value">
                {privacy ? <span class="masked">{fmtMoney(perf.cost, currency)}</span> : fmtMoney(perf.cost, currency)}
              </div>
            </div>
            <div class="stat-sub">Across {holdings?.length || 0} holdings</div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <div>
            <div class="card-title">Portfolio performance</div>
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 13, color: perf.pnl >= 0 ? 'var(--pos)' : 'var(--neg)', marginTop: 4 }}>
              {fmtMoney(perf.pnl, currency, { sign: true })} · {fmtPct(perf.pnl_pct)} <span style={{ color: 'var(--text-faint)' }}>total</span>
            </div>
          </div>
          <div class="timeframe">
            {TFS.map(t => (
              <button key={t} class={tf === t ? 'active' : ''} onClick={() => setTf(t)}>{t}</button>
            ))}
          </div>
        </div>
        <PerformanceChart series={series} privacy={privacy} currency={currency} />
      </div>

      <div class="card mt-16">
        <div class="card-header">
          <div class="card-title">Top movers · all time</div>
        </div>
        <table class="table">
          <thead>
            <tr>
              <th>Asset</th>
              <th style={{ textAlign: 'right' }}>Value</th>
              <th style={{ textAlign: 'right' }}>Cost basis</th>
              <th style={{ textAlign: 'right' }}>PnL</th>
              <th style={{ textAlign: 'right' }}>Return</th>
            </tr>
          </thead>
          <tbody>
            {movers.length === 0 && (
              <tr><td colspan="5" class="empty">No holdings yet.</td></tr>
            )}
            {movers.map(h => (
              <tr key={h.Symbol}>
                <td>
                  <div class="ticker">
                    <div class="ticker-icon" style={{ background: 'var(--terra)' }}>{h.Symbol.slice(0, 2)}</div>
                    <div class="ticker-meta">
                      <div class="ticker-sym">{h.Symbol}</div>
                      <div class="ticker-name">{h.Currency}</div>
                    </div>
                  </div>
                </td>
                <td class="num" style={{ textAlign: 'right' }}>
                  {privacy ? <span class="masked">{fmtMoney(h.ValueBase, currency)}</span> : fmtMoney(h.ValueBase, currency)}
                </td>
                <td class="num" style={{ textAlign: 'right', color: 'var(--text-muted)' }}>
                  {privacy ? <span class="masked">{fmtMoney(h.CostBase, currency)}</span> : fmtMoney(h.CostBase, currency)}
                </td>
                <td class="num" style={{ textAlign: 'right', color: h.PnLBase >= 0 ? 'var(--pos)' : 'var(--neg)' }}>
                  {privacy ? <span class="masked">{fmtMoney(h.PnLBase, currency, { sign: true })}</span> : fmtMoney(h.PnLBase, currency, { sign: true })}
                </td>
                <td class="num" style={{ textAlign: 'right', color: h.PnLBase >= 0 ? 'var(--pos)' : 'var(--neg)' }}>
                  {fmtPct(h.PnLPctBase)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
