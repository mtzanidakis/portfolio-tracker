import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    listTokens:  vi.fn(),
    createToken: vi.fn(),
    revokeToken: vi.fn(),
    deleteToken: vi.fn(),
  },
}));

import { api } from '../api.js';
import { TokensModal } from './TokensModal.jsx';

const ACTIVE = {
  id: 1, name: 'laptop-cli',
  created_at: '2026-04-01T12:00:00Z',
  last_used_at: '2026-04-20T08:00:00Z',
  revoked_at: null,
};
const REVOKED = {
  id: 2, name: 'old-laptop',
  created_at: '2026-01-01T12:00:00Z',
  last_used_at: null,
  revoked_at: '2026-03-01T00:00:00Z',
};

beforeEach(() => {
  vi.clearAllMocks();
  api.listTokens.mockResolvedValue([ACTIVE, REVOKED]);
  api.createToken.mockResolvedValue({ id: 3, name: 'new-one', token: 'pt_xyz_secret' });
  api.revokeToken.mockResolvedValue(null);
  api.deleteToken.mockResolvedValue(null);
});

describe('TokensModal — listing', () => {
  it('shows Loading… while listTokens is in flight', () => {
    let resolve;
    api.listTokens.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
    render(<TokensModal onClose={vi.fn()} />);
    expect(screen.getByText('Loading…')).toBeInTheDocument();
    resolve([]);
  });

  it('renders an Active row and a Revoked row from the server', async () => {
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeInTheDocument());
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('old-laptop')).toBeInTheDocument();
    expect(screen.getByText('Revoked')).toBeInTheDocument();
    // The Revoke button only renders for active tokens.
    const revokeButtons = screen.getAllByRole('button', { name: 'Revoke' });
    expect(revokeButtons).toHaveLength(1);
  });

  it('shows the empty state when the user has no tokens', async () => {
    api.listTokens.mockResolvedValueOnce([]);
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('No tokens yet.')).toBeInTheDocument());
  });

  it('renders a — for tokens that were never used', async () => {
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('old-laptop')).toBeInTheDocument());
    // Two — entries: revoked token's last_used_at column.
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });
});

describe('TokensModal — create', () => {
  it('Create button stays disabled until a name is entered', async () => {
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeInTheDocument());

    const btn = screen.getByRole('button', { name: /create token/i });
    expect(btn).toBeDisabled();

    await user.type(screen.getByPlaceholderText(/Token name/i), 'ci-bot');
    expect(btn).toBeEnabled();
  });

  it('shows the just-created token panel and reloads the list on submit', async () => {
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(api.listTokens).toHaveBeenCalledOnce());

    await user.type(screen.getByPlaceholderText(/Token name/i), 'ci-bot');
    await user.click(screen.getByRole('button', { name: /create token/i }));

    await waitFor(() => expect(api.createToken).toHaveBeenCalledWith('ci-bot'));
    // The reveal panel renders the secret verbatim — copied once, gone forever.
    await waitFor(() => expect(screen.getByText('pt_xyz_secret')).toBeInTheDocument());
    // listTokens is called again to refresh the table.
    await waitFor(() => expect(api.listTokens).toHaveBeenCalledTimes(2));
    // The name input was cleared.
    expect(screen.getByPlaceholderText(/Token name/i).value).toBe('');
  });

  it('"Done" dismisses the just-created panel', async () => {
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(api.listTokens).toHaveBeenCalled());

    await user.type(screen.getByPlaceholderText(/Token name/i), 'ci-bot');
    await user.click(screen.getByRole('button', { name: /create token/i }));
    await waitFor(() => expect(screen.getByText('pt_xyz_secret')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Done' }));
    expect(screen.queryByText('pt_xyz_secret')).not.toBeInTheDocument();
  });

  it('renders the API error if createToken rejects', async () => {
    api.createToken.mockRejectedValueOnce(new Error('quota exceeded'));
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(api.listTokens).toHaveBeenCalled());

    await user.type(screen.getByPlaceholderText(/Token name/i), 'ci-bot');
    await user.click(screen.getByRole('button', { name: /create token/i }));
    await waitFor(() => expect(screen.getByText('quota exceeded')).toBeInTheDocument());
  });
});

describe('TokensModal — copy', () => {
  it('Copy button writes the secret to the clipboard and flips its label', async () => {
    // happy-dom ships a real navigator.clipboard; spy on writeText so
    // we can assert the call and short-circuit the actual write.
    const writeSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue();
    try {
      const user = userEvent.setup();
      render(<TokensModal onClose={vi.fn()} />);
      await waitFor(() => expect(api.listTokens).toHaveBeenCalled());

      await user.type(screen.getByPlaceholderText(/Token name/i), 'ci-bot');
      await user.click(screen.getByRole('button', { name: /create token/i }));
      await waitFor(() => expect(screen.getByText('pt_xyz_secret')).toBeInTheDocument());

      await user.click(screen.getByRole('button', { name: /copy to clipboard/i }));
      expect(writeSpy).toHaveBeenCalledWith('pt_xyz_secret');
      await waitFor(() => expect(screen.getByText('Copied!')).toBeInTheDocument());
    } finally {
      writeSpy.mockRestore();
    }
  });
});

describe('TokensModal — revoke', () => {
  let originalConfirm;
  beforeEach(() => {
    originalConfirm = window.confirm;
  });
  afterEach(() => {
    window.confirm = originalConfirm;
  });

  it('cancelled confirm aborts the revoke', async () => {
    window.confirm = vi.fn().mockReturnValue(false);
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Revoke' }));
    expect(window.confirm).toHaveBeenCalled();
    expect(api.revokeToken).not.toHaveBeenCalled();
  });

  it('confirmed revoke calls api.revokeToken with the row id and reloads', async () => {
    window.confirm = vi.fn().mockReturnValue(true);
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(api.listTokens).toHaveBeenCalledOnce());

    await user.click(screen.getByRole('button', { name: 'Revoke' }));
    await waitFor(() => expect(api.revokeToken).toHaveBeenCalledWith(1));
    await waitFor(() => expect(api.listTokens).toHaveBeenCalledTimes(2));
  });

  it('renders the API error if revokeToken rejects', async () => {
    window.confirm = vi.fn().mockReturnValue(true);
    api.revokeToken.mockRejectedValueOnce(new Error('not found'));
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Revoke' }));
    await waitFor(() => expect(screen.getByText('not found')).toBeInTheDocument());
  });
});

describe('TokensModal — delete', () => {
  let originalConfirm;
  beforeEach(() => {
    originalConfirm = window.confirm;
  });
  afterEach(() => {
    window.confirm = originalConfirm;
  });

  it('every row gets a Delete button — even revoked ones', async () => {
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeInTheDocument());
    // Two rows in the fixture → two Delete buttons.
    expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(2);
  });

  it('cancelled confirm aborts the delete', async () => {
    window.confirm = vi.fn().mockReturnValue(false);
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeInTheDocument());

    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0]);
    expect(window.confirm).toHaveBeenCalled();
    expect(api.deleteToken).not.toHaveBeenCalled();
  });

  it('confirmed delete calls api.deleteToken with the row id and reloads', async () => {
    window.confirm = vi.fn().mockReturnValue(true);
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(api.listTokens).toHaveBeenCalledOnce());

    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0]);
    await waitFor(() => expect(api.deleteToken).toHaveBeenCalledWith(1));
    await waitFor(() => expect(api.listTokens).toHaveBeenCalledTimes(2));
  });

  it('renders the API error if deleteToken rejects', async () => {
    window.confirm = vi.fn().mockReturnValue(true);
    api.deleteToken.mockRejectedValueOnce(new Error('boom'));
    const user = userEvent.setup();
    render(<TokensModal onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeInTheDocument());

    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0]);
    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument());
  });
});
