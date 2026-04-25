import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';
import { AccountCardMenu } from './AccountCardMenu.jsx';

function setup() {
  const onEdit = vi.fn();
  const onDelete = vi.fn();
  const onClose = vi.fn();
  const utils = render(
    <AccountCardMenu onEdit={onEdit} onDelete={onDelete} onClose={onClose} />,
  );
  return { onEdit, onDelete, onClose, ...utils };
}

describe('AccountCardMenu', () => {
  it('renders Edit and Delete items', () => {
    setup();
    expect(screen.getByText('Edit')).toBeInTheDocument();
    expect(screen.getByText('Delete')).toBeInTheDocument();
  });

  it('clicking Edit calls onEdit then onClose', async () => {
    const user = userEvent.setup();
    const { onEdit, onDelete, onClose } = setup();
    await user.click(screen.getByText('Edit'));
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
    expect(onDelete).not.toHaveBeenCalled();
  });

  it('clicking Delete calls onDelete then onClose', async () => {
    const user = userEvent.setup();
    const { onEdit, onDelete, onClose } = setup();
    await user.click(screen.getByText('Delete'));
    expect(onDelete).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
    expect(onEdit).not.toHaveBeenCalled();
  });

  it('Escape closes the menu', async () => {
    const { onClose } = setup();
    // The mousedown/keydown listeners attach inside a setTimeout(0)
    // so the click that opened the menu can't immediately close it.
    // Wait a microtask to let that fire.
    await new Promise((r) => setTimeout(r, 5));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('mousedown outside the menu closes it', async () => {
    const { onClose } = setup();
    await new Promise((r) => setTimeout(r, 5));
    fireEvent.mouseDown(document.body);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('mousedown inside the menu does not close it', async () => {
    const { onClose } = setup();
    await new Promise((r) => setTimeout(r, 5));
    fireEvent.mouseDown(screen.getByText('Edit'));
    expect(onClose).not.toHaveBeenCalled();
  });
});
