import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: { allocations: vi.fn() },
}));

import { api } from '../api.js';
import { AllocationsPage } from './AllocationsPage.jsx';

const ASSET_GROUPS = [
  { key: 'AAPL', label: 'AAPL', sub: 'Apple Inc.', value: 6000 },
  { key: 'BTC',  label: 'BTC',  sub: 'Bitcoin',    value: 3000 },
  { key: 'CASH', label: 'Cash', sub: 'EUR',        value: 1000 },
];

beforeEach(() => {
  vi.clearAllMocks();
  api.allocations.mockResolvedValue(ASSET_GROUPS);
});

describe('AllocationsPage — load + state', () => {
  it('shows Loading… until allocations resolves', () => {
    let resolve;
    api.allocations.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
    render(<AllocationsPage privacy={false} currency="USD" />);
    expect(screen.getByText('Loading…')).toBeInTheDocument();
    resolve(ASSET_GROUPS);
  });

  it('renders an Error: <msg> empty state when the fetch fails', async () => {
    api.allocations.mockRejectedValueOnce(new Error('upstream down'));
    render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText(/Error: upstream down/)).toBeInTheDocument());
  });

  it('shows "No holdings yet" when the response is an empty list', async () => {
    api.allocations.mockResolvedValueOnce([]);
    render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(screen.getByText(/no holdings yet/i)).toBeInTheDocument());
  });
});

describe('AllocationsPage — view toggle', () => {
  it('mounts with view=asset and labels the count line accordingly', async () => {
    render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(api.allocations).toHaveBeenCalledWith('asset'));
    expect(screen.getByText(/3 assets/)).toBeInTheDocument();
  });

  it('clicking By type re-fetches with group=type and updates the count line', async () => {
    const user = userEvent.setup();
    render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(api.allocations).toHaveBeenCalledWith('asset'));

    api.allocations.mockResolvedValueOnce([
      { key: 'stock', label: 'Stocks', value: 6000 },
      { key: 'crypto', label: 'Crypto', value: 4000 },
    ]);
    await user.click(screen.getByRole('button', { name: /by type/i }));
    await waitFor(() => expect(api.allocations).toHaveBeenCalledWith('type'));
    await waitFor(() => expect(screen.getByText(/2 types/)).toBeInTheDocument());
  });

  it('clicking By account re-fetches with group=account', async () => {
    const user = userEvent.setup();
    render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(api.allocations).toHaveBeenCalledWith('asset'));

    api.allocations.mockResolvedValueOnce([{ key: '1', label: 'Brokerage', value: 1 }]);
    await user.click(screen.getByRole('button', { name: /by account/i }));
    await waitFor(() => expect(api.allocations).toHaveBeenCalledWith('account'));
    await waitFor(() => expect(screen.getByText(/1 accounts/)).toBeInTheDocument());
  });
});

describe('AllocationsPage — donut + list rendering', () => {
  it('renders one donut path and one list row per group, sorted by value desc', async () => {
    const { container } = render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(container.querySelector('svg path')).not.toBeNull());

    const paths = container.querySelectorAll('svg path');
    expect(paths).toHaveLength(3);

    const rows = container.querySelectorAll('.alloc-row');
    expect(rows).toHaveLength(3);
    // Sorted by value desc: AAPL (6000) → BTC (3000) → CASH (1000).
    expect(rows[0].textContent).toContain('AAPL');
    expect(rows[1].textContent).toContain('BTC');
    expect(rows[2].textContent).toContain('Cash');
  });

  it('shows the total in the donut centre and the rendered percentages', async () => {
    const { container } = render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(container.querySelector('.donut-center')).not.toBeNull());

    // Total = 6000 + 3000 + 1000 = 10000. Appears twice (header + centre).
    expect(screen.getAllByText('$10,000.00')).toHaveLength(2);
    // 100% on the centre when nothing is hovered.
    expect(screen.getByText('100%')).toBeInTheDocument();
    // 60% / 30% / 10% in the list rows (.pct cells).
    const pcts = [...container.querySelectorAll('.alloc-row .pct')].map((n) => n.textContent);
    expect(pcts).toEqual(['60.0%', '30.0%', '10.0%']);
  });

  it('hovering a list row swaps the donut centre to that group', async () => {
    const { container } = render(<AllocationsPage privacy={false} currency="USD" />);
    await waitFor(() => expect(container.querySelector('.alloc-row')).not.toBeNull());

    // Initial centre shows 100%.
    expect(container.querySelector('.donut-center .p').textContent).toBe('100%');

    const aaplRow = container.querySelectorAll('.alloc-row')[0];
    fireEvent.mouseEnter(aaplRow);
    await waitFor(() =>
      expect(container.querySelector('.donut-center .p').textContent).toBe('60.0%'),
    );
    expect(container.querySelector('.donut-center .l').textContent).toBe('AAPL');
  });

  it('wraps monetary values in .masked when privacy is on', async () => {
    const { container } = render(<AllocationsPage privacy currency="USD" />);
    await waitFor(() => expect(container.querySelector('.masked')).not.toBeNull());
    // Header total + centre value + each row's amount → multiple .masked spans.
    expect(container.querySelectorAll('.masked').length).toBeGreaterThan(2);
  });
});
