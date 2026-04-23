import { useEffect, useRef } from 'preact/hooks';

// Small popover menu used on the ... button of each account card.
export function AccountCardMenu({ onEdit, onDelete, onClose }) {
  const ref = useRef(null);

  useEffect(() => {
    const onDocClick = (e) => {
      if (!ref.current) return;
      if (!ref.current.contains(e.target)) onClose();
    };
    const onKey = (e) => { if (e.key === 'Escape') onClose(); };
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
      onClick={(e) => { e.stopPropagation(); fn(); onClose(); }}
      style={{
        display: 'block', width: '100%', textAlign: 'left',
        padding: '8px 12px', border: 'none', background: 'transparent',
        fontSize: 13, color: 'var(--text)', cursor: 'pointer',
        ...extraStyle,
      }}
      onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--bg-hover)'; }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; }}
    >{label}</button>
  );

  return (
    <div ref={ref} onClick={(e) => e.stopPropagation()} style={{
      position: 'absolute',
      right: 8, top: 'calc(100% + 2px)',
      minWidth: 140,
      background: 'var(--bg-elev)',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-sm)',
      boxShadow: '0 6px 24px rgba(0,0,0,0.14)',
      zIndex: 10,
      overflow: 'hidden',
    }}>
      {item('Edit', onEdit)}
      <div style={{ height: 1, background: 'var(--border)' }} />
      {item('Delete', onDelete, { color: 'var(--neg)' })}
    </div>
  );
}
