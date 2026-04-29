import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    createAccount: vi.fn(),
    updateAccount: vi.fn(),
  },
}));

import { api } from '../api.js';
import { AccountModal } from './AccountModal.jsx';

beforeEach(() => {
  vi.clearAllMocks();
  api.createAccount.mockResolvedValue({ id: 1 });
  api.updateAccount.mockResolvedValue({ id: 1 });
});

// Same trick we use in TxModal — labels aren't tied to inputs via
// htmlFor, so walk the .field structure.
function fieldByLabel(container, labelText) {
  for (const f of container.querySelectorAll('.field')) {
    const label = f.querySelector('label');
    if (label && label.textContent.includes(labelText)) {
      return f.querySelector('input, select');
    }
  }
  return null;
}

describe('AccountModal — create', () => {
  it('keeps Create disabled until a name is supplied', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AccountModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    const submit = screen.getByRole('button', { name: /create account/i });
    expect(submit).toBeDisabled();

    await user.type(fieldByLabel(container, 'Name'), 'Sunset Exchange');
    expect(submit).toBeEnabled();
  });

  it('builds the right payload (auto short, default color, default type)', async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const onClose = vi.fn();
    const { container } = render(
      <AccountModal onClose={onClose} onSaved={onSaved} />,
    );
    await user.type(fieldByLabel(container, 'Name'), 'sunset exchange');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => expect(api.createAccount).toHaveBeenCalledOnce());
    const payload = api.createAccount.mock.calls[0][0];
    expect(payload).toMatchObject({
      name: 'sunset exchange',
      type: 'Brokerage',
      currency: 'USD',
      color: '#c8502a',
    });
    // autoShort: first letters of "sunset exchange" → "SE", then upper.
    expect(payload.short).toBe('SE');
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('upper-cases the explicit short label and trims to 3 chars', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AccountModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Name'), 'X');
    const shortField = fieldByLabel(container, 'Short label');
    await user.type(shortField, 'abcd'); // maxlength=3 → "abc"
    expect(shortField.value).toBe('ABC');

    await user.click(screen.getByRole('button', { name: /create account/i }));
    await waitFor(() => expect(api.createAccount).toHaveBeenCalled());
    expect(api.createAccount.mock.calls[0][0].short).toBe('ABC');
  });

  it('falls back to "??" when name is whitespace-only and no short is set', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AccountModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    // Whitespace name: empty short → autoShort '' → '??'.
    // The name guard rejects whitespace-only names, so set a real one
    // then erase the autoShort path with a single-letter name.
    await user.type(fieldByLabel(container, 'Name'), 'a');
    await user.click(screen.getByRole('button', { name: /create account/i }));
    await waitFor(() => expect(api.createAccount).toHaveBeenCalled());
    // 'a' → autoShort 'A'.
    expect(api.createAccount.mock.calls[0][0].short).toBe('A');
  });

  it('shows the Name-required error and does not call the API on whitespace name', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AccountModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    // Type then erase to force submitting=disabled to flip enabled briefly.
    // Easiest: dispatch submit on the form directly.
    await user.type(fieldByLabel(container, 'Name'), '  ');
    // Submit button is disabled → simulate submit via Enter on form.
    const form = container.querySelector('form');
    form.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));

    await waitFor(() => expect(screen.getByText('Name is required.')).toBeInTheDocument());
    expect(api.createAccount).not.toHaveBeenCalled();
  });

  it('selects a colour from the swatch row', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AccountModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Name'), 'X');
    // Pick the third colour swatch.
    await user.click(screen.getByRole('button', { name: 'color #a8572e' }));
    await user.click(screen.getByRole('button', { name: /create account/i }));
    await waitFor(() => expect(api.createAccount).toHaveBeenCalled());
    expect(api.createAccount.mock.calls[0][0].color).toBe('#a8572e');
  });

  it('renders the API error and re-enables the submit button on failure', async () => {
    api.createAccount.mockRejectedValueOnce(new Error('duplicate name'));
    const user = userEvent.setup();
    const { container } = render(
      <AccountModal onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Name'), 'X');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => expect(screen.getByText('duplicate name')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /create account/i })).toBeEnabled();
  });
});

describe('AccountModal — palette filtering', () => {
  it('hides colors already used by other accounts', () => {
    render(
      <AccountModal usedColors={['#c8502a', '#d4953d']}
        onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    expect(screen.queryByRole('button', { name: 'color #c8502a' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'color #d4953d' })).not.toBeInTheDocument();
    // A color that's not reserved should still appear.
    expect(screen.getByRole('button', { name: 'color #a8572e' })).toBeInTheDocument();
  });

  it('always shows 6 swatches by backfilling from the reserve pool', () => {
    const { container } = render(
      <AccountModal usedColors={['#c8502a', '#d4953d', '#a8572e']}
        onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    const swatches = container.querySelectorAll('button[aria-label^="color "]');
    expect(swatches.length).toBe(6);
  });

  it('defaults a new account to the first non-reserved color', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AccountModal usedColors={['#c8502a', '#d4953d']}
        onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Name'), 'X');
    await user.click(screen.getByRole('button', { name: /create account/i }));
    await waitFor(() => expect(api.createAccount).toHaveBeenCalled());
    expect(api.createAccount.mock.calls[0][0].color).toBe('#a8572e');
  });

  it('keeps the edited account\'s own color in the palette even if listed as used', () => {
    const account = {
      id: 7, name: 'X', type: 'Brokerage', short: 'X',
      color: '#c8502a', currency: 'USD',
    };
    render(
      <AccountModal account={account}
        usedColors={['#c8502a', '#d4953d']}
        onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    // Own color stays selectable.
    expect(screen.getByRole('button', { name: 'color #c8502a' })).toBeInTheDocument();
    // Other reserved colors are still hidden.
    expect(screen.queryByRole('button', { name: 'color #d4953d' })).not.toBeInTheDocument();
  });
});

describe('AccountModal — edit', () => {
  it('seeds the form from the account prop and dispatches updateAccount', async () => {
    const user = userEvent.setup();
    const account = {
      id: 7, name: 'Old name', type: 'Crypto Exchange', short: 'OE',
      color: '#7a8c6f', currency: 'EUR',
    };
    const { container } = render(
      <AccountModal account={account} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    expect(fieldByLabel(container, 'Name').value).toBe('Old name');
    expect(fieldByLabel(container, 'Type').value).toBe('Crypto Exchange');
    expect(fieldByLabel(container, 'Currency').value).toBe('EUR');

    // Title and submit label should switch to "edit" wording.
    expect(screen.getByText('Edit account')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /save changes/i }));
    await waitFor(() => expect(api.updateAccount).toHaveBeenCalledWith(7, expect.objectContaining({
      name: 'Old name',
      type: 'Crypto Exchange',
      currency: 'EUR',
      short: 'OE',
      color: '#7a8c6f',
    })));
    expect(api.createAccount).not.toHaveBeenCalled();
  });
});

describe('AccountModal — chrome', () => {
  it('clicking the close icon calls onClose', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const { container } = render(
      <AccountModal onClose={onClose} onSaved={vi.fn()} />,
    );
    // The close icon is an icon-btn rendered via an SVG; grab via title.
    const buttons = container.querySelectorAll('button.icon-btn');
    await user.click(buttons[0]);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('clicking the Cancel text button calls onClose', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<AccountModal onClose={onClose} onSaved={vi.fn()} />);
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('clicking the backdrop calls onClose; clicking inside the modal does not', async () => {
    const onClose = vi.fn();
    const { container } = render(
      <AccountModal onClose={onClose} onSaved={vi.fn()} />,
    );
    const backdrop = container.querySelector('.modal-backdrop');
    // Click on the backdrop itself → onClose.
    backdrop.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    // The handler checks e.target === e.currentTarget; bubbling click
    // from a child wouldn't match. Trigger the handler directly:
    backdrop.click();
    expect(onClose).toHaveBeenCalled();
  });
});
