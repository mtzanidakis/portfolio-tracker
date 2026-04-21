import { useState, useEffect, useRef, useMemo } from 'preact/hooks';
import { fmtDate, fmtMoney, fmtPct } from '../format.js';

// Monotone-cubic interpolation through a list of {x,y} points. Produces
// a smooth line that never overshoots its data — unlike Catmull-Rom this
// guarantees the curve stays monotonic between samples, which matches
// how financial series are expected to be read.
function monotoneCubic(points) {
  const n = points.length;
  if (n < 2) return '';
  if (n === 2) {
    return `M ${points[0].x} ${points[0].y} L ${points[1].x} ${points[1].y}`;
  }

  const dx = new Array(n - 1);
  const slopes = new Array(n - 1);
  for (let i = 0; i < n - 1; i++) {
    dx[i] = points[i + 1].x - points[i].x;
    slopes[i] = (points[i + 1].y - points[i].y) / dx[i];
  }

  // Fritsch–Carlson tangents.
  const tangents = new Array(n);
  tangents[0] = slopes[0];
  tangents[n - 1] = slopes[n - 2];
  for (let i = 1; i < n - 1; i++) {
    if (slopes[i - 1] * slopes[i] <= 0) {
      tangents[i] = 0;
    } else {
      const w1 = 2 * dx[i] + dx[i - 1];
      const w2 = dx[i] + 2 * dx[i - 1];
      tangents[i] = (w1 + w2) / (w1 / slopes[i - 1] + w2 / slopes[i]);
    }
  }

  let d = `M ${points[0].x} ${points[0].y}`;
  for (let i = 0; i < n - 1; i++) {
    const c1x = points[i].x + dx[i] / 3;
    const c1y = points[i].y + (tangents[i] * dx[i]) / 3;
    const c2x = points[i + 1].x - dx[i] / 3;
    const c2y = points[i + 1].y - (tangents[i + 1] * dx[i]) / 3;
    d += ` C ${c1x.toFixed(2)} ${c1y.toFixed(2)}, ${c2x.toFixed(2)} ${c2y.toFixed(2)}, ${points[i + 1].x.toFixed(2)} ${points[i + 1].y.toFixed(2)}`;
  }
  return d;
}

// Pick up to `max` evenly-spaced tick indices from a series of length n.
// Always includes the first and last index so the chart starts and ends
// with visible labels (matches the mockup).
function pickTickIndices(n, max) {
  if (n <= max) return [...Array(n).keys()];
  const out = new Set();
  for (let i = 0; i < max; i++) {
    out.add(Math.round((i / (max - 1)) * (n - 1)));
  }
  return [...out].sort((a, b) => a - b);
}

// Line chart with crosshair. series = [{d: Date|string, v: number}]
export function PerformanceChart({ series, privacy, currency }) {
  const wrapRef = useRef(null);
  const [hover, setHover] = useState(null);
  const [size, setSize] = useState({ w: 900, h: 320 });

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver(entries => {
      for (const e of entries) setSize({ w: Math.max(400, e.contentRect.width), h: 320 });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  if (!series || series.length < 2) {
    return <div class="chart-wrap" ref={wrapRef}>
      <div class="empty" style={{ padding: 24 }}>No performance data yet.</div>
    </div>;
  }

  // Right padding is generous so the final x-axis label ("Apr 21") isn't
  // clipped; top padding leaves a quiet strip above the curve.
  const padding = { l: 12, r: 28, t: 18, b: 32 };
  const { w, h } = size;
  const innerW = w - padding.l - padding.r;
  const innerH = h - padding.t - padding.b;

  const values = series.map(s => s.v);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const yPad = range * 0.12;
  const yMin = min - yPad;
  const yMax = max + yPad;

  const x = i => padding.l + (i / (series.length - 1)) * innerW;
  const y = v => padding.t + innerH - ((v - yMin) / (yMax - yMin)) * innerH;

  const pts = useMemo(
    () => series.map((s, i) => ({ x: x(i), y: y(s.v) })),
    [series, w, h]
  );

  const pathD = useMemo(() => monotoneCubic(pts), [pts]);
  const areaD = useMemo(() => {
    if (!pathD) return '';
    const last = pts[pts.length - 1];
    const first = pts[0];
    const baseY = padding.t + innerH;
    return `${pathD} L ${last.x.toFixed(2)} ${baseY} L ${first.x.toFixed(2)} ${baseY} Z`;
  }, [pathD, pts, padding.t, innerH]);

  const onMove = e => {
    const rect = e.currentTarget.getBoundingClientRect();
    const px = e.clientX - rect.left;
    const ratio = Math.max(0, Math.min(1, (px - padding.l) / innerW));
    const i = Math.round(ratio * (series.length - 1));
    setHover({ i, px: x(i), py: y(series[i].v) });
  };

  const xTickIdxs = pickTickIndices(series.length, 6);
  const xTicks = xTickIdxs.map(i => ({ i, x: x(i), d: series[i].d }));

  const firstV = series[0].v;
  const hoverPoint = hover ? series[hover.i] : null;
  const hoverDelta = hoverPoint ? hoverPoint.v - firstV : 0;
  const hoverDeltaPct = hoverPoint && firstV !== 0 ? (hoverDelta / firstV) * 100 : 0;

  return (
    <div class="chart-wrap" ref={wrapRef}>
      <svg class="chart-svg"
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}>
        <defs>
          <linearGradient id="area-fill" x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="var(--chart-line)" stopOpacity="0.12" />
            <stop offset="100%" stopColor="var(--chart-line)" stopOpacity="0" />
          </linearGradient>
        </defs>

        <path d={areaD} fill="url(#area-fill)" />
        <path d={pathD} fill="none" stroke="var(--chart-line)"
          strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" />

        {xTicks.map((t, i) => (
          <text key={i} x={t.x} y={h - 8}
            fill="var(--text-faint)" fontSize="11"
            textAnchor={i === 0 ? 'start' : i === xTicks.length - 1 ? 'end' : 'middle'}
            fontFamily="var(--font-mono)">
            {fmtDate(t.d)}
          </text>
        ))}

        {hover && (
          <>
            <line x1={hover.px} x2={hover.px} y1={padding.t} y2={padding.t + innerH}
              stroke="var(--chart-line)" strokeWidth="1" strokeDasharray="3 3" opacity="0.5" />
            <circle cx={hover.px} cy={hover.py} r="5"
              fill="var(--bg-elev)" stroke="var(--chart-line)" strokeWidth="2" />
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
