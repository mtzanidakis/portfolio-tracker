import { useEffect, useRef } from 'preact/hooks';

export function UserMenu({ onProfile, onSettings, onTokens, onSignOut, onClose }) {
  const ref = useRef(null);

  useEffect(() => {
    const onDocClick = (e) => {
      if (!ref.current) return;
      if (!ref.current.contains(e.target)) onClose();
    };
    const onKey = (e) => { if (e.key === 'Escape') onClose(); };
    // Delay attaching to avoid catching the click that opened us.
    const tid = setTimeout(() => {
      document.addEventListener('mousedown', onDocClick);
      document.addEventListener('keydown', onKey);
    }, 0);
    return () => {
      clearTimeout(tid);
      document.removeEventListener('mousedown', onDocClick);
      document.removeEventListener('keydown', onKey);
    };
  }, [onClose]);

  const item = (label, fn, extraStyle = {}) => (
    <button
      onClick={() => { fn(); onClose(); }}
      style={{
        display: 'block', width: '100%', textAlign: 'left',
        padding: '10px 14px', border: 'none', background: 'transparent',
        fontSize: 13, color: 'var(--text)', cursor: 'pointer',
        ...extraStyle,
      }}
      onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--bg-hover)'; }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; }}
    >{label}</button>
  );

  return (
    <div ref={ref} class="user-menu" style={{
      position: 'absolute',
      bottom: 'calc(100% + 8px)',
      left: 12, right: 12,
      background: 'var(--bg-elev)',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-sm)',
      boxShadow: '0 6px 24px rgba(0,0,0,0.14)',
      zIndex: 100,
      overflow: 'hidden',
    }}>
      {item('Profile', onProfile)}
      {item('Settings', onSettings)}
      {item('API tokens', onTokens)}
      <div style={{ height: 1, background: 'var(--border)' }} />
      {item('Sign out', onSignOut, { color: 'var(--neg)' })}
    </div>
  );
}
