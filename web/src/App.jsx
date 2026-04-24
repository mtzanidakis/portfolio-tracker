import { useState, useEffect } from 'preact/hooks';
import { Sidebar, Topbar } from './components/Shell.jsx';
import { Icon } from './components/Icons.jsx';
import { PerformancePage } from './components/PerformancePage.jsx';
import { AllocationsPage } from './components/AllocationsPage.jsx';
import { ActivitiesPage } from './components/ActivitiesPage.jsx';
import { AccountsPage } from './components/AccountsPage.jsx';
import { AssetsPage } from './components/AssetsPage.jsx';
import { ImportExportPage } from './components/ImportExportPage.jsx';
import { TxModal } from './components/TxModal.jsx';
import { LoginForm } from './components/LoginForm.jsx';
import { ProfileModal } from './components/ProfileModal.jsx';
import { SettingsModal } from './components/SettingsModal.jsx';
import { TokensModal } from './components/TokensModal.jsx';
import { setDateFormat } from './format.js';
import { api } from './api.js';

const TITLES = {
  performance: { t: 'Performance', s: 'Your portfolio over time' },
  allocations: { t: 'Allocations', s: 'Where your money is working' },
  activities:  { t: 'Activities',  s: 'Every buy, every sell' },
  accounts:    { t: 'Accounts',    s: 'Brokerages, exchanges, wallets' },
  assets:      { t: 'Assets',      s: 'Tickers you can trade or hold' },
  importexport:{ t: 'Import / Export', s: 'Bring data in, take backups out' },
};

export function App() {
  const [page, setPage] = useState(() => localStorage.getItem('pt-page') || 'performance');
  const [theme, setTheme] = useState(() => localStorage.getItem('pt-theme') || 'system');
  const [aesthetic, setAesthetic] = useState(() => localStorage.getItem('pt-aesthetic') || 'technical');
  // Seed format.js' module-level pattern synchronously so the very
  // first render of any page already formats dates the way the user
  // asked — otherwise we'd flash the browser-locale default.
  const [dateFormat, setDateFormatState] = useState(() => {
    const saved = localStorage.getItem('pt-dateformat') || 'browser';
    setDateFormat(saved);
    return saved;
  });
  const [privacy, setPrivacy] = useState(() => localStorage.getItem('pt-privacy') === '1');
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showAddTx, setShowAddTx] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [tokensOpen, setTokensOpen] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [refreshTick, setRefreshTick] = useState(0);
  // Seed state for cross-page navigation. Clicking an account card or
  // asset row routes to Activities with this id/symbol pre-selected;
  // the ActivitiesPage reads it once on mount and controls the filter
  // itself thereafter.
  const [activityAccountFilter, setActivityAccountFilter] = useState(0);
  const [activityAssetFilter, setActivityAssetFilter] = useState('');

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
  useEffect(() => {
    localStorage.setItem('pt-dateformat', dateFormat);
    setDateFormat(dateFormat);
    // Force a remount of the active page so all already-rendered
    // dates pick up the new pattern in place.
    setRefreshTick(t => t + 1);
  }, [dateFormat]);
  useEffect(() => { localStorage.setItem('pt-privacy', privacy ? '1' : '0'); }, [privacy]);

  // The account / asset filter seeds are one-shots handed to
  // ActivitiesPage on mount. Clear them as soon as the page becomes
  // active so a subsequent natural navigation (sidebar click) starts
  // unfiltered. The effect runs after the child's useState initializer
  // has already captured the current value, so the filter still
  // applies on this mount.
  useEffect(() => {
    if (page !== 'activities') return;
    if (activityAccountFilter !== 0) setActivityAccountFilter(0);
    if (activityAssetFilter !== '') setActivityAssetFilter('');
  }, [page, activityAccountFilter, activityAssetFilter]);

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
  const pageProps = { privacy, currency };

  const openAccountActivity = (accountId) => {
    setActivityAccountFilter(accountId);
    setActivityAssetFilter('');
    setPage('activities');
    setRefreshTick(t => t + 1); // force remount so the initial filter takes
  };

  const openAssetActivity = (symbol) => {
    setActivityAssetFilter(symbol);
    setActivityAccountFilter(0);
    setPage('activities');
    setRefreshTick(t => t + 1);
  };

  return (
    <div class="app" data-screen-label={page}>
      <Sidebar
        page={page} setPage={setPage} user={user}
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        onProfile={() => setProfileOpen(true)}
        onSettings={() => setSettingsOpen(true)}
        onTokens={() => setTokensOpen(true)}
        onSignOut={signOut}
      />
      <main class="main">
        <Topbar title={TITLES[page].t} sub={TITLES[page].s} actions={topActions}
          onMenuClick={() => setSidebarOpen(true)} />
        <div class="content">
          {/* key forces a remount after any cross-page action (Add
            * transaction from the topbar) so each page re-fetches
            * from scratch. Preact doesn't pick up `key` from a
            * spread — it has to be on the element directly. */}
          {page === 'performance' && <PerformancePage key={refreshTick} {...pageProps} />}
          {page === 'allocations' && <AllocationsPage key={refreshTick} {...pageProps} />}
          {page === 'activities'  && <ActivitiesPage  key={refreshTick} {...pageProps} user={user} initialAccountId={activityAccountFilter} initialAssetSymbol={activityAssetFilter} />}
          {page === 'accounts'    && <AccountsPage    key={refreshTick} {...pageProps} onOpenActivity={openAccountActivity} />}
          {page === 'assets'      && <AssetsPage      key={refreshTick} {...pageProps} onOpenActivity={openAssetActivity} />}
          {page === 'importexport'&& <ImportExportPage key={refreshTick} />}
        </div>
      </main>

      {showAddTx && (
        <TxModal user={user}
          onClose={() => setShowAddTx(false)}
          onSaved={() => setRefreshTick(t => t + 1)} />
      )}
      {profileOpen && (
        <ProfileModal user={user}
          onSaved={(u) => setUser(u)}
          onClose={() => setProfileOpen(false)} />
      )}
      {settingsOpen && (
        <SettingsModal user={user}
          aesthetic={aesthetic}
          setAesthetic={setAesthetic}
          dateFormat={dateFormat}
          setDateFormat={setDateFormatState}
          onSaved={(u) => setUser(u)}
          onClose={() => setSettingsOpen(false)} />
      )}
      {tokensOpen && (
        <TokensModal onClose={() => setTokensOpen(false)} />
      )}
    </div>
  );
}
