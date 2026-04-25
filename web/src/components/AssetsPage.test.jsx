import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    assets:      vi.fn(),
    deleteAsset: vi.fn(),
  },
}));

// Stub AssetModal + AssetDetailsModal so AssetsPage's own logic is
// what's under test. Each stub renders a recognisable marker so we
// can assert it appears with the right asset.
vi.mock('./AssetModal.jsx', () => ({
  AssetModal: ({ asset, onClose, onSaved }) => (
    <div data-testid="asset-modal" data-symbol={asset?.symbol || ''}>
      <button onClick={onClose}>close-asset-modal</button>
      <button onClick={onSaved}>save-asset-modal</button>
    </div>
  ),
}));
vi.mock('./AssetDetailsModal.jsx', () => ({
  AssetDetailsModal: ({ asset, onClose, onShowActivities }) => (
    <div data-testid="asset-details" data-symbol={asset.symbol}>
      <button onClick={() => onShowActivities(asset.symbol)}>show-activities</button>
      <button onClick={onClose}>close-details</button>
    </div>
  ),
}));

import { api } from '../api.js';
import { AssetsPage } from './AssetsPage.jsx';

const ASSETS = [
  { symbol: 'AAPL', name: 'Apple Inc.', type: 'stock',  currency: 'USD', provider: 'yahoo',     provider_id: 'AAPL' },
  { symbol: 'BTC',  name: 'Bitcoin',    type: 'crypto', currency: 'USD', provider: 'coingecko', provider_id: 'bitcoin' },
  { symbol: 'CASH-EUR', name: 'EUR Cash', type: 'cash', currency: 'EUR' },
];

beforeEach(() => {
  vi.clearAllMocks();
  api.assets.mockResolvedValue(ASSETS);
  api.deleteAsset.mockResolvedValue(null);
});

describe('AssetsPage — load + render', () => {
  it('renders one row per asset and the "tracked" count', async () => {
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());
    expect(screen.getByText('Bitcoin')).toBeInTheDocument();
    expect(screen.getByText('EUR Cash')).toBeInTheDocument();
    expect(screen.getByText(/3 tracked/)).toBeInTheDocument();
  });

  it('renders an Error: <msg> when the fetch fails', async () => {
    api.assets.mockRejectedValueOnce(new Error('upstream'));
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText(/Error: upstream/)).toBeInTheDocument());
  });

  it('renders the empty state when there are no assets at all', async () => {
    api.assets.mockResolvedValueOnce([]);
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText(/No assets yet/)).toBeInTheDocument(),
    );
  });

  it('shows "No matches" when the filter excludes every row', async () => {
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    await user.type(screen.getByPlaceholderText(/Filter…/i), 'zzz-not-a-thing');
    await waitFor(() => expect(screen.getByText('No matches.')).toBeInTheDocument());
  });
});

describe('AssetsPage — filter', () => {
  it('filters by symbol substring (case-insensitive)', async () => {
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    await user.type(screen.getByPlaceholderText(/Filter…/i), 'btc');
    await waitFor(() => expect(screen.queryByText('Apple Inc.')).not.toBeInTheDocument());
    expect(screen.getByText('Bitcoin')).toBeInTheDocument();
  });

  it('filters by name substring (case-insensitive)', async () => {
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    await user.type(screen.getByPlaceholderText(/Filter…/i), 'apple');
    await waitFor(() => expect(screen.queryByText('Bitcoin')).not.toBeInTheDocument());
    expect(screen.getByText('Apple Inc.')).toBeInTheDocument();
  });
});

describe('AssetsPage — row interactions', () => {
  it('clicking a non-cash row opens the details modal', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AssetsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    // First non-cash row in tbody → AAPL.
    const rows = container.querySelectorAll('tbody tr');
    await user.click(rows[0]);

    const details = await screen.findByTestId('asset-details');
    expect(details.dataset.symbol).toBe('AAPL');
  });

  it('clicking a cash row does not open the details modal', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AssetsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('EUR Cash')).toBeInTheDocument());

    const cashRow = [...container.querySelectorAll('tbody tr')]
      .find((r) => r.textContent.includes('EUR Cash'));
    await user.click(cashRow);
    expect(screen.queryByTestId('asset-details')).not.toBeInTheDocument();
  });

  it('Edit button stops propagation and opens AssetModal with that asset', async () => {
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    // Two Edit buttons (AAPL + BTC + cash). Click the first.
    const editButtons = screen.getAllByRole('button', { name: 'Edit' });
    await user.click(editButtons[0]);

    const modal = await screen.findByTestId('asset-modal');
    expect(modal.dataset.symbol).toBe('AAPL');
    // Details modal must NOT have opened (stopPropagation).
    expect(screen.queryByTestId('asset-details')).not.toBeInTheDocument();
  });

  it('show-activities from AssetDetailsModal calls onOpenActivity then closes', async () => {
    const user = userEvent.setup();
    const onOpenActivity = vi.fn();
    const { container } = render(
      <AssetsPage privacy={false} onOpenActivity={onOpenActivity} />,
    );
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    const rows = container.querySelectorAll('tbody tr');
    await user.click(rows[0]); // open details for AAPL
    await screen.findByTestId('asset-details');

    await user.click(screen.getByText('show-activities'));
    expect(onOpenActivity).toHaveBeenCalledWith('AAPL');
  });
});

describe('AssetsPage — delete', () => {
  let originalConfirm;
  beforeEach(() => {
    originalConfirm = window.confirm;
  });
  afterEach(() => {
    window.confirm = originalConfirm;
  });

  it('cancelled confirm aborts the delete', async () => {
    window.confirm = vi.fn().mockReturnValue(false);
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0]);
    expect(window.confirm).toHaveBeenCalled();
    expect(api.deleteAsset).not.toHaveBeenCalled();
  });

  it('confirmed delete calls api.deleteAsset and reloads the list', async () => {
    window.confirm = vi.fn().mockReturnValue(true);
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(api.assets).toHaveBeenCalledOnce());

    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0]);
    await waitFor(() => expect(api.deleteAsset).toHaveBeenCalledWith('AAPL'));
    await waitFor(() => expect(api.assets).toHaveBeenCalledTimes(2));
  });
});

describe('AssetsPage — add', () => {
  it('Add asset opens AssetModal in create mode (no asset prop)', async () => {
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Apple Inc.')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /add asset/i }));
    const modal = await screen.findByTestId('asset-modal');
    expect(modal.dataset.symbol).toBe('');
  });

  it('saving from the Add modal closes it and reloads assets', async () => {
    const user = userEvent.setup();
    render(<AssetsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(api.assets).toHaveBeenCalledOnce());

    await user.click(screen.getByRole('button', { name: /add asset/i }));
    await screen.findByTestId('asset-modal');
    await user.click(screen.getByText('save-asset-modal'));

    await waitFor(() => expect(screen.queryByTestId('asset-modal')).not.toBeInTheDocument());
    await waitFor(() => expect(api.assets).toHaveBeenCalledTimes(2));
  });
});
