import { useState } from 'preact/hooks';
import { Icon } from './Icons.jsx';
import { UserMenu } from './UserMenu.jsx';
import { fmtMoney } from '../format.js';

export function Sidebar({ page, setPage, user, open, onClose, onProfile, onSettings, onTokens, onSignOut }) {
  const [menuOpen, setMenuOpen] = useState(false);
  const items = [
    { id: 'performance', label: 'Performance', icon: 'chart' },
    { id: 'allocations', label: 'Allocations', icon: 'pie' },
    { id: 'activities',  label: 'Activities',  icon: 'activity' },
    { id: 'accounts',    label: 'Accounts',    icon: 'wallet' },
    { id: 'assets',      label: 'Assets',      icon: 'coins' },
  ];
  const initials = (user?.name || '?').split(/\s+/).map(w => w[0]).slice(0, 2).join('').toUpperCase();
  // On mobile the sidebar slides in as a drawer; picking a nav item
  // auto-closes it so the user doesn't need to dismiss manually.
  const go = (id) => { setPage(id); if (onClose) onClose(); };
  return (
    <>
      <div class={`sidebar-backdrop ${open ? 'on' : ''}`} onClick={onClose} />
      <aside class={`sidebar ${open ? 'open' : ''}`}>
        <div class="brand">
          <div class="brand-mark" />
          <div class="brand-name">Portfolio</div>
        </div>
        <div class="nav">
          <div class="nav-label">Portfolio</div>
          {items.map(it => (
            <button key={it.id}
              class={`nav-item ${page === it.id ? 'active' : ''}`}
              onClick={() => go(it.id)}>
              <Icon name={it.icon} />
              {it.label}
            </button>
          ))}
        </div>
        <div class="sidebar-footer" style={{ position: 'relative' }}>
          <button
            class="user-chip"
            type="button"
            onClick={() => setMenuOpen(o => !o)}
            style={{
              background: 'transparent', border: 'none', padding: 0,
              width: '100%', textAlign: 'left', cursor: 'pointer',
              display: 'flex', alignItems: 'center', gap: 10,
            }}
            aria-haspopup="menu"
            aria-expanded={menuOpen}
          >
            <div class="avatar">{initials}</div>
            <div class="user-chip-meta">
              <div class="n">{user?.name || 'Unknown'}</div>
              <div class="e">{user?.email || ''}</div>
            </div>
          </button>
          {menuOpen && (
            <UserMenu
              onProfile={onProfile}
              onSettings={onSettings}
              onTokens={onTokens}
              onSignOut={onSignOut}
              onClose={() => setMenuOpen(false)}
            />
          )}
        </div>
      </aside>
    </>
  );
}

export function Topbar({ title, sub, actions, onMenuClick }) {
  return (
    <div class="topbar">
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
        {onMenuClick && (
          <button class="icon-btn menu-toggle" aria-label="Open menu" onClick={onMenuClick}>
            <Icon name="menu" />
          </button>
        )}
        <div style={{ minWidth: 0 }}>
          <h1 class="page-title">{title}</h1>
          {sub && <div class="page-sub">{sub}</div>}
        </div>
      </div>
      <div class="top-actions">{actions}</div>
    </div>
  );
}

export function Money({ value, privacy, currency, sign = false, decimals }) {
  const text = fmtMoney(value, currency, { sign, decimals });
  if (privacy) return <span class="masked">{text}</span>;
  return <span>{text}</span>;
}
