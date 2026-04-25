import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/preact';

// Stub Chart.js: we only care that PerformanceChart calls the
// constructor at the right times, with the right data shape, and
// destroys the instance on prop change / unmount. The mock factory
// is hoisted, so the instance store is hoisted too via vi.hoisted.
const { ChartCtor, instances } = vi.hoisted(() => {
  const instances = [];
  const ChartCtor = vi.fn(function ChartMock(ctx, config) {
    this.ctx = ctx;
    this.config = config;
    this.data = config.data;
    this.options = config.options;
    this.destroy = vi.fn();
    this.update = vi.fn();
    instances.push(this);
  });
  ChartCtor.register = vi.fn();
  return { ChartCtor, instances };
});

vi.mock('chart.js', () => ({
  Chart: ChartCtor,
  LineController: {}, LineElement: {}, PointElement: {},
  LinearScale: {}, CategoryScale: {}, Filler: {}, Tooltip: {},
}));

// happy-dom's canvas.getContext returns null; the build() path early-returns
// in that case. Force a stub so the production code path runs.
beforeEach(() => {
  instances.length = 0;
  ChartCtor.mockClear();
  HTMLCanvasElement.prototype.getContext = function () {
    return { id: 'ctx-stub' };
  };
});

import { PerformanceChart } from './Chart.jsx';

const SERIES = [
  { d: '2026-04-01', v: 1000 },
  { d: '2026-04-02', v: 1010 },
  { d: '2026-04-03', v: 1025 },
];

describe('PerformanceChart', () => {
  it('renders the empty state when series has < 2 points', () => {
    const { container } = render(<PerformanceChart series={[]} currency="USD" />);
    expect(container.querySelector('.empty')).not.toBeNull();
    expect(container.querySelector('canvas')).toBeNull();
    expect(ChartCtor).not.toHaveBeenCalled();
  });

  it('renders the empty state for a single point too', () => {
    const { container } = render(
      <PerformanceChart series={[{ d: '2026-04-01', v: 1 }]} currency="USD" />,
    );
    expect(container.querySelector('.empty')).not.toBeNull();
    expect(ChartCtor).not.toHaveBeenCalled();
  });

  it('instantiates Chart.js once with labels + values from series', () => {
    const { container } = render(
      <PerformanceChart series={SERIES} currency="USD" />,
    );
    expect(container.querySelector('canvas')).not.toBeNull();
    expect(ChartCtor).toHaveBeenCalledOnce();
    const inst = instances[0];
    expect(inst.config.type).toBe('line');
    expect(inst.data.labels).toEqual(['2026-04-01', '2026-04-02', '2026-04-03']);
    expect(inst.data.datasets[0].data).toEqual([1000, 1010, 1025]);
  });

  it('destroys + rebuilds when series changes', () => {
    const { rerender } = render(
      <PerformanceChart series={SERIES} currency="USD" />,
    );
    expect(ChartCtor).toHaveBeenCalledTimes(1);
    const first = instances[0];

    rerender(
      <PerformanceChart series={[...SERIES, { d: '2026-04-04', v: 1040 }]} currency="USD" />,
    );
    expect(first.destroy).toHaveBeenCalledOnce();
    expect(ChartCtor).toHaveBeenCalledTimes(2);
    expect(instances[1].data.labels).toHaveLength(4);
  });

  it('destroys + rebuilds when currency changes', () => {
    const { rerender } = render(
      <PerformanceChart series={SERIES} currency="USD" />,
    );
    rerender(<PerformanceChart series={SERIES} currency="EUR" />);
    expect(instances[0].destroy).toHaveBeenCalledOnce();
    expect(ChartCtor).toHaveBeenCalledTimes(2);
  });

  it('destroys the chart on unmount', () => {
    const { unmount } = render(
      <PerformanceChart series={SERIES} currency="USD" />,
    );
    const inst = instances[0];
    unmount();
    expect(inst.destroy).toHaveBeenCalled();
  });

  it('tooltip label callback masks values when privacy is on', () => {
    render(<PerformanceChart series={SERIES} currency="USD" privacy />);
    const cb = instances[0].options.plugins.tooltip.callbacks.label;
    expect(cb({ parsed: { y: 1234 } })).toBe('••••••');
  });

  it('tooltip label callback formats money in the active currency when privacy is off', () => {
    render(<PerformanceChart series={SERIES} currency="EUR" />);
    const cb = instances[0].options.plugins.tooltip.callbacks.label;
    // fmtMoney(en-US, EUR) → '€1,234.00'.
    expect(cb({ parsed: { y: 1234 } })).toBe('€1,234.00');
  });

  it('tooltip title callback returns "" when items is empty', () => {
    render(<PerformanceChart series={SERIES} currency="USD" />);
    const cb = instances[0].options.plugins.tooltip.callbacks.title;
    expect(cb([])).toBe('');
    expect(cb([{ label: '2026-04-01' }])).toMatch(/2026/);
  });
});
