import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    assets:     vi.fn(),
    accounts:   vi.fn(),
    fxRate:     vi.fn(),
    createTx:   vi.fn(),
    updateTx:   vi.fn(),
  },
}));

import { api } from '../api.js';
import { TxModal } from './TxModal.jsx';

const ASSETS = [
  { symbol: 'AAPL',     name: 'Apple Inc.', type: 'stock', currency: 'USD' },
  { symbol: 'CASH-EUR', name: 'EUR Cash',   type: 'cash',  currency: 'EUR' },
  { symbol: 'CASH-USD', name: 'USD Cash',   type: 'cash',  currency: 'USD' },
];
const USER_EUR_BASE = { base_currency: 'EUR' };
const USD_ACC = { id: 1, name: 'Brokerage', currency: 'USD' };
const EUR_ACC = { id: 2, name: 'EU Bank', currency: 'EUR' };
const EUR_CASH_ACC = { id: 3, name: 'Savings', type: 'cash', currency: 'EUR' };

beforeEach(() => {
  vi.clearAllMocks();
  api.assets.mockResolvedValue(ASSETS);
  api.accounts.mockResolvedValue([USD_ACC, EUR_ACC, EUR_CASH_ACC]);
  api.fxRate.mockResolvedValue({ rate: 0.92 });
  api.createTx.mockResolvedValue({ id: 99 });
  api.updateTx.mockResolvedValue({ id: 99 });
});

// A few small helpers — TxModal's <label>s aren't connected to <input>s
// via htmlFor, so we walk the .field DOM structure directly.
function fieldByLabel(container, labelText) {
  for (const f of container.querySelectorAll('.field')) {
    const label = f.querySelector('label');
    if (label && label.textContent.includes(labelText)) {
      return f.querySelector('input, select');
    }
  }
  return null;
}

async function untilLoaded(container) {
  // Wait until both dropdowns have options. Reading `select.value` is
  // unreliable in happy-dom for controlled <select value={...}> when the
  // value is set in the same render as the options — the synced .value
  // can lag — so we don't gate on it here.
  await waitFor(() => {
    const acc = fieldByLabel(container, 'Account');
    const sym = fieldByLabel(container, 'Asset');
    expect(acc?.options?.length || 0).toBeGreaterThan(0);
    expect(sym?.options?.length || 0).toBeGreaterThan(0);
  });
}

describe('TxModal — load + side adapt', () => {
  it('fetches assets + accounts on mount and populates the dropdowns', async () => {
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    expect(api.assets).toHaveBeenCalledOnce();
    expect(api.accounts).toHaveBeenCalledOnce();
    expect(fieldByLabel(container, 'Account').options).toHaveLength(3);
    expect(fieldByLabel(container, 'Asset').options).toHaveLength(3);
    // The first asset (AAPL) is a stock, so the side seg shows Buy/Sell.
    expect(screen.getByRole('button', { name: 'Buy' })).toBeInTheDocument();
  });

  it('switches side options when an asset of type cash is picked', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    // Default (AAPL) shows Buy/Sell.
    expect(screen.getByRole('button', { name: 'Buy' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sell' })).toBeInTheDocument();

    await user.selectOptions(fieldByLabel(container, 'Asset'), 'CASH-EUR');
    // Now Deposit / Withdraw / Interest.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Deposit' })).toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Withdraw' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Interest' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Buy' })).not.toBeInTheDocument();
  });

  it('auto-picks CASH-<currency> when a cash account is selected', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    await user.selectOptions(fieldByLabel(container, 'Account'), '3'); // EUR_CASH_ACC
    await waitFor(() => expect(fieldByLabel(container, 'Asset').value).toBe('CASH-EUR'));
  });
});

describe('TxModal — FX behaviour', () => {
  it('does not fetch FX when account currency matches base', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    // The default USD account triggered one initial fetch; clear so we
    // can assert that switching to EUR does not trigger another.
    await waitFor(() => expect(api.fxRate).toHaveBeenCalled());
    api.fxRate.mockClear();

    await user.selectOptions(fieldByLabel(container, 'Account'), '2'); // EUR
    await waitFor(() => expect(fieldByLabel(container, 'FX rate')).toBeNull());
    expect(api.fxRate).not.toHaveBeenCalled();
  });

  it('fetches FX when account currency differs from base', async () => {
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    // Default account is USD; base is EUR → needsFx is true.
    await waitFor(() => expect(api.fxRate).toHaveBeenCalled());
    const [from, to] = api.fxRate.mock.calls[0];
    expect(from).toBe('USD');
    expect(to).toBe('EUR');
  });

  it('the auto checkbox starts on for new transactions and toggles off on click', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    await waitFor(() => expect(fieldByLabel(container, 'FX rate')).not.toBeNull());

    const auto = container.querySelector('input[type="checkbox"]');
    expect(auto.checked).toBe(true);

    await user.click(auto);
    await waitFor(() => expect(auto.checked).toBe(false));
  });
});

describe('TxModal — submit', () => {
  it('builds the right payload for a stock buy and calls createTx', async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const onClose = vi.fn();
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={onClose} onSaved={onSaved} />,
    );
    await untilLoaded(container);
    await waitFor(() => expect(api.fxRate).toHaveBeenCalled());

    await user.type(fieldByLabel(container, 'Quantity'), '5');
    await user.type(fieldByLabel(container, 'Price per unit'), '198');
    await user.type(fieldByLabel(container, 'Fee'), '1');

    const dateField = fieldByLabel(container, 'Date');
    // Date inputs accept yyyy-mm-dd directly.
    dateField.value = '2026-04-01';
    dateField.dispatchEvent(new Event('input', { bubbles: true }));

    await user.click(screen.getByRole('button', { name: /record buy/i }));
    await waitFor(() => expect(api.createTx).toHaveBeenCalledOnce());

    const payload = api.createTx.mock.calls[0][0];
    expect(payload).toMatchObject({
      account_id: 1,
      asset_symbol: 'AAPL',
      side: 'buy',
      qty: 5,
      price: 198,
      fee: 1,
      fx_to_base: 0.92,
      note: '',
    });
    expect(payload.occurred_at).toBe('2026-04-01T12:00:00.000Z');

    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('forces price=1 and skips the price field for cash sides', async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={onSaved} />,
    );
    await untilLoaded(container);
    await user.selectOptions(fieldByLabel(container, 'Asset'), 'CASH-EUR');
    // Now in cash mode, no Price field.
    await waitFor(() => expect(fieldByLabel(container, 'Price per unit')).toBeNull());

    await user.type(fieldByLabel(container, 'Amount'), '500');
    await user.click(screen.getByRole('button', { name: /record deposit/i }));
    await waitFor(() => expect(api.createTx).toHaveBeenCalledOnce());

    const payload = api.createTx.mock.calls[0][0];
    expect(payload.asset_symbol).toBe('CASH-EUR');
    expect(payload.side).toBe('deposit');
    expect(payload.qty).toBe(500);
    expect(payload.price).toBe(1);
  });

  it('calls updateTx (not createTx) when transaction prop is supplied', async () => {
    const user = userEvent.setup();
    const tx = {
      id: 42, account_id: 1, asset_symbol: 'AAPL', side: 'buy',
      qty: 3, price: 100, fee: 0, fx_to_base: 0.9,
      occurred_at: '2026-04-01T12:00:00.000Z', note: '',
    };
    const { container } = render(
      <TxModal user={USER_EUR_BASE} transaction={tx}
        onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    // Tweak qty and save.
    const qty = fieldByLabel(container, 'Quantity');
    await user.clear(qty);
    await user.type(qty, '7');

    await user.click(screen.getByRole('button', { name: /save changes/i }));
    await waitFor(() => expect(api.updateTx).toHaveBeenCalledOnce());
    const [id, payload] = api.updateTx.mock.calls[0];
    expect(id).toBe(42);
    expect(payload.qty).toBe(7);
    expect(api.createTx).not.toHaveBeenCalled();
  });

  it('shows the API error message and re-enables the submit button on failure', async () => {
    const user = userEvent.setup();
    api.createTx.mockRejectedValueOnce(new Error('account not found'));
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    await waitFor(() => expect(api.fxRate).toHaveBeenCalled());

    await user.type(fieldByLabel(container, 'Quantity'), '1');
    await user.type(fieldByLabel(container, 'Price per unit'), '10');
    await user.click(screen.getByRole('button', { name: /record buy/i }));

    await waitFor(() => expect(screen.getByText('account not found')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /record buy/i })).toBeEnabled();
  });

  it('keeps submit disabled until quantity is filled', async () => {
    const { container } = render(
      <TxModal user={USER_EUR_BASE} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await untilLoaded(container);
    expect(screen.getByRole('button', { name: /record buy/i })).toBeDisabled();
  });
});
