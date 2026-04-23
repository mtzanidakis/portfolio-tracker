import { useEffect, useRef } from 'preact/hooks';
import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Filler,
  Tooltip,
} from 'chart.js';
import { fmtDate, fmtMoney } from '../format.js';

// Tree-shaken Chart.js registration — pulls in ~35 kB gzipped vs. ~65 kB
// for chart.js/auto. Mutation observer re-themes the chart when the user
// toggles aesthetic/theme so canvas colors don't lag behind CSS vars.
Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip);

// cssVar reads a computed custom property from :root, falling back when
// SSR or early renders haven't wired the stylesheet yet.
function cssVar(name, fallback) {
  if (typeof window === 'undefined') return fallback;
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

// withAlpha turns a #rrggbb or rgb(…) into the same colour at the given
// alpha in 0..1. Keeps the gradient in the chart-line hue regardless of
// how the theme defines --chart-line.
function withAlpha(color, alpha) {
  const c = color.trim();
  if (c.startsWith('#')) {
    const hex = c.slice(1);
    const full = hex.length === 3
      ? hex.split('').map(ch => ch + ch).join('')
      : hex;
    const r = parseInt(full.slice(0, 2), 16);
    const g = parseInt(full.slice(2, 4), 16);
    const b = parseInt(full.slice(4, 6), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  const match = c.match(/rgba?\(([^)]+)\)/);
  if (match) {
    const parts = match[1].split(',').map(s => s.trim());
    return `rgba(${parts[0]}, ${parts[1]}, ${parts[2]}, ${alpha})`;
  }
  return c;
}

export function PerformanceChart({ series, privacy, currency }) {
  const wrapRef = useRef(null);
  const canvasRef = useRef(null);
  const chartRef = useRef(null);

  // Colour helpers are captured fresh inside build/restyle so theme
  // switches propagate without having to destroy the chart.
  const build = () => {
    if (!canvasRef.current) return null;
    const ctx = canvasRef.current.getContext('2d');
    if (!ctx) return null;

    const labels = (series || []).map(p => p.d);
    const data = (series || []).map(p => p.v);

    const chart = new Chart(ctx, {
      type: 'line',
      data: {
        labels,
        datasets: [{
          data,
          borderColor: (c) => cssVar('--chart-line', '#60a5fa'),
          borderWidth: 2,
          // Gradient is scriptable so it re-evaluates on resize (chartArea
          // changes) and on theme change (line colour changes).
          backgroundColor: (c) => {
            const area = c.chart.chartArea;
            if (!area) return 'transparent';
            const line = cssVar('--chart-line', '#60a5fa');
            const g = c.chart.ctx.createLinearGradient(0, area.top, 0, area.bottom);
            g.addColorStop(0, withAlpha(line, 0.18));
            g.addColorStop(1, withAlpha(line, 0));
            return g;
          },
          fill: true,
          tension: 0.35,
          cubicInterpolationMode: 'monotone',
          pointRadius: 0,
          pointHoverRadius: 4,
          pointHoverBorderWidth: 2,
          pointHoverBorderColor: cssVar('--chart-line', '#60a5fa'),
          pointHoverBackgroundColor: cssVar('--bg-elev', '#121821'),
          pointHitRadius: 10,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        interaction: { mode: 'index', intersect: false },
        layout: { padding: { top: 8, right: 18, bottom: 0, left: 4 } },
        scales: {
          x: {
            type: 'category',
            grid: { display: false },
            border: { display: false },
            ticks: {
              autoSkip: true,
              maxTicksLimit: 6,
              maxRotation: 0,
              align: 'center',
              padding: 8,
              color: cssVar('--text-faint', '#64748b'),
              font: { family: cssVar('--font-mono', 'ui-monospace, monospace'), size: 11 },
              callback(val) {
                const lbl = this.getLabelForValue(val);
                return fmtDate(lbl, { month: 'short', day: 'numeric' });
              },
            },
          },
          y: {
            display: false,
            grace: '8%',
          },
        },
        plugins: {
          legend: { display: false },
          tooltip: {
            enabled: true,
            backgroundColor: cssVar('--bg-elev', '#121821'),
            borderColor: cssVar('--border-strong', '#2c3849'),
            borderWidth: 1,
            titleColor: cssVar('--text-muted', '#94a3b8'),
            bodyColor: cssVar('--text', '#e6edf6'),
            titleFont: { family: cssVar('--font-sans', 'system-ui'), size: 11, weight: '400' },
            bodyFont: { family: cssVar('--font-mono', 'ui-monospace, monospace'), size: 13, weight: '500' },
            padding: { x: 12, y: 8 },
            displayColors: false,
            cornerRadius: 6,
            callbacks: {
              title: (items) => items.length
                ? fmtDate(items[0].label, { month: 'short', day: 'numeric', year: 'numeric' })
                : '',
              label: (item) => privacy
                ? '••••••'
                : fmtMoney(item.parsed.y, currency),
            },
          },
        },
      },
    });
    return chart;
  };

  // (Re)build on series / currency / privacy changes.
  useEffect(() => {
    chartRef.current?.destroy();
    chartRef.current = build();
    return () => {
      chartRef.current?.destroy();
      chartRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [series, currency, privacy]);

  // Re-theme on aesthetic/theme attribute flips — the user can toggle
  // dark/light from the tweaks panel and we want the canvas to follow.
  useEffect(() => {
    const obs = new MutationObserver(() => {
      const chart = chartRef.current;
      if (!chart) return;
      const line = cssVar('--chart-line', '#60a5fa');
      const ds = chart.data.datasets[0];
      ds.pointHoverBorderColor = line;
      ds.pointHoverBackgroundColor = cssVar('--bg-elev', '#121821');
      chart.options.scales.x.ticks.color = cssVar('--text-faint', '#64748b');
      chart.options.plugins.tooltip.backgroundColor = cssVar('--bg-elev', '#121821');
      chart.options.plugins.tooltip.borderColor = cssVar('--border-strong', '#2c3849');
      chart.options.plugins.tooltip.titleColor = cssVar('--text-muted', '#94a3b8');
      chart.options.plugins.tooltip.bodyColor = cssVar('--text', '#e6edf6');
      chart.update('none');
    });
    obs.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme', 'data-aesthetic'],
    });
    return () => obs.disconnect();
  }, []);

  if (!series || series.length < 2) {
    return (
      <div class="chart-wrap" ref={wrapRef}>
        <div class="empty" style={{ padding: 24 }}>No performance data yet.</div>
      </div>
    );
  }
  return (
    <div class="chart-wrap" ref={wrapRef} style={{ height: 320 }}>
      <canvas ref={canvasRef}></canvas>
    </div>
  );
}
