import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    assets:           vi.fn(),
    accounts:         vi.fn(),
    txSummary:        vi.fn(),
    transactionsPage: vi.fn(),
    deleteTx:         vi.fn(),
  },
}));

// TxModal pulls in chart.js indirectly; stub it out so we don't have
// to load that dependency tree just for an edit-button test we don't
// even run.
vi.mock('./TxModal.jsx', () => ({
  TxModal: () => null,
}));

import { api } from '../api.js';
import { ActivitiesPage } from './ActivitiesPage.jsx';

const ACCOUNTS = [
  { id: 1, name: 'Brokerage', currency: 'USD' },
  { id: 2, name: 'Bank',      currency: 'EUR' },
];
const ASSETS = [
  { symbol: 'AAPL', name: 'Apple Inc.',  type: 'stock', currency: 'USD' },
  { symbol: 'BTC',  name: 'Bitcoin',     type: 'crypto', currency: 'USD' },
];
const SUMMARY = {
  count: 17, asset_count: 2, account_count: 2,
  total_buys: 1000, total_sells: 200, buy_count: 10, sell_count: 2,
  total_deposits: 0, total_withdraws: 0, total_interest: 0,
};
const PAGE_1 = [
  { id: 1, account_id: 1, asset_symbol: 'AAPL', side: 'buy',
    qty: 5, price: 198, fee: 1, fx_to_base: 1,
    occurred_at: '2026-04-01T12:00:00Z', note: '' },
];

beforeEach(() => {
  vi.clearAllMocks();
  api.assets.mockResolvedValue(ASSETS);
  api.accounts.mockResolvedValue(ACCOUNTS);
  api.txSummary.mockResolvedValue(SUMMARY);
  api.transactionsPage.mockResolvedValue({ items: PAGE_1, nextCursor: '' });
});

async function untilFirstFetch() {
  await waitFor(() => expect(api.transactionsPage).toHaveBeenCalled());
}

// Read the most recent transactionsPage call's params object.
function lastQuery() {
  const calls = api.transactionsPage.mock.calls;
  return calls[calls.length - 1][0];
}

describe('ActivitiesPage — initial load', () => {
  it('mounts and fires the lookup, summary, and first-page fetches', async () => {
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await waitFor(() => {
      expect(api.assets).toHaveBeenCalledOnce();
      expect(api.accounts).toHaveBeenCalledOnce();
      expect(api.txSummary).toHaveBeenCalledOnce();
      expect(api.transactionsPage).toHaveBeenCalled();
    });
  });

  it('passes initialAccountId / initialAssetSymbol through to the first fetch', async () => {
    render(
      <ActivitiesPage privacy={false} currency="USD" user={{}}
        initialAccountId={2} initialAssetSymbol="AAPL" />,
    );
    await untilFirstFetch();
    const q = lastQuery();
    expect(q.accountId).toBe(2);
    expect(q.symbol).toBe('AAPL');
    expect(q.side).toBe('');
    expect(q.cursor).toBe('');
    expect(q.limit).toBe(50);
  });

  it('renders the row from the first page (date + asset + total)', async () => {
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());
    // 5 × 198 = 990 USD.
    expect(screen.getByText('$990.00')).toBeInTheDocument();
  });

  it('shows the empty-state row when items is empty after load', async () => {
    api.transactionsPage.mockResolvedValueOnce({ items: [], nextCursor: '' });
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await waitFor(() =>
      expect(screen.getByText(/No transactions match your filter/i)).toBeInTheDocument(),
    );
  });
});

describe('ActivitiesPage — filter / sort / pagination', () => {
  it('clicking the Trades filter sends side=buy,sell', async () => {
    const user = userEvent.setup();
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await untilFirstFetch();
    api.transactionsPage.mockClear();

    await user.click(screen.getByRole('button', { name: 'Trades' }));
    await waitFor(() => expect(api.transactionsPage).toHaveBeenCalled());
    expect(lastQuery().side).toBe('buy,sell');
  });

  it('clicking the Cash filter sends side=deposit,withdraw,interest', async () => {
    const user = userEvent.setup();
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await untilFirstFetch();
    api.transactionsPage.mockClear();

    await user.click(screen.getByRole('button', { name: 'Cash' }));
    await waitFor(() => expect(api.transactionsPage).toHaveBeenCalled());
    expect(lastQuery().side).toBe('deposit,withdraw,interest');
  });

  it('clearing the asset pill drops the symbol filter', async () => {
    const user = userEvent.setup();
    render(
      <ActivitiesPage privacy={false} currency="USD" user={{}}
        initialAssetSymbol="AAPL" />,
    );
    await untilFirstFetch();
    api.transactionsPage.mockClear();

    await user.click(screen.getByRole('button', { name: /clear asset filter/i }));
    await waitFor(() => expect(api.transactionsPage).toHaveBeenCalled());
    expect(lastQuery().symbol).toBe('');
  });

  it('clicking the Date header twice toggles asc/desc on the same column', async () => {
    const user = userEvent.setup();
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await untilFirstFetch();
    api.transactionsPage.mockClear();

    // First click on already-sorted column flips to asc.
    await user.click(screen.getByText(/Date/));
    await waitFor(() => expect(lastQuery().order).toBe('asc'));

    // Second click flips back to desc.
    await user.click(screen.getByText(/Date/));
    await waitFor(() => expect(lastQuery().order).toBe('desc'));
  });

  it('clicking a non-active sort column resets to its default order', async () => {
    const user = userEvent.setup();
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await untilFirstFetch();
    api.transactionsPage.mockClear();

    // Quantity is not currently the sort column → default asc.
    await user.click(screen.getByText(/Quantity/));
    await waitFor(() => {
      const q = lastQuery();
      expect(q.sort).toBe('qty');
      expect(q.order).toBe('asc');
    });
  });

  it('Load more uses the cursor from the previous response', async () => {
    api.transactionsPage.mockResolvedValueOnce({ items: PAGE_1, nextCursor: 'cur-1' });
    const user = userEvent.setup();
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await waitFor(() => expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument());

    api.transactionsPage.mockResolvedValueOnce({
      items: [{ ...PAGE_1[0], id: 2 }], nextCursor: '',
    });
    await user.click(screen.getByRole('button', { name: /load more/i }));
    await waitFor(() => expect(lastQuery().cursor).toBe('cur-1'));
  });

  it('debounces the search input — only one fetch per debounce window', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
      await untilFirstFetch();
      api.transactionsPage.mockClear();

      const search = document.querySelector('input.search');
      await user.type(search, 'app');
      // Three keystrokes inside the debounce window → no extra fetches yet.
      expect(api.transactionsPage).not.toHaveBeenCalled();
      // Advance past the 300 ms debounce.
      vi.advanceTimersByTime(310);
      await waitFor(() => expect(api.transactionsPage).toHaveBeenCalledOnce());
      expect(lastQuery().q).toBe('app');
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('ActivitiesPage — delete', () => {
  let originalConfirm;
  let originalAlert;
  beforeEach(() => {
    originalConfirm = window.confirm;
    originalAlert = window.alert;
  });
  afterEach(() => {
    window.confirm = originalConfirm;
    window.alert = originalAlert;
  });

  it('cancelled confirm aborts the delete', async () => {
    window.confirm = vi.fn().mockReturnValue(false);
    const user = userEvent.setup();
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /^Delete$/ }));
    expect(window.confirm).toHaveBeenCalled();
    expect(api.deleteTx).not.toHaveBeenCalled();
  });

  it('confirmed delete calls api.deleteTx and refreshes the page + summary', async () => {
    window.confirm = vi.fn().mockReturnValue(true);
    api.deleteTx.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<ActivitiesPage privacy={false} currency="USD" user={{}} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());
    api.transactionsPage.mockClear();
    api.txSummary.mockClear();

    await user.click(screen.getByRole('button', { name: /^Delete$/ }));
    await waitFor(() => expect(api.deleteTx).toHaveBeenCalledWith(1));
    await waitFor(() => expect(api.transactionsPage).toHaveBeenCalled());
    await waitFor(() => expect(api.txSummary).toHaveBeenCalled());
  });
});
