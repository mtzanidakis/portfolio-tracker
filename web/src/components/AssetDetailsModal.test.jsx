import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    assetPrice:   vi.fn(),
    transactions: vi.fn(),
  },
}));

import { api } from '../api.js';
import { AssetDetailsModal } from './AssetDetailsModal.jsx';

const ASSET = { symbol: 'AAPL', name: 'Apple Inc.', type: 'stock', currency: 'USD' };

beforeEach(() => {
  vi.clearAllMocks();
});

describe('AssetDetailsModal — load', () => {
  it('shows Loading… until both fetches resolve', () => {
    let resolveP;
    api.assetPrice.mockReturnValueOnce(new Promise((r) => { resolveP = r; }));
    api.transactions.mockResolvedValue([]);
    render(
      <AssetDetailsModal asset={ASSET} privacy={false}
        onClose={vi.fn()} onShowActivities={vi.fn()} />,
    );
    expect(screen.getByText('Loading…')).toBeInTheDocument();
    resolveP({ price: 100, stale: false });
  });

  it('renders the API error inline', async () => {
    api.assetPrice.mockRejectedValueOnce(new Error('boom'));
    api.transactions.mockResolvedValue([]);
    render(
      <AssetDetailsModal asset={ASSET} privacy={false}
        onClose={vi.fn()} onShowActivities={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument());
  });

  it('queries transactions filtered to this asset (percent-encodes the symbol)', async () => {
    api.assetPrice.mockResolvedValue({ price: 0, stale: true });
    api.transactions.mockResolvedValue([]);
    const exotic = { symbol: 'BTC-USD', name: 'Bitcoin USD', type: 'crypto', currency: 'USD' };
    render(
      <AssetDetailsModal asset={exotic} privacy={false}
        onClose={vi.fn()} onShowActivities={vi.fn()} />,
    );
    await waitFor(() => expect(api.transactions).toHaveBeenCalledWith('?symbol=BTC-USD'));
  });
});

describe('AssetDetailsModal — derivations', () => {
  // Two buys + one sell, no fees. Average cost basis after first two
  // buys: (100×10 + 110×10) / 20 = 105. Sell of 5 @ 130 → realized
  // (130 − 105) × 5 = 125. Open qty = 15, open cost = 105 × 15 = 1575.
  // Current price = 120 → current value = 1800, unrealized = 225.
  // Investment sum (buys only) = 1000 + 1100 = 2100.
  // pnl% against investment = (225 + 125) / 2100 = 16.666…%
  const TXS = [
    { id: 1, side: 'buy',  qty: 10, price: 100, fee: 0, occurred_at: '2026-01-15T12:00:00Z' },
    { id: 2, side: 'buy',  qty: 10, price: 110, fee: 0, occurred_at: '2026-02-15T12:00:00Z' },
    { id: 3, side: 'sell', qty: 5,  price: 130, fee: 0, occurred_at: '2026-03-15T12:00:00Z' },
  ];

  beforeEach(() => {
    api.assetPrice.mockResolvedValue({ price: 120, stale: false });
    api.transactions.mockResolvedValue(TXS);
  });

  it('renders current price, quantity, value, investment sum, and PnL', async () => {
    render(
      <AssetDetailsModal asset={ASSET} privacy={false}
        onClose={vi.fn()} onShowActivities={vi.fn()} />,
    );
    await waitFor(() => expect(screen.queryByText('Loading…')).not.toBeInTheDocument());

    expect(screen.getByText('$120.00')).toBeInTheDocument();           // current price
    expect(screen.getByText('15')).toBeInTheDocument();                // quantity
    expect(screen.getByText('$1,800.00')).toBeInTheDocument();         // current value
    expect(screen.getByText('$2,100.00')).toBeInTheDocument();         // investment sum
    expect(screen.getByText(/across 2 buys/)).toBeInTheDocument();
    expect(screen.getByText(/3$/)).toBeInTheDocument();                // total activities

    // Buy price range: min $100 — max $110.
    expect(screen.getByText(/\$100\.00.*\$110\.00/)).toBeInTheDocument();
    // Avg buy = (1000 + 1100) / 20 = 105.
    expect(screen.getByText('$105.00')).toBeInTheDocument();
  });

  it('marks the price unavailable when priceInfo.stale is true', async () => {
    api.assetPrice.mockResolvedValueOnce({ price: 0, stale: true });
    render(
      <AssetDetailsModal asset={ASSET} privacy={false}
        onClose={vi.fn()} onShowActivities={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('price unavailable')).toBeInTheDocument());
  });

  it('renders an em-dash for the buy-price range when there are no buys', async () => {
    api.transactions.mockResolvedValueOnce([]);
    render(
      <AssetDetailsModal asset={ASSET} privacy={false}
        onClose={vi.fn()} onShowActivities={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('no buys yet')).toBeInTheDocument());
  });

  it('wraps monetary values in .masked when privacy is on', async () => {
    const { container } = render(
      <AssetDetailsModal asset={ASSET} privacy
        onClose={vi.fn()} onShowActivities={vi.fn()} />,
    );
    await waitFor(() => expect(container.querySelector('.masked')).not.toBeNull());
    // Several monetary stats; expect more than 2 masked spans.
    expect(container.querySelectorAll('.masked').length).toBeGreaterThan(2);
  });
});

describe('AssetDetailsModal — actions', () => {
  beforeEach(() => {
    api.assetPrice.mockResolvedValue({ price: 100, stale: false });
    api.transactions.mockResolvedValue([]);
  });

  it('Close button calls onClose', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <AssetDetailsModal asset={ASSET} privacy={false}
        onClose={onClose} onShowActivities={vi.fn()} />,
    );
    await waitFor(() => expect(screen.queryByText('Loading…')).not.toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /^close$/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it('"Show activities →" calls onShowActivities then onClose', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onShowActivities = vi.fn();
    render(
      <AssetDetailsModal asset={ASSET} privacy={false}
        onClose={onClose} onShowActivities={onShowActivities} />,
    );
    await waitFor(() => expect(screen.queryByText('Loading…')).not.toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /show activities/i }));
    expect(onShowActivities).toHaveBeenCalledWith('AAPL');
    expect(onClose).toHaveBeenCalled();
  });
});
