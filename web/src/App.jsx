import { useState, useEffect } from 'preact/hooks';
import { Sidebar, Topbar } from './components/Shell.jsx';
import { Icon } from './components/Icons.jsx';
import { PerformancePage } from './components/PerformancePage.jsx';
import { AllocationsPage } from './components/AllocationsPage.jsx';
import { ActivitiesPage } from './components/ActivitiesPage.jsx';
import { AccountsPage } from './components/AccountsPage.jsx';
import { AssetsPage } from './components/AssetsPage.jsx';
import { TxModal } from './components/TxModal.jsx';
import { LoginForm } from './components/LoginForm.jsx';
import { ProfileModal } from './components/ProfileModal.jsx';
import { TokensModal } from './components/TokensModal.jsx';
import { api } from './api.js';

const TITLES = {
  performance: { t: 'Performance', s: 'Your portfolio over time' },
  allocations: { t: 'Allocations', s: 'Where your money is working' },
  activities:  { t: 'Activities',  s: 'Every buy, every sell' },
  accounts:    { t: 'Accounts',    s: 'Brokerages, exchanges, wallets' },
  assets:      { t: 'Assets',      s: 'Tickers you can trade or hold' },
};

export function App() {
  const [page, setPage] = useState(() => localStorage.getItem('pt-page') || 'performance');
  const [theme, setTheme] = useState(() => localStorage.getItem('pt-theme') || 'system');
  const [aesthetic, setAesthetic] = useState(() => localStorage.getItem('pt-aesthetic') || 'technical');
  const [privacy, setPrivacy] = useState(() => localStorage.getItem('pt-privacy') === '1');
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showAddTx, setShowAddTx] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const [tokensOpen, setTokensOpen] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [refreshTick, setRefreshTick] = useState(0);

  useEffect(() => { localStorage.setItem('pt-page', page); }, [page]);
  useEffect(() => {
    localStorage.setItem('pt-theme', theme);
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const applied = theme === 'system' ? (prefersDark ? 'dark' : 'light') : theme;
    document.documentElement.setAttribute('data-theme', applied);
  }, [theme]);
  useEffect(() => {
    localStorage.setItem('pt-aesthetic', aesthetic);
    document.documentElement.setAttribute('data-aesthetic', aesthetic);
  }, [aesthetic]);
  useEffect(() => { localStorage.setItem('pt-privacy', privacy ? '1' : '0'); }, [privacy]);

  // Attempt to resolve the current user on first mount. 401 → show login.
  useEffect(() => {
    api.me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  const signOut = async () => {
    try { await api.logout(); } catch { /* ignore */ }
    setUser(null);
  };

  if (loading) {
    return <div class="empty" style={{ padding: 48 }}>Loading…</div>;
  }
  if (!user) {
    return <LoginForm onLoggedIn={(u) => setUser(u)} />;
  }

  const appliedTheme = theme === 'system'
    ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    : theme;

  const topActions = (
    <>
      <button class="icon-btn" title="Toggle privacy" onClick={() => setPrivacy(p => !p)}>
        <Icon name={privacy ? 'eyeOff' : 'eye'} />
      </button>
      <button class="icon-btn" title={`Theme: ${theme}`}
        onClick={() => setTheme(appliedTheme === 'dark' ? 'light' : 'dark')}>
        <Icon name={appliedTheme === 'dark' ? 'sun' : 'moon'} />
      </button>
      <button class="btn primary btn-add-tx" onClick={() => setShowAddTx(true)}
        aria-label="Add transaction" title="Add transaction">
        <Icon name="plus" />
        <span class="label">Add transaction</span>
      </button>
    </>
  );

  const currency = user.base_currency;
  const pageProps = { privacy, currency, key: refreshTick };

  return (
    <div class="app" data-screen-label={page}>
      <Sidebar
        page={page} setPage={setPage} user={user}
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        onProfile={() => setProfileOpen(true)}
        onTokens={() => setTokensOpen(true)}
        onSignOut={signOut}
      />
      <main class="main">
        <Topbar title={TITLES[page].t} sub={TITLES[page].s} actions={topActions}
          onMenuClick={() => setSidebarOpen(true)} />
        <div class="content">
          {page === 'performance' && <PerformancePage {...pageProps} />}
          {page === 'allocations' && <AllocationsPage {...pageProps} />}
          {page === 'activities'  && <ActivitiesPage  {...pageProps} user={user} />}
          {page === 'accounts'    && <AccountsPage    {...pageProps} />}
          {page === 'assets'      && <AssetsPage      {...pageProps} />}
        </div>
      </main>

      {showAddTx && (
        <TxModal user={user}
          onClose={() => setShowAddTx(false)}
          onSaved={() => setRefreshTick(t => t + 1)} />
      )}
      {profileOpen && (
        <ProfileModal user={user}
          aesthetic={aesthetic}
          setAesthetic={setAesthetic}
          onSaved={(u) => setUser(u)}
          onClose={() => setProfileOpen(false)} />
      )}
      {tokensOpen && (
        <TokensModal onClose={() => setTokensOpen(false)} />
      )}
    </div>
  );
}
