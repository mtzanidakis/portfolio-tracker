import { useState, useEffect, useRef, useMemo } from 'preact/hooks';
import { fmtDate, fmtMoney, fmtPct } from '../format.js';

// Line chart with crosshair. series = [{d: Date|string, v: number}]
export function PerformanceChart({ series, privacy, currency }) {
  const wrapRef = useRef(null);
  const [hover, setHover] = useState(null);
  const [size, setSize] = useState({ w: 900, h: 280 });

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver(entries => {
      for (const e of entries) setSize({ w: Math.max(400, e.contentRect.width), h: 280 });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  if (!series || series.length < 2) {
    return <div class="chart-wrap" ref={wrapRef}>
      <div class="empty" style={{ padding: 24 }}>No performance data yet.</div>
    </div>;
  }

  const padding = { l: 0, r: 0, t: 18, b: 28 };
  const { w, h } = size;
  const innerW = w - padding.l - padding.r;
  const innerH = h - padding.t - padding.b;

  const values = series.map(s => s.v);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const yPad = range * 0.15;
  const yMin = min - yPad;
  const yMax = max + yPad;

  const x = i => padding.l + (i / (series.length - 1)) * innerW;
  const y = v => padding.t + innerH - ((v - yMin) / (yMax - yMin)) * innerH;

  const pathD = useMemo(
    () => series.map((s, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(2)} ${y(s.v).toFixed(2)}`).join(' '),
    [series, w, h]
  );
  const areaD = useMemo(() => {
    const top = series.map((s, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(2)} ${y(s.v).toFixed(2)}`).join(' ');
    return `${top} L ${x(series.length - 1).toFixed(2)} ${padding.t + innerH} L ${x(0).toFixed(2)} ${padding.t + innerH} Z`;
  }, [series, w, h]);

  const onMove = e => {
    const rect = e.currentTarget.getBoundingClientRect();
    const px = e.clientX - rect.left;
    const ratio = Math.max(0, Math.min(1, (px - padding.l) / innerW));
    const i = Math.round(ratio * (series.length - 1));
    setHover({ i, px: x(i), py: y(series[i].v) });
  };

  const gridLines = 5;
  const yTicks = [];
  for (let i = 0; i <= gridLines; i++) {
    const v = yMin + (i / gridLines) * (yMax - yMin);
    yTicks.push({ v, y: y(v) });
  }

  const xTickCount = 6;
  const xTicks = [];
  for (let i = 0; i < xTickCount; i++) {
    const idx = Math.round((i / (xTickCount - 1)) * (series.length - 1));
    xTicks.push({ i: idx, x: x(idx), d: series[idx].d });
  }

  const firstV = series[0].v;
  const hoverPoint = hover ? series[hover.i] : null;
  const hoverDelta = hoverPoint ? hoverPoint.v - firstV : 0;
  const hoverDeltaPct = hoverPoint ? (hoverDelta / firstV) * 100 : 0;

  return (
    <div class="chart-wrap" ref={wrapRef}>
      <svg class="chart-svg"
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}>
        <defs>
          <linearGradient id="area-fill" x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="var(--terra)" stopOpacity="0.18" />
            <stop offset="100%" stopColor="var(--terra)" stopOpacity="0" />
          </linearGradient>
        </defs>

        {yTicks.map((t, i) => (
          <line key={i} x1={padding.l} x2={w - padding.r} y1={t.y} y2={t.y}
            stroke="var(--chart-grid)" strokeWidth="1"
            strokeDasharray={i === 0 || i === gridLines ? '0' : '2 4'}
            opacity={i === 0 || i === gridLines ? 1 : 0.8} />
        ))}

        {xTicks.map((t, i) => (
          <text key={i} x={t.x} y={h - 8}
            fill="var(--text-faint)" fontSize="10.5"
            textAnchor={i === 0 ? 'start' : i === xTicks.length - 1 ? 'end' : 'middle'}
            fontFamily="var(--font-mono)">
            {fmtDate(t.d)}
          </text>
        ))}

        <path d={areaD} fill="url(#area-fill)" />
        <path d={pathD} fill="none" stroke="var(--chart-line)"
          strokeWidth="1.6" strokeLinejoin="round" strokeLinecap="round" />

        {hover && (
          <>
            <line x1={hover.px} x2={hover.px} y1={padding.t} y2={padding.t + innerH}
              stroke="var(--terra)" strokeWidth="1" strokeDasharray="3 3" opacity="0.6" />
            <circle cx={hover.px} cy={hover.py} r="5"
              fill="var(--bg-elev)" stroke="var(--terra)" strokeWidth="2" />
          </>
        )}
      </svg>

      {hover && hoverPoint && (
        <div class="chart-tooltip"
          style={{ left: `${(hover.px / w) * 100}%`, top: `${((hover.py - 12) / h) * 100}%` }}>
          <div class="date">{fmtDate(hoverPoint.d, { month: 'short', day: 'numeric', year: 'numeric' })}</div>
          <div class="val">
            {privacy ? <span class="masked">{fmtMoney(hoverPoint.v, currency)}</span> : fmtMoney(hoverPoint.v, currency)}
          </div>
          <div style={{ fontSize: 11, color: hoverDelta >= 0 ? 'var(--pos)' : 'var(--neg)', fontFamily: 'var(--font-mono)' }}>
            {fmtPct(hoverDeltaPct)} ({fmtMoney(hoverDelta, currency, { sign: true })})
          </div>
        </div>
      )}
    </div>
  );
}
