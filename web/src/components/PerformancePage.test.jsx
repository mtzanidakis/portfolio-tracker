import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

// PerformanceChart pulls in chart.js; stub it out so this file tests
// PerformancePage's own logic (timeframe, refresh, hero, top movers).
vi.mock('./Chart.jsx', () => ({
  PerformanceChart: ({ series }) => (
    <div data-testid="perf-chart" data-points={series?.length ?? 0} />
  ),
}));

vi.mock('../api.js', () => ({
  api: {
    performance:   vi.fn(),
    holdings:      vi.fn(),
    assets:        vi.fn(),
    refreshPrices: vi.fn(),
  },
}));

import { api } from '../api.js';
import { PerformancePage } from './PerformancePage.jsx';

const PERF = {
  total: 12000, cost: 10000, pnl: 2000, pnl_pct: 20,
  unrealized: 1500, realized: 500,
  series: [
    { at: '2026-01-01', value: 10000, cost: 10000 },
    { at: '2026-04-01', value: 12000, cost: 10000 },
  ],
};
const HOLDINGS = [
  { Symbol: 'AAPL', Currency: 'USD', Qty: 5,
    ValueBase: 1000, CostBase: 800, PnLBase: 200, PnLPctBase: 25, PriceStale: false },
  { Symbol: 'MSFT', Currency: 'USD', Qty: 2,
    ValueBase: 500, CostBase: 600, PnLBase: -100, PnLPctBase: -16.6, PriceStale: false },
  // Stale row: should still render with a warning marker.
  { Symbol: 'OBSC', Currency: 'USD', Qty: 1,
    ValueBase: 50, CostBase: 50, PnLBase: 0, PnLPctBase: 0, PriceStale: true },
  // Sold-out row (Qty=0): should be filtered out of the movers list.
  { Symbol: 'GONE', Currency: 'USD', Qty: 0,
    ValueBase: 0, CostBase: 0, PnLBase: 0, PnLPctBase: 0, PriceStale: false },
];
const ASSETS = [
  { symbol: 'AAPL', name: 'Apple Inc.', type: 'stock', currency: 'USD' },
  { symbol: 'MSFT', name: 'Microsoft',  type: 'stock', currency: 'USD' },
  { symbol: 'OBSC', name: 'Obscure Co', type: 'stock', currency: 'USD' },
];

beforeEach(() => {
  vi.clearAllMocks();
  api.performance.mockResolvedValue(PERF);
  api.holdings.mockResolvedValue(HOLDINGS);
  api.assets.mockResolvedValue(ASSETS);
  api.refreshPrices.mockResolvedValue(null);
});

describe('PerformancePage — load + states', () => {
  it('shows Loading… until performance resolves', () => {
    let resolve;
    api.performance.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
    render(<PerformancePage privacy={false} currency="USD" />);
    expect(screen.getByText('Loading…')).toBeInTheDocument();
    resolve(PERF);
  });

  it('shows Error: <msg> when api.performance rejects', async () => {
    api.performance.mockRejectedValueOnce(new Error('server fault'));
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText(/Error: server fault/)).toBeInTheDocument());
  });

  it('mounts with tf=6M and fetches all three sources', async () => {
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => {
      expect(api.performance).toHaveBeenCalledWith('6M');
      expect(api.holdings).toHaveBeenCalled();
      expect(api.assets).toHaveBeenCalled();
    });
  });
});

describe('PerformancePage — hero + period stats', () => {
  it('renders total / pnl / pct in the hero', async () => {
    const { container } = render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText('$12,000.00')).toBeInTheDocument());
    // The hero-delta div mixes an icon, the formatted PnL, " · ", and the
    // percentage in one block. Match against its textContent rather
    // than fishing for an isolated "+$2,000.00".
    const delta = container.querySelector('.hero-delta');
    expect(delta).not.toBeNull();
    expect(delta.textContent).toMatch(/\+\$2,000\.00/);
    expect(delta.textContent).toMatch(/\+20\.00%/);
    // Cost basis row.
    expect(screen.getByText('$10,000.00')).toBeInTheDocument();
  });

  it('renders the unrealized / realized split when either is non-zero', async () => {
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText(/Unrealized/)).toBeInTheDocument());
    expect(screen.getByText(/Realized/)).toBeInTheDocument();
  });

  it('shows the stale-prices banner when any holding has PriceStale=true', async () => {
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() =>
      expect(screen.getByText(/Some prices are unavailable/)).toBeInTheDocument(),
    );
  });

  it('Today stat is the PnL delta between the last two series points, not the value delta', async () => {
    // Yesterday: value 10000, cost  9000  → PnL 1000.
    // Today:     value 11000, cost 10000  → PnL 1000.
    // Raw value delta is +1000 (capital deployed), but PnL is unchanged
    // → the Today stat must read $0.00, not +$1,000.00.
    api.performance.mockResolvedValueOnce({
      ...PERF,
      series: [
        { at: '2026-04-01', value: 10000, cost:  9000 },
        { at: '2026-04-02', value: 11000, cost: 10000 },
      ],
    });
    const { container } = render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText('Today')).toBeInTheDocument());
    const todayStat = [...container.querySelectorAll('.stat')]
      .find((n) => n.textContent.startsWith('Today'));
    expect(todayStat).toBeDefined();
    const todayValue = todayStat.querySelector('.stat-value');
    expect(todayValue.textContent.trim()).toBe('$0.00');
  });
});

describe('PerformancePage — timeframe + chart', () => {
  it('clicking a timeframe button refetches with the new tf', async () => {
    const user = userEvent.setup();
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(api.performance).toHaveBeenCalledWith('6M'));

    api.performance.mockResolvedValueOnce({ ...PERF, total: 5000 });
    await user.click(screen.getByRole('button', { name: '1Y' }));
    await waitFor(() => expect(api.performance).toHaveBeenCalledWith('1Y'));
  });

  it('passes the series length down to PerformanceChart', async () => {
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => {
      const chart = screen.getByTestId('perf-chart');
      expect(chart.getAttribute('data-points')).toBe('2');
    });
  });
});

describe('PerformancePage — refresh', () => {
  it('Refresh prices calls api.refreshPrices then reloads everything', async () => {
    const user = userEvent.setup();
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(api.performance).toHaveBeenCalledOnce());
    api.performance.mockClear();
    api.holdings.mockClear();
    api.assets.mockClear();

    await user.click(screen.getByRole('button', { name: /refresh prices/i }));
    await waitFor(() => expect(api.refreshPrices).toHaveBeenCalledOnce());
    await waitFor(() => {
      expect(api.performance).toHaveBeenCalled();
      expect(api.holdings).toHaveBeenCalled();
      expect(api.assets).toHaveBeenCalled();
    });
  });

  it('Refresh-button label flips to "Refreshing…" while in flight', async () => {
    let resolve;
    api.refreshPrices.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
    const user = userEvent.setup();
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText(/Refresh prices/)).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /refresh prices/i }));
    await waitFor(() => expect(screen.getByText('Refreshing…')).toBeInTheDocument());
    resolve(null);
  });

  it('renders the API error if api.refreshPrices rejects', async () => {
    api.refreshPrices.mockRejectedValueOnce(new Error('rate limited'));
    const user = userEvent.setup();
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText(/Refresh prices/)).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /refresh prices/i }));
    await waitFor(() => expect(screen.getByText(/Error: rate limited/)).toBeInTheDocument());
  });
});

describe('PerformancePage — top movers', () => {
  it('orders holdings by PnLPctBase descending and excludes Qty=0', async () => {
    const { container } = render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    const symCells = container.querySelectorAll('.ticker-sym');
    // AAPL +25 → top, OBSC 0 next, MSFT -16.6 last; GONE filtered out.
    const tickerOrder = [...symCells].slice(-3).map((n) => n.textContent.trim());
    expect(tickerOrder).toEqual(['AAPL', 'OBSC', 'MSFT']);
  });

  it('marks stale rows with a ⚠ symbol next to the value', async () => {
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText('Obscure Co')).toBeInTheDocument());
    expect(screen.getByText('⚠')).toBeInTheDocument();
  });

  it('shows "No holdings yet" when the holdings list is empty', async () => {
    api.holdings.mockResolvedValueOnce([]);
    render(<PerformancePage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText(/no holdings yet/i)).toBeInTheDocument());
  });
});
