import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';

// LoginForm's <label>s aren't tied to inputs via htmlFor, so we locate
// the inputs by their HTML type — same as a screen reader fallback.
const emailInput = (root) => root.querySelector('input[type="email"]');
const pwInput    = (root) => root.querySelector('input[type="password"]');
import userEvent from '@testing-library/user-event';
import { LoginForm } from './LoginForm.jsx';

vi.mock('../api.js', () => ({
  api: { login: vi.fn() },
}));

import { api } from '../api.js';

beforeEach(() => {
  api.login.mockReset();
});

describe('LoginForm', () => {
  it('disables Sign in until email + 8-char password are present', async () => {
    const user = userEvent.setup();
    const { container } = render(<LoginForm onLoggedIn={() => {}} />);

    const btn = screen.getByRole('button', { name: /sign in/i });
    expect(btn).toBeDisabled();

    await user.type(emailInput(container), 'me@x');
    expect(btn).toBeDisabled(); // no password yet

    const pw = pwInput(container);
    await user.type(pw, 'short');
    expect(btn).toBeDisabled(); // <8 chars

    await user.type(pw, '123'); // now 8 chars total
    expect(btn).toBeEnabled();
  });

  it('happy-path submit calls api.login and reports the user up', async () => {
    const user = userEvent.setup();
    const me = { id: 1, email: 'me@x' };
    api.login.mockResolvedValueOnce(me);
    const onLoggedIn = vi.fn();
    const { container } = render(<LoginForm onLoggedIn={onLoggedIn} />);

    await user.type(emailInput(container), 'me@x');
    await user.type(pwInput(container), 'password');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => expect(onLoggedIn).toHaveBeenCalledWith(me));
    expect(api.login).toHaveBeenCalledWith('me@x', 'password');
  });

  it('renders an error and re-enables the form on failed login', async () => {
    const user = userEvent.setup();
    api.login.mockRejectedValueOnce(new Error('nope'));
    const { container } = render(<LoginForm onLoggedIn={() => {}} />);

    await user.type(emailInput(container), 'me@x');
    await user.type(pwInput(container), 'password');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() =>
      expect(screen.getByText(/invalid email or password/i)).toBeInTheDocument(),
    );
    // Button is re-enabled so user can retry.
    expect(screen.getByRole('button', { name: /sign in/i })).toBeEnabled();
  });

  it('shows "Signing in…" while in flight', async () => {
    const user = userEvent.setup();
    let resolve;
    api.login.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
    const { container } = render(<LoginForm onLoggedIn={() => {}} />);

    await user.type(emailInput(container), 'me@x');
    await user.type(pwInput(container), 'password');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled();
    resolve({ id: 1 });
  });
});
