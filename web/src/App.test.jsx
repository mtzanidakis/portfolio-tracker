import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

// Stub every child page + modal so App's own logic — auth gating,
// page routing, persisted preferences, modal toggles — is what's
// under test. Each stub renders a recognisable marker so we can
// assert it appears.
vi.mock('./components/PerformancePage.jsx', () => ({
  PerformancePage: (p) => <div data-testid="page-performance" data-currency={p.currency} />,
}));
vi.mock('./components/AllocationsPage.jsx', () => ({
  AllocationsPage: () => <div data-testid="page-allocations" />,
}));
vi.mock('./components/ActivitiesPage.jsx', () => ({
  ActivitiesPage: (p) => (
    <div data-testid="page-activities"
      data-account-filter={p.initialAccountId}
      data-asset-filter={p.initialAssetSymbol} />
  ),
}));
vi.mock('./components/AccountsPage.jsx', () => ({
  AccountsPage: ({ onOpenActivity }) => (
    <div data-testid="page-accounts">
      <button onClick={() => onOpenActivity(7)}>open-acc-7</button>
    </div>
  ),
}));
vi.mock('./components/AssetsPage.jsx', () => ({
  AssetsPage: ({ onOpenActivity }) => (
    <div data-testid="page-assets">
      <button onClick={() => onOpenActivity('AAPL')}>open-aapl</button>
    </div>
  ),
}));
vi.mock('./components/ImportExportPage.jsx', () => ({
  ImportExportPage: () => <div data-testid="page-importexport" />,
}));
vi.mock('./components/TxModal.jsx', () => ({
  TxModal: ({ onClose }) => (
    <div data-testid="modal-addtx">
      <button onClick={onClose}>close-tx</button>
    </div>
  ),
}));
vi.mock('./components/ProfileModal.jsx', () => ({
  ProfileModal: ({ onClose }) => (
    <div data-testid="modal-profile">
      <button onClick={onClose}>close-profile</button>
    </div>
  ),
}));
vi.mock('./components/SettingsModal.jsx', () => ({
  SettingsModal: ({ onClose }) => (
    <div data-testid="modal-settings">
      <button onClick={onClose}>close-settings</button>
    </div>
  ),
}));
vi.mock('./components/TokensModal.jsx', () => ({
  TokensModal: ({ onClose }) => (
    <div data-testid="modal-tokens">
      <button onClick={onClose}>close-tokens</button>
    </div>
  ),
}));
vi.mock('./components/LoginForm.jsx', () => ({
  LoginForm: ({ onLoggedIn }) => (
    <button data-testid="login" onClick={() => onLoggedIn({ id: 1, name: 'Me', email: 'me@x', base_currency: 'USD' })}>
      login
    </button>
  ),
}));

vi.mock('./api.js', () => ({
  api: {
    me:     vi.fn(),
    logout: vi.fn(),
  },
}));

import { api } from './api.js';
import { App } from './App.jsx';

const ME = { id: 1, name: 'Alex Rivera', email: 'alex@x', base_currency: 'EUR' };

beforeEach(() => {
  vi.clearAllMocks();
  // Default: authenticated.
  api.me.mockResolvedValue(ME);
  api.logout.mockResolvedValue(null);
  // happy-dom doesn't expose matchMedia by default; provide a stub
  // since App reads it for the theme heuristic.
  window.matchMedia = vi.fn().mockReturnValue({ matches: false });
});

describe('App — auth gating', () => {
  it('shows the loading marker until api.me() resolves', async () => {
    let resolveMe;
    api.me.mockReturnValueOnce(new Promise((r) => { resolveMe = r; }));
    render(<App />);
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
    resolveMe(ME);
    await waitFor(() => expect(screen.queryByText(/loading/i)).not.toBeInTheDocument());
  });

  it('shows LoginForm when api.me() rejects (401)', async () => {
    api.me.mockRejectedValueOnce(Object.assign(new Error('unauthorized'), { status: 401 }));
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('login')).toBeInTheDocument());
  });

  it('login → app shell mounted', async () => {
    api.me.mockRejectedValueOnce(new Error('401'));
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('login')).toBeInTheDocument());
    await user.click(screen.getByTestId('login'));
    // Default page is performance; the stub picks up the user's currency.
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());
  });

  it('signing out from the user menu calls api.logout and re-renders LoginForm', async () => {
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());

    // Open the user menu (sidebar footer button).
    const userChip = screen.getByRole('button', { name: /alex rivera/i });
    await user.click(userChip);
    await user.click(screen.getByText('Sign out'));

    await waitFor(() => expect(api.logout).toHaveBeenCalledOnce());
    await waitFor(() => expect(screen.getByTestId('login')).toBeInTheDocument());
  });
});

describe('App — routing + persistence', () => {
  it('clicking a sidebar nav item swaps the page', async () => {
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /allocations/i }));
    await waitFor(() => expect(screen.getByTestId('page-allocations')).toBeInTheDocument());
    expect(screen.queryByTestId('page-performance')).not.toBeInTheDocument();
  });

  it('persists the active page to localStorage', async () => {
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /assets/i }));
    await waitFor(() => expect(localStorage.getItem('pt-page')).toBe('assets'));
  });

  it('reloads on the persisted page from a previous session', async () => {
    localStorage.setItem('pt-page', 'accounts');
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-accounts')).toBeInTheDocument());
  });
});

describe('App — cross-page filter seeding', () => {
  it('AccountsPage onOpenActivity routes to Activities with the account preselected', async () => {
    const user = userEvent.setup();
    localStorage.setItem('pt-page', 'accounts');
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-accounts')).toBeInTheDocument());

    await user.click(screen.getByText('open-acc-7'));
    const activities = await screen.findByTestId('page-activities');
    expect(activities.dataset.accountFilter).toBe('7');
    expect(activities.dataset.assetFilter).toBe('');
  });

  it('AssetsPage onOpenActivity routes to Activities with the asset preselected', async () => {
    const user = userEvent.setup();
    localStorage.setItem('pt-page', 'assets');
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-assets')).toBeInTheDocument());

    await user.click(screen.getByText('open-aapl'));
    const activities = await screen.findByTestId('page-activities');
    expect(activities.dataset.assetFilter).toBe('AAPL');
    expect(activities.dataset.accountFilter).toBe('0');
  });
});

describe('App — privacy + theme toggles', () => {
  it('privacy toggle persists to localStorage', async () => {
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());

    expect(localStorage.getItem('pt-privacy')).toBe('0');
    await user.click(screen.getByRole('button', { name: /toggle privacy/i }));
    await waitFor(() => expect(localStorage.getItem('pt-privacy')).toBe('1'));
  });

  it('theme toggle writes data-theme attribute on <html>', async () => {
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());

    // Initial: theme=system, matchMedia returns matches=false → light.
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');

    // Click toggles to dark.
    await user.click(screen.getByRole('button', { name: /theme/i }));
    await waitFor(() => expect(document.documentElement.getAttribute('data-theme')).toBe('dark'));
  });
});

describe('App — modal lifecycle', () => {
  it('opens and closes the Add transaction modal', async () => {
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /add transaction/i }));
    expect(screen.getByTestId('modal-addtx')).toBeInTheDocument();

    await user.click(screen.getByText('close-tx'));
    await waitFor(() => expect(screen.queryByTestId('modal-addtx')).not.toBeInTheDocument());
  });

  it.each([
    ['Profile',    'modal-profile',  'close-profile'],
    ['Settings',   'modal-settings', 'close-settings'],
    ['API tokens', 'modal-tokens',   'close-tokens'],
  ])('user menu → %s opens then closes %s', async (label, testId, closeText) => {
    const user = userEvent.setup();
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('page-performance')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /alex rivera/i }));
    await user.click(screen.getByText(label));
    expect(screen.getByTestId(testId)).toBeInTheDocument();

    await user.click(screen.getByText(closeText));
    await waitFor(() => expect(screen.queryByTestId(testId)).not.toBeInTheDocument());
  });
});
