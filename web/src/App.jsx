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
  const [tweaksOpen, setTweaksOpen] = useState(false);
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
      <button class="icon-btn" onClick={() => setTweaksOpen(o => !o)}>
        <Icon name="bell" />
      </button>
      <button class="btn primary" onClick={() => setShowAddTx(true)}>
        <Icon name="plus" /> Add transaction
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
          onSaved={(u) => setUser(u)}
          onClose={() => setProfileOpen(false)} />
      )}
      {tokensOpen && (
        <TokensModal onClose={() => setTokensOpen(false)} />
      )}

      <div class={`tweaks-panel ${tweaksOpen ? 'on' : ''}`}>
        <h3 class="tweaks-title">Tweaks</h3>

        <div style={{ marginBottom: 10 }}>
          <div style={{ fontSize: 11, color: 'var(--text-faint)', textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: 6 }}>Aesthetic</div>
          <div style={{ display: 'grid', gap: 6 }}>
            {[
              { id: 'technical', label: 'Technical', sub: 'Slate + electric blue' },
              { id: 'editorial', label: 'Editorial', sub: 'Neutral paper + red' },
              { id: 'forest',    label: 'Forest',    sub: 'Cool green + slate' },
            ].map(opt => (
              <button key={opt.id} onClick={() => setAesthetic(opt.id)}
                style={{
                  textAlign: 'left', padding: '8px 10px',
                  border: `1px solid ${aesthetic === opt.id ? 'var(--terra)' : 'var(--border)'}`,
                  background: aesthetic === opt.id ? 'var(--terra-wash)' : 'var(--bg-sunken)',
                  borderRadius: 'var(--radius-sm)',
                  color: aesthetic === opt.id ? 'var(--terra)' : 'var(--text)',
                  display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                }}>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 500 }}>{opt.label}</div>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 1 }}>{opt.sub}</div>
                </div>
                {aesthetic === opt.id && <Icon name="check" size={14} />}
              </button>
            ))}
          </div>
        </div>

        <div class="tweak-row">
          <span>Theme</span>
          <div class="timeframe">
            {['light', 'dark', 'system'].map(m => (
              <button key={m} class={theme === m ? 'active' : ''} onClick={() => setTheme(m)}>{m}</button>
            ))}
          </div>
        </div>

        <div class="tweak-row">
          <span>Privacy mode</span>
          <button class={`switch ${privacy ? 'on' : ''}`} onClick={() => setPrivacy(p => !p)} />
        </div>
      </div>
    </div>
  );
}
