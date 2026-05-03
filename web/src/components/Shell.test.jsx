import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';
import { Sidebar, Topbar, Money } from './Shell.jsx';

const SAMPLE_USER = { name: 'Alex Rivera', email: 'alex@x.io' };

function renderSidebar(overrides = {}) {
  const props = {
    page: 'performance',
    setPage: vi.fn(),
    user: SAMPLE_USER,
    open: false,
    onClose: vi.fn(),
    onProfile: vi.fn(),
    onSettings: vi.fn(),
    onTokens: vi.fn(),
    onSignOut: vi.fn(),
    ...overrides,
  };
  return { props, ...render(<Sidebar {...props} />) };
}

describe('Sidebar', () => {
  it('renders all six nav items', () => {
    renderSidebar();
    for (const label of ['Performance', 'Allocations', 'Accounts',
      'Assets', 'Activities', 'Import / Export']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it('marks the active item with .active', () => {
    const { container } = renderSidebar({ page: 'accounts' });
    const active = container.querySelector('.nav-item.active');
    expect(active.textContent).toContain('Accounts');
  });

  it('clicking a nav item calls setPage and onClose', async () => {
    const user = userEvent.setup();
    const { props } = renderSidebar();
    await user.click(screen.getByText('Allocations'));
    expect(props.setPage).toHaveBeenCalledWith('allocations');
    expect(props.onClose).toHaveBeenCalledOnce();
  });

  it('user chip toggles the UserMenu', async () => {
    const user = userEvent.setup();
    renderSidebar();
    expect(screen.queryByText('Profile')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText('Profile')).toBeInTheDocument();
    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('API tokens')).toBeInTheDocument();
    expect(screen.getByText('Sign out')).toBeInTheDocument();
  });

  it('renders avatar initials from the user name', () => {
    const { container } = renderSidebar({ user: { name: 'Maria Schwarz', email: 'm@s' } });
    expect(container.querySelector('.avatar').textContent).toBe('MS');
  });

  it('falls back to ? for missing user', () => {
    const { container } = renderSidebar({ user: null });
    expect(container.querySelector('.avatar').textContent).toBe('?');
  });

  it('exposes the open class on the aside when open', () => {
    const { container } = renderSidebar({ open: true });
    expect(container.querySelector('aside.sidebar').className).toContain('open');
  });

  it('clicking the backdrop fires onClose', async () => {
    const user = userEvent.setup();
    const { container, props } = renderSidebar({ open: true });
    await user.click(container.querySelector('.sidebar-backdrop'));
    expect(props.onClose).toHaveBeenCalledOnce();
  });

  it('omits the collapse button when onToggleCollapse is missing', () => {
    renderSidebar();
    expect(screen.queryByRole('button', { name: /collapse sidebar|expand sidebar/i }))
      .not.toBeInTheDocument();
  });

  it('shows a "Collapse sidebar" button when expanded and toggles on click', async () => {
    const user = userEvent.setup();
    const onToggleCollapse = vi.fn();
    renderSidebar({ collapsed: false, onToggleCollapse });
    const btn = screen.getByRole('button', { name: /collapse sidebar/i });
    await user.click(btn);
    expect(onToggleCollapse).toHaveBeenCalledOnce();
  });

  it('shows an "Expand sidebar" button and a .collapsed aside when collapsed', () => {
    const { container } = renderSidebar({ collapsed: true, onToggleCollapse: vi.fn() });
    expect(screen.getByRole('button', { name: /expand sidebar/i })).toBeInTheDocument();
    expect(container.querySelector('aside.sidebar').className).toContain('collapsed');
  });

  it('adds title tooltips to nav items only when collapsed', () => {
    const { container, rerender } = render(
      <Sidebar page="performance" setPage={vi.fn()} user={SAMPLE_USER}
        open={false} collapsed onToggleCollapse={vi.fn()}
        onClose={vi.fn()} onProfile={vi.fn()} onSettings={vi.fn()}
        onTokens={vi.fn()} onSignOut={vi.fn()} />,
    );
    const navWhenCollapsed = container.querySelector('.nav-item');
    expect(navWhenCollapsed.getAttribute('title')).toBe('Performance');

    rerender(
      <Sidebar page="performance" setPage={vi.fn()} user={SAMPLE_USER}
        open={false} collapsed={false} onToggleCollapse={vi.fn()}
        onClose={vi.fn()} onProfile={vi.fn()} onSettings={vi.fn()}
        onTokens={vi.fn()} onSignOut={vi.fn()} />,
    );
    const navWhenExpanded = container.querySelector('.nav-item');
    // Preact renders an empty string for title={undefined}; what
    // matters is that no real tooltip is shown.
    expect(navWhenExpanded.getAttribute('title') || '').toBe('');
  });
});

describe('Topbar', () => {
  it('renders title + sub + actions', () => {
    render(
      <Topbar title="Performance" sub="Your portfolio over time"
        actions={<button>Add</button>} />,
    );
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Performance');
    expect(screen.getByText('Your portfolio over time')).toBeInTheDocument();
    expect(screen.getByText('Add')).toBeInTheDocument();
  });

  it('omits the menu toggle when onMenuClick is missing', () => {
    render(<Topbar title="X" />);
    expect(screen.queryByRole('button', { name: /open menu/i })).not.toBeInTheDocument();
  });

  it('menu toggle calls onMenuClick', async () => {
    const user = userEvent.setup();
    const onMenuClick = vi.fn();
    render(<Topbar title="X" onMenuClick={onMenuClick} />);
    await user.click(screen.getByRole('button', { name: /open menu/i }));
    expect(onMenuClick).toHaveBeenCalledOnce();
  });
});

describe('Money', () => {
  it('renders a plain span with the formatted value', () => {
    const { container } = render(<Money value={1234} currency="USD" />);
    const span = container.querySelector('span');
    expect(span.className).toBe('');
    expect(span.textContent).toBe('$1,234.00');
  });

  it('wraps in .masked when privacy is on', () => {
    const { container } = render(<Money value={1234} currency="USD" privacy />);
    const masked = container.querySelector('span.masked');
    expect(masked).not.toBeNull();
    expect(masked.textContent).toBe('$1,234.00');
  });

  it('forwards sign and decimals to fmtMoney', () => {
    const { container } = render(
      <Money value={50} currency="USD" sign decimals={0} />,
    );
    expect(container.textContent).toBe('+$50');
  });
});
