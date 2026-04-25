import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    upsertAsset: vi.fn(),
    lookupAsset: vi.fn(),
  },
}));

import { api } from '../api.js';
import { AssetModal } from './AssetModal.jsx';

beforeEach(() => {
  vi.clearAllMocks();
  api.upsertAsset.mockResolvedValue({ ok: true });
  api.lookupAsset.mockResolvedValue({});
});

function fieldByLabel(container, labelText) {
  for (const f of container.querySelectorAll('.field')) {
    const label = f.querySelector('label');
    if (label && label.textContent.includes(labelText)) {
      return f.querySelector('input, select');
    }
  }
  return null;
}

describe('AssetModal — render', () => {
  it('opens in Add mode with default Type=stock and Provider=yahoo', () => {
    const { container } = render(
      <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    expect(screen.getByText('Add asset')).toBeInTheDocument();
    expect(fieldByLabel(container, 'Type').value).toBe('stock');
    expect(fieldByLabel(container, 'Provider').value).toBe('yahoo');
    expect(fieldByLabel(container, 'Native currency').value).toBe('USD');
  });

  it('seeds every field from the asset prop in Edit mode', () => {
    const asset = {
      symbol: 'AAPL', name: 'Apple Inc.', type: 'stock',
      currency: 'USD', provider: 'yahoo', provider_id: 'AAPL',
      logo_url: 'https://x/aapl.png',
    };
    const { container } = render(
      <AssetModal asset={asset} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    expect(screen.getByText('Edit asset')).toBeInTheDocument();
    expect(fieldByLabel(container, 'Symbol').value).toBe('AAPL');
    expect(fieldByLabel(container, 'Name').value).toBe('Apple Inc.');
    // Symbol input is locked once the row exists.
    expect(fieldByLabel(container, 'Symbol')).toBeDisabled();
  });

  it('hides symbol/name/provider rows for Cash type', async () => {
    const { container } = render(
      <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    fireEvent.change(fieldByLabel(container, 'Type'), { target: { value: 'cash' } });
    await waitFor(() => expect(fieldByLabel(container, 'Symbol')).toBeNull());
    expect(fieldByLabel(container, 'Provider')).toBeNull();
  });
});

describe('AssetModal — type → provider auto-snap', () => {
  it('switching to Crypto snaps Provider to coingecko', async () => {
    const { container } = render(
      <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    fireEvent.change(fieldByLabel(container, 'Type'), { target: { value: 'crypto' } });
    await waitFor(() =>
      expect(fieldByLabel(container, 'Provider').value).toBe('coingecko'),
    );
  });

  it('switching back to Stock snaps Provider to yahoo', async () => {
    const { container } = render(
      <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    fireEvent.change(fieldByLabel(container, 'Type'), { target: { value: 'crypto' } });
    await waitFor(() =>
      expect(fieldByLabel(container, 'Provider').value).toBe('coingecko'),
    );
    fireEvent.change(fieldByLabel(container, 'Type'), { target: { value: 'stock' } });
    await waitFor(() =>
      expect(fieldByLabel(container, 'Provider').value).toBe('yahoo'),
    );
  });

  it('does not auto-snap on first render in edit mode (preserves stored provider)', async () => {
    const asset = {
      symbol: 'BTC', name: 'Bitcoin', type: 'crypto',
      currency: 'USD', provider: 'coingecko', provider_id: 'bitcoin',
    };
    render(<AssetModal asset={asset} onClose={vi.fn()} onSaved={vi.fn()} />);
    // Allow any auto-snap effect a tick to fire.
    await new Promise((r) => setTimeout(r, 5));
    // Provider stayed put; lookup also did NOT fire on first edit-render.
    expect(api.lookupAsset).not.toHaveBeenCalled();
  });
});

describe('AssetModal — provider lookup', () => {
  it('debounces the lookup and applies the response (name, currency, type, provider_id, logo)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      api.lookupAsset.mockResolvedValueOnce({
        name: 'Apple Inc.', currency: 'USD', type: 'stock',
        provider_id: 'AAPL', logo_url: 'https://x/aapl.png',
      });
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      const { container } = render(
        <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
      );
      await user.type(fieldByLabel(container, 'Symbol'), 'AAPL');

      // Lookup is gated behind a 400 ms debounce.
      expect(api.lookupAsset).not.toHaveBeenCalled();
      vi.advanceTimersByTime(420);

      await waitFor(() =>
        expect(api.lookupAsset).toHaveBeenCalledWith('AAPL', 'yahoo'),
      );
      vi.useRealTimers();
      // Now real timers — wait for the state updates to flush.
      await waitFor(() =>
        expect(fieldByLabel(container, 'Name').value).toBe('Apple Inc.'),
      );
      expect(fieldByLabel(container, 'Provider ID').value).toBe('AAPL');
    } finally {
      vi.useRealTimers();
    }
  });

  it('skips the lookup when symbol is empty', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      render(<AssetModal onClose={vi.fn()} onSaved={vi.fn()} />);
      vi.advanceTimersByTime(500);
      expect(api.lookupAsset).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it('swallows lookup errors silently (keeps the user input)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      api.lookupAsset.mockRejectedValueOnce(new Error('404'));
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      const { container } = render(
        <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
      );
      await user.type(fieldByLabel(container, 'Name'), 'My override');
      await user.type(fieldByLabel(container, 'Symbol'), 'XYZ');
      vi.advanceTimersByTime(420);
      await waitFor(() => expect(api.lookupAsset).toHaveBeenCalled());
      vi.useRealTimers();
      // The user-entered name is preserved.
      expect(fieldByLabel(container, 'Name').value).toBe('My override');
    } finally {
      vi.useRealTimers();
    }
  });

  it('cash type short-circuits the lookup effect entirely', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { container } = render(
        <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
      );
      fireEvent.change(fieldByLabel(container, 'Type'), { target: { value: 'cash' } });
      vi.advanceTimersByTime(500);
      expect(api.lookupAsset).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('AssetModal — submit', () => {
  it('upper-cases the symbol and trims fields in the upsert payload', async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const onClose = vi.fn();
    const { container } = render(
      <AssetModal onClose={onClose} onSaved={onSaved} />,
    );
    await user.type(fieldByLabel(container, 'Symbol'), 'aapl');
    await user.type(fieldByLabel(container, 'Name'), '  Apple Inc.  ');
    await user.type(fieldByLabel(container, 'Provider ID'), '  AAPL  ');

    await user.click(screen.getByRole('button', { name: /create asset/i }));
    await waitFor(() => expect(api.upsertAsset).toHaveBeenCalledOnce());
    expect(api.upsertAsset.mock.calls[0][0]).toMatchObject({
      symbol: 'AAPL',
      name: 'Apple Inc.',
      provider_id: 'AAPL',
      type: 'stock',
      currency: 'USD',
      provider: 'yahoo',
    });
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('cash submit derives the symbol/name from currency and clears provider info', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    fireEvent.change(fieldByLabel(container, 'Type'), { target: { value: 'cash' } });
    fireEvent.change(fieldByLabel(container, 'Native currency'), { target: { value: 'EUR' } });

    await user.click(screen.getByRole('button', { name: /create asset/i }));
    await waitFor(() => expect(api.upsertAsset).toHaveBeenCalledOnce());
    expect(api.upsertAsset.mock.calls[0][0]).toEqual({
      symbol: 'CASH-EUR',
      name: 'EUR Cash',
      type: 'cash',
      currency: 'EUR',
      provider: '',
      provider_id: '',
      logo_url: '',
    });
  });

  it('rejects submit and surfaces the inline error when symbol/name missing', async () => {
    const { container } = render(
      <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    // Submit button is disabled when fields are empty; bypass via the form.
    container.querySelector('form').dispatchEvent(
      new Event('submit', { cancelable: true, bubbles: true }),
    );
    await waitFor(() =>
      expect(screen.getByText(/symbol and name are required/i)).toBeInTheDocument(),
    );
    expect(api.upsertAsset).not.toHaveBeenCalled();
  });

  it('renders the API error and re-enables the submit button on failure', async () => {
    api.upsertAsset.mockRejectedValueOnce(new Error('duplicate symbol'));
    const user = userEvent.setup();
    const { container } = render(
      <AssetModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Symbol'), 'AAPL');
    await user.type(fieldByLabel(container, 'Name'), 'Apple');

    await user.click(screen.getByRole('button', { name: /create asset/i }));
    await waitFor(() => expect(screen.getByText('duplicate symbol')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /create asset/i })).toBeEnabled();
  });
});

describe('AssetModal — chrome', () => {
  it('Cancel + close-icon both call onClose', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const { container } = render(
      <AssetModal onClose={onClose} onSaved={vi.fn()} />,
    );
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalledTimes(1);

    const closeIcon = container.querySelectorAll('button.icon-btn')[0];
    await user.click(closeIcon);
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
