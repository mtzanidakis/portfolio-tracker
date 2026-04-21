import { useState, useEffect } from 'preact/hooks';
import { fmtMoney } from '../format.js';
import { api } from '../api.js';

const PALETTE = ['#c8502a', '#d4953d', '#7a8c6f', '#a8572e', '#b8632e', '#e8876a', '#c9a87c', '#b8a68b'];

// Small angular gap between donut arcs, in radians. Matches the mockup
// where each slice has a visible break rather than meeting its neighbor.
const ARC_GAP = 0.035;

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

  const size = 300, cx = size / 2, cy = size / 2, r = 122, rInner = 86;
  let cursor = -Math.PI / 2;
  const arcs = groups.map(g => {
    const frac = total > 0 ? g.value / total : 0;
    const sweep = frac * Math.PI * 2;
    // Shrink each arc by ARC_GAP to leave a visible break between slices.
    // Skip the inset when the slice is tiny so it doesn't invert.
    const inset = sweep > ARC_GAP * 2 ? ARC_GAP / 2 : 0;
    const start = cursor + inset;
    const end = cursor + sweep - inset;
    cursor += sweep;

    const large = end - start > Math.PI ? 1 : 0;
    const x1 = cx + r * Math.cos(start),      y1 = cy + r * Math.sin(start);
    const x2 = cx + r * Math.cos(end),        y2 = cy + r * Math.sin(end);
    const x3 = cx + rInner * Math.cos(end),   y3 = cy + rInner * Math.sin(end);
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
                  style={{
                    opacity: hover && hover !== a.key ? 0.32 : 1,
                    transition: 'opacity 150ms, transform 150ms',
                    transformOrigin: `${cx}px ${cy}px`,
                    transform: hover === a.key ? 'scale(1.035)' : 'scale(1)',
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
              <div class="p">
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
                  <div class="sym">{a.label}</div>
                  {a.sub && <div class="sub">{a.sub}</div>}
                </div>
                <div class="alloc-bar">
                  <div class="fill"
                    style={{ width: `${a.frac * 100}%`, background: a.color }} />
                </div>
                <div class="alloc-meta">
                  <div class="pct">{(a.frac * 100).toFixed(1)}%</div>
                  <div class="amt">
                    {privacy
                      ? <span class="masked">{fmtMoney(a.value, currency, { decimals: 0 })}</span>
                      : fmtMoney(a.value, currency, { decimals: 0 })}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
