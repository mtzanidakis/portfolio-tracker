import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    updateMe:       vi.fn(),
    changePassword: vi.fn(),
  },
}));

import { api } from '../api.js';
import { ProfileModal } from './ProfileModal.jsx';

const USER = { name: 'Alex', email: 'alex@x', base_currency: 'EUR' };

beforeEach(() => {
  vi.clearAllMocks();
  api.updateMe.mockResolvedValue({ ...USER });
  api.changePassword.mockResolvedValue(null);
});

function fieldByLabel(container, labelText) {
  for (const f of container.querySelectorAll('.field')) {
    const label = f.querySelector('label');
    if (label && label.textContent.includes(labelText)) {
      return f.querySelector('input, select');
    }
  }
  return null;
}

describe('ProfileModal — profile save', () => {
  it('does not call updateMe when no fields changed and reports "No changes"', async () => {
    const user = userEvent.setup();
    render(<ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />);

    await user.click(screen.getByRole('button', { name: /save profile/i }));
    await waitFor(() => expect(screen.getByText(/no changes/i)).toBeInTheDocument());
    expect(api.updateMe).not.toHaveBeenCalled();
  });

  it('sends only the changed fields when name is edited', async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={onSaved} />,
    );
    const nameField = fieldByLabel(container, 'Name');
    await user.clear(nameField);
    await user.type(nameField, 'Alexandra');

    await user.click(screen.getByRole('button', { name: /save profile/i }));
    await waitFor(() => expect(api.updateMe).toHaveBeenCalledOnce());
    expect(api.updateMe).toHaveBeenCalledWith({ name: 'Alexandra' });
    expect(onSaved).toHaveBeenCalled();
    await waitFor(() => expect(screen.getByText('Saved.')).toBeInTheDocument());
  });

  it('sends both name and email when both changed', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    const nameField = fieldByLabel(container, 'Name');
    const emailField = fieldByLabel(container, 'Email');
    await user.clear(nameField); await user.type(nameField, 'Alexandra');
    await user.clear(emailField); await user.type(emailField, 'a@y');

    await user.click(screen.getByRole('button', { name: /save profile/i }));
    await waitFor(() => expect(api.updateMe).toHaveBeenCalledWith({
      name: 'Alexandra', email: 'a@y',
    }));
  });

  it('renders the API error from updateMe', async () => {
    api.updateMe.mockRejectedValueOnce(new Error('email already taken'));
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    const emailField = fieldByLabel(container, 'Email');
    await user.clear(emailField); await user.type(emailField, 'taken@x');
    await user.click(screen.getByRole('button', { name: /save profile/i }));
    await waitFor(() => expect(screen.getByText(/email already taken/i)).toBeInTheDocument());
  });
});

describe('ProfileModal — password change', () => {
  it('keeps Change password disabled until current + new ≥ 8 + match', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    const btn = screen.getByRole('button', { name: /change password/i });
    expect(btn).toBeDisabled();

    await user.type(fieldByLabel(container, 'Current password'), 'oldpassword');
    expect(btn).toBeDisabled(); // no new yet

    await user.type(fieldByLabel(container, 'New password'), 'short');
    expect(btn).toBeDisabled(); // <8

    await user.type(fieldByLabel(container, 'New password'), 'enough');
    // 11 chars total now; confirm still empty.
    expect(btn).toBeDisabled();

    await user.type(fieldByLabel(container, 'Confirm new password'), 'shortenough');
    expect(btn).toBeEnabled();
  });

  it('rejects mismatched confirmation with an inline error', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Current password'), 'oldpassword');
    await user.type(fieldByLabel(container, 'New password'), 'newpassword');
    await user.type(fieldByLabel(container, 'Confirm new password'), 'newpasswordX');
    // Buttons are disabled because of mismatch — submit via the form
    // to bypass the disabled gate and exercise the inline guard.
    container.querySelectorAll('form')[1].dispatchEvent(
      new Event('submit', { cancelable: true, bubbles: true }),
    );
    await waitFor(() =>
      expect(screen.getByText('Passwords do not match.')).toBeInTheDocument(),
    );
    expect(api.changePassword).not.toHaveBeenCalled();
  });

  it('rejects new password < 8 chars with an inline error', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Current password'), 'oldpassword');
    await user.type(fieldByLabel(container, 'New password'), 'short');
    await user.type(fieldByLabel(container, 'Confirm new password'), 'short');
    container.querySelectorAll('form')[1].dispatchEvent(
      new Event('submit', { cancelable: true, bubbles: true }),
    );
    await waitFor(() =>
      expect(screen.getByText(/at least 8 characters/i)).toBeInTheDocument(),
    );
    expect(api.changePassword).not.toHaveBeenCalled();
  });

  it('happy path: calls api.changePassword and clears the form on success', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    const cur     = fieldByLabel(container, 'Current password');
    const next    = fieldByLabel(container, 'New password');
    const confirm = fieldByLabel(container, 'Confirm new password');
    await user.type(cur,     'oldpassword');
    await user.type(next,    'newpassword');
    await user.type(confirm, 'newpassword');

    await user.click(screen.getByRole('button', { name: /change password/i }));
    await waitFor(() => expect(api.changePassword).toHaveBeenCalledWith('oldpassword', 'newpassword'));
    await waitFor(() => expect(screen.getByText(/password changed/i)).toBeInTheDocument());
    expect(cur.value).toBe('');
    expect(next.value).toBe('');
    expect(confirm.value).toBe('');
  });

  it('translates a 401 into a friendly "Current password is incorrect"', async () => {
    api.changePassword.mockRejectedValueOnce(Object.assign(new Error('unauthorized'), { status: 401 }));
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Current password'), 'wrongpass1');
    await user.type(fieldByLabel(container, 'New password'),     'newpassword');
    await user.type(fieldByLabel(container, 'Confirm new password'), 'newpassword');
    await user.click(screen.getByRole('button', { name: /change password/i }));
    await waitFor(() =>
      expect(screen.getByText(/current password is incorrect/i)).toBeInTheDocument(),
    );
  });

  it('shows the raw server error on non-401 failure', async () => {
    api.changePassword.mockRejectedValueOnce(new Error('rate limited'));
    const user = userEvent.setup();
    const { container } = render(
      <ProfileModal user={USER} onClose={vi.fn()} onSaved={vi.fn()} />,
    );
    await user.type(fieldByLabel(container, 'Current password'), 'oldpassword');
    await user.type(fieldByLabel(container, 'New password'),     'newpassword');
    await user.type(fieldByLabel(container, 'Confirm new password'), 'newpassword');
    await user.click(screen.getByRole('button', { name: /change password/i }));
    await waitFor(() => expect(screen.getByText('rate limited')).toBeInTheDocument());
  });
});
