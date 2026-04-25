import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    accounts:      vi.fn(),
    transactions:  vi.fn(),
    holdings:      vi.fn(),
    deleteAccount: vi.fn(),
  },
}));

// Stub the child modals/menus so we can assert *what* opens and
// *with what* without testing their internals from this file.
vi.mock('./AccountModal.jsx', () => ({
  AccountModal: ({ account, onClose, onSaved }) => (
    <div data-testid="account-modal" data-id={account?.id ?? ''}>
      <button onClick={onClose}>close-acc-modal</button>
      <button onClick={onSaved}>save-acc-modal</button>
    </div>
  ),
}));
vi.mock('./AccountCardMenu.jsx', () => ({
  AccountCardMenu: ({ onEdit, onDelete, onClose }) => (
    <div data-testid="acc-card-menu">
      <button onClick={onEdit}>menu-edit</button>
      <button onClick={onDelete}>menu-delete</button>
      <button onClick={onClose}>menu-close</button>
    </div>
  ),
}));

import { api } from '../api.js';
import { AccountsPage } from './AccountsPage.jsx';

const ACC_BROK = { id: 1, name: 'Main Brokerage', type: 'Brokerage', currency: 'USD', short: 'BR', color: '#c8502a' };
const ACC_BANK = { id: 2, name: 'EU Bank',   type: 'Cash / Savings', currency: 'EUR', short: 'EB' };
const ACC_EMPTY = { id: 3, name: 'New Wallet', type: 'Self-custody', currency: 'USD', short: 'NW' };

// AAPL: 5 @ 100 + 3 @ 110 = avg 103.75; sell 4 @ 120 → realized 65,
// open qty=4, open cost=415. Holdings give per-unit price 130 →
// open value 520, unrealized = 520 − 415 = 105.
const TXS = [
  { id: 1, account_id: 1, asset_symbol: 'AAPL', side: 'buy',  qty: 5, price: 100, fee: 0, occurred_at: '2026-01-01T12:00:00Z' },
  { id: 2, account_id: 1, asset_symbol: 'AAPL', side: 'buy',  qty: 3, price: 110, fee: 0, occurred_at: '2026-02-01T12:00:00Z' },
  { id: 3, account_id: 1, asset_symbol: 'AAPL', side: 'sell', qty: 4, price: 120, fee: 0, occurred_at: '2026-03-01T12:00:00Z' },
  // Cash account with deposit/interest/withdraw flows.
  { id: 4, account_id: 2, asset_symbol: 'CASH-EUR', side: 'deposit',  qty: 1000, price: 1, fee: 0, occurred_at: '2026-01-15T12:00:00Z' },
  { id: 5, account_id: 2, asset_symbol: 'CASH-EUR', side: 'interest', qty:   50, price: 1, fee: 0, occurred_at: '2026-02-15T12:00:00Z' },
  { id: 6, account_id: 2, asset_symbol: 'CASH-EUR', side: 'withdraw', qty:  200, price: 1, fee: 0, occurred_at: '2026-03-15T12:00:00Z' },
];
const HOLDINGS = [
  // AAPL: qty 4 across the user, ValueNative = 4 × 130 = 520.
  { Symbol: 'AAPL', Currency: 'USD', Qty: 4, ValueNative: 520, PriceStale: false },
];

beforeEach(() => {
  vi.clearAllMocks();
  api.accounts.mockResolvedValue([ACC_BROK, ACC_BANK, ACC_EMPTY]);
  api.transactions.mockResolvedValue(TXS);
  api.holdings.mockResolvedValue(HOLDINGS);
  api.deleteAccount.mockResolvedValue(null);
});

describe('AccountsPage — load', () => {
  it('mounts and fires accounts + transactions + holdings', async () => {
    render(<AccountsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => {
      expect(api.accounts).toHaveBeenCalledOnce();
      expect(api.transactions).toHaveBeenCalledOnce();
      expect(api.holdings).toHaveBeenCalledOnce();
    });
  });

  it('renders an Error: <msg> empty state when one of the fetches fails', async () => {
    api.accounts.mockRejectedValueOnce(new Error('upstream'));
    render(<AccountsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText(/Error: upstream/)).toBeInTheDocument());
  });
});

describe('AccountsPage — per-account stats', () => {
  it('cash account renders the running balance (deposit + interest − withdraw)', async () => {
    render(<AccountsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('EU Bank')).toBeInTheDocument());

    // Cash balance = 1000 + 50 − 200 = 850 EUR.
    expect(screen.getByText(/Cash balance · EUR/)).toBeInTheDocument();
    expect(screen.getByText('€850.00')).toBeInTheDocument();
  });

  it('trade account shows open value, cost, unrealized PnL%, and realised PnL', async () => {
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('Main Brokerage')).toBeInTheDocument());

    // Current value = 520, cost = 415, unrealized = +105 (≈ 25.30%).
    expect(screen.getByText('$520.00')).toBeInTheDocument();
    const card = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('Main Brokerage'));
    expect(card.textContent).toMatch(/Cost \$415\.00/);
    expect(card.textContent).toMatch(/\+\$105\.00/);
    expect(card.textContent).toMatch(/25\.30%/);
    // Realized PnL = (120 − 103.75) × 4 = 65.
    expect(card.textContent).toMatch(/Realized \+\$65\.00/);
  });

  it('falls back to cost when the holdings price is stale + flags ⚠', async () => {
    api.holdings.mockResolvedValueOnce([
      { Symbol: 'AAPL', Currency: 'USD', Qty: 4, ValueNative: 0, PriceStale: true },
    ]);
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('Main Brokerage')).toBeInTheDocument());

    const card = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('Main Brokerage'));
    expect(card.querySelector('span[title*="Price data missing"]')).not.toBeNull();
    // Open value falls back to open cost (415); unrealized = 0.
    expect(card.textContent).toMatch(/\$415\.00/);
  });

  it('account with no activity renders the zeroed cost-basis stat', async () => {
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('New Wallet')).toBeInTheDocument());

    const card = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('New Wallet'));
    expect(card.textContent).toMatch(/Cost basis · USD/);
    expect(card.textContent).toMatch(/\$0\.00/);
    expect(card.textContent).toMatch(/0 transactions/);
  });

  it('account count line uses singular for exactly one transaction', async () => {
    api.transactions.mockResolvedValueOnce([
      { id: 1, account_id: 3, asset_symbol: 'CASH-USD', side: 'deposit', qty: 100, price: 1, fee: 0, occurred_at: '2026-01-01T12:00:00Z' },
    ]);
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('New Wallet')).toBeInTheDocument());

    const card = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('New Wallet'));
    expect(card.textContent).toMatch(/1 transaction\b/);
  });
});

describe('AccountsPage — interactions', () => {
  it('clicking an active card calls onOpenActivity with the account id', async () => {
    const user = userEvent.setup();
    const onOpenActivity = vi.fn();
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={onOpenActivity} />,
    );
    await waitFor(() => expect(screen.getByText('Main Brokerage')).toBeInTheDocument());

    const card = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('Main Brokerage'));
    await user.click(card);
    expect(onOpenActivity).toHaveBeenCalledWith(1);
  });

  it('a card with zero transactions is not clickable', async () => {
    const user = userEvent.setup();
    const onOpenActivity = vi.fn();
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={onOpenActivity} />,
    );
    await waitFor(() => expect(screen.getByText('New Wallet')).toBeInTheDocument());

    const card = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('New Wallet'));
    await user.click(card);
    expect(onOpenActivity).not.toHaveBeenCalled();
  });

  it('"more" button opens the card menu (does not bubble to the card click)', async () => {
    const user = userEvent.setup();
    const onOpenActivity = vi.fn();
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={onOpenActivity} />,
    );
    await waitFor(() => expect(screen.getByText('Main Brokerage')).toBeInTheDocument());

    const card = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('Main Brokerage'));
    const moreBtn = card.querySelector('button[aria-haspopup="menu"]');
    await user.click(moreBtn);

    expect(screen.getByTestId('acc-card-menu')).toBeInTheDocument();
    expect(onOpenActivity).not.toHaveBeenCalled();
  });

  it('menu Edit opens AccountModal with the right account id', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('Main Brokerage')).toBeInTheDocument());

    const brokCard = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('Main Brokerage'));
    await user.click(brokCard.querySelector('button[aria-haspopup="menu"]'));
    await user.click(screen.getByText('menu-edit'));

    const modal = await screen.findByTestId('account-modal');
    expect(modal.dataset.id).toBe('1');
  });
});

describe('AccountsPage — delete', () => {
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
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(screen.getByText('Main Brokerage')).toBeInTheDocument());

    const brokCard = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('Main Brokerage'));
    await user.click(brokCard.querySelector('button[aria-haspopup="menu"]'));
    await user.click(screen.getByText('menu-delete'));
    expect(api.deleteAccount).not.toHaveBeenCalled();
  });

  it('confirmed delete calls api.deleteAccount and reloads everything', async () => {
    window.confirm = vi.fn().mockReturnValue(true);
    const user = userEvent.setup();
    const { container } = render(
      <AccountsPage privacy={false} onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(api.accounts).toHaveBeenCalledOnce());

    const brokCard = [...container.querySelectorAll('.acc-card')]
      .find((c) => c.textContent.includes('Main Brokerage'));
    await user.click(brokCard.querySelector('button[aria-haspopup="menu"]'));
    await user.click(screen.getByText('menu-delete'));

    await waitFor(() => expect(api.deleteAccount).toHaveBeenCalledWith(1));
    await waitFor(() => expect(api.accounts).toHaveBeenCalledTimes(2));
  });
});

describe('AccountsPage — add', () => {
  it('Add an account opens AccountModal in create mode (no account prop)', async () => {
    const user = userEvent.setup();
    render(<AccountsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Main Brokerage')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /add an account/i }));
    const modal = await screen.findByTestId('account-modal');
    expect(modal.dataset.id).toBe('');
  });

  it('saved → modal closes and the page reloads', async () => {
    const user = userEvent.setup();
    render(<AccountsPage privacy={false} onOpenActivity={vi.fn()} />);
    await waitFor(() => expect(api.accounts).toHaveBeenCalledOnce());

    await user.click(screen.getByRole('button', { name: /add an account/i }));
    await user.click(screen.getByText('save-acc-modal'));
    await waitFor(() => expect(screen.queryByTestId('account-modal')).not.toBeInTheDocument());
    await waitFor(() => expect(api.accounts).toHaveBeenCalledTimes(2));
  });
});

describe('AccountsPage — privacy', () => {
  it('wraps monetary values in .masked when privacy is on', async () => {
    const { container } = render(
      <AccountsPage privacy onOpenActivity={vi.fn()} />,
    );
    await waitFor(() => expect(container.querySelector('.masked')).not.toBeNull());
    // Cash balance, current value, cost, unrealized, realized → many .masked.
    expect(container.querySelectorAll('.masked').length).toBeGreaterThan(3);
  });
});
