import { useState, useEffect } from 'preact/hooks';
import { fmtMoney } from '../format.js';
import { api } from '../api.js';

const PALETTE = ['#c8502a', '#d4953d', '#7a8c6f', '#a8572e', '#b8632e', '#e8876a', '#c9a87c', '#b8a68b'];

export function AllocationsPage({ privacy, currency }) {
  const [view, setView] = useState('asset');
  const [hover, setHover] = useState(null);
  const [data, setData] = useState(null);
  const [err, setErr] = useState(null);

  useEffect(() => {
    setErr(null);
    setData(null);
    api.allocations(view).then(setData).catch(e => setErr(e.message));
  }, [view]);

  if (err) return <div class="empty">Error: {err}</div>;
  if (!data) return <div class="empty">Loading…</div>;

  const groups = [...data].sort((a, b) => b.value - a.value).map((g, i) => ({
    ...g,
    color: PALETTE[i % PALETTE.length],
  }));
  const total = groups.reduce((s, g) => s + g.value, 0);

  const size = 280, cx = size / 2, cy = size / 2, r = 110, rInner = 80;
  let cursor = -Math.PI / 2;
  const arcs = groups.map(g => {
    const frac = total > 0 ? g.value / total : 0;
    const start = cursor, end = cursor + frac * Math.PI * 2;
    cursor = end;
    const large = end - start > Math.PI ? 1 : 0;
    const x1 = cx + r * Math.cos(start), y1 = cy + r * Math.sin(start);
    const x2 = cx + r * Math.cos(end),   y2 = cy + r * Math.sin(end);
    const x3 = cx + rInner * Math.cos(end), y3 = cy + rInner * Math.sin(end);
    const x4 = cx + rInner * Math.cos(start), y4 = cy + rInner * Math.sin(start);
    const d = `M ${x1} ${y1} A ${r} ${r} 0 ${large} 1 ${x2} ${y2} L ${x3} ${y3} A ${rInner} ${rInner} 0 ${large} 0 ${x4} ${y4} Z`;
    return { ...g, d, frac };
  });

  const focused = hover ? arcs.find(a => a.key === hover) : null;

  return (
    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Portfolio allocation</div>
          <div style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 2 }}>
            {groups.length} {view === 'asset' ? 'assets' : view === 'type' ? 'types' : 'accounts'} · total{' '}
            {privacy ? <span class="masked">{fmtMoney(total, currency)}</span> : <span class="mono">{fmtMoney(total, currency)}</span>}
          </div>
        </div>
        <div class="timeframe">
          <button class={view === 'asset' ? 'active' : ''} onClick={() => setView('asset')}>By asset</button>
          <button class={view === 'type' ? 'active' : ''} onClick={() => setView('type')}>By type</button>
          <button class={view === 'account' ? 'active' : ''} onClick={() => setView('account')}>By account</button>
        </div>
      </div>

      {groups.length === 0 && <div class="empty" style={{ padding: 24 }}>No holdings yet.</div>}

      {groups.length > 0 && (
        <div class="alloc-grid">
          <div class="donut-wrap">
            <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
              {arcs.map(a => (
                <path key={a.key} d={a.d} fill={a.color}
                  stroke="var(--bg-elev)" strokeWidth="2"
                  style={{
                    opacity: hover && hover !== a.key ? 0.35 : 1,
                    transition: 'opacity 150ms, transform 150ms',
                    transformOrigin: `${cx}px ${cy}px`,
                    transform: hover === a.key ? 'scale(1.03)' : 'scale(1)',
                    cursor: 'pointer',
                  }}
                  onMouseEnter={() => setHover(a.key)}
                  onMouseLeave={() => setHover(null)} />
              ))}
            </svg>
            <div class="donut-center">
              <div class="l">{focused ? focused.label : 'Total'}</div>
              <div class="v">
                {privacy
                  ? <span class="masked">{fmtMoney(focused ? focused.value : total, currency)}</span>
                  : fmtMoney(focused ? focused.value : total, currency)}
              </div>
              <div style={{ fontSize: 12, color: 'var(--text-faint)', fontFamily: 'var(--font-mono)', marginTop: 2 }}>
                {focused ? (focused.frac * 100).toFixed(1) + '%' : '100%'}
              </div>
            </div>
          </div>

          <ul class="alloc-list">
            {arcs.map(a => (
              <li key={a.key} class="alloc-row"
                onMouseEnter={() => setHover(a.key)}
                onMouseLeave={() => setHover(null)}
                style={{ background: hover === a.key ? 'var(--bg-hover)' : undefined }}>
                <span class="alloc-dot" style={{ background: a.color }} />
                <div class="alloc-name">
                  <div>{a.label}</div>
                  <div class="sub">{a.sub}</div>
                </div>
                <div class="alloc-bar">
                  <div class="fill" style={{ width: `${a.frac * 100}%`, background: a.color }} />
                </div>
                <div class="alloc-pct">
                  {(a.frac * 100).toFixed(1)}%<br />
                  <span style={{ fontSize: 11, color: 'var(--text-faint)' }}>
                    {privacy ? <span class="masked">{fmtMoney(a.value, currency, { decimals: 0 })}</span> : fmtMoney(a.value, currency, { decimals: 0 })}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
