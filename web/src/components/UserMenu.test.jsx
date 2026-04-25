import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';
import { UserMenu } from './UserMenu.jsx';

function setup() {
  const handlers = {
    onProfile: vi.fn(),
    onSettings: vi.fn(),
    onTokens: vi.fn(),
    onSignOut: vi.fn(),
    onClose: vi.fn(),
  };
  const utils = render(<UserMenu {...handlers} />);
  return { ...handlers, ...utils };
}

describe('UserMenu', () => {
  it('renders all four actions', () => {
    setup();
    expect(screen.getByText('Profile')).toBeInTheDocument();
    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('API tokens')).toBeInTheDocument();
    expect(screen.getByText('Sign out')).toBeInTheDocument();
  });

  it.each([
    ['Profile',    'onProfile'],
    ['Settings',   'onSettings'],
    ['API tokens', 'onTokens'],
    ['Sign out',   'onSignOut'],
  ])('%s click fires %s and onClose', async (label, key) => {
    const user = userEvent.setup();
    const ctx = setup();
    await user.click(screen.getByText(label));
    expect(ctx[key]).toHaveBeenCalledOnce();
    expect(ctx.onClose).toHaveBeenCalledOnce();
  });

  it('Escape closes the menu', async () => {
    const { onClose } = setup();
    await new Promise((r) => setTimeout(r, 5));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('mousedown outside closes the menu', async () => {
    const { onClose } = setup();
    await new Promise((r) => setTimeout(r, 5));
    fireEvent.mouseDown(document.body);
    expect(onClose).toHaveBeenCalledOnce();
  });
});
