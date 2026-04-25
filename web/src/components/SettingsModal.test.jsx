import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: { updateMe: vi.fn() },
}));

import { api } from '../api.js';
import { SettingsModal } from './SettingsModal.jsx';

const USER = { name: 'Alex', email: 'alex@x', base_currency: 'EUR' };

function setup(overrides = {}) {
  const props = {
    user: USER,
    aesthetic: 'technical',
    setAesthetic: vi.fn(),
    dateFormat: 'browser',
    setDateFormat: vi.fn(),
    onSaved: vi.fn(),
    onClose: vi.fn(),
    ...overrides,
  };
  return { props, ...render(<SettingsModal {...props} />) };
}

beforeEach(() => {
  vi.clearAllMocks();
  api.updateMe.mockResolvedValue({ ...USER });
});

describe('SettingsModal — base currency', () => {
  it('skips api.updateMe when the base currency is unchanged', async () => {
    const user = userEvent.setup();
    const { props } = setup();
    await user.click(screen.getByRole('button', { name: /save settings/i }));
    await waitFor(() => expect(screen.getByText('Saved.')).toBeInTheDocument());
    expect(api.updateMe).not.toHaveBeenCalled();
    // setDateFormat is always called on save (it persists the chosen
    // pattern to localStorage via the parent); browser is the default.
    expect(props.setDateFormat).toHaveBeenCalledWith('browser');
  });

  it('persists a base-currency change and reports it up via onSaved', async () => {
    const user = userEvent.setup();
    api.updateMe.mockResolvedValueOnce({ ...USER, base_currency: 'USD' });
    const { props, container } = setup();

    // happy-dom doesn't reliably sync controlled <select value={...}>
    // when user-event drives the change; fire the input event directly.
    const baseSelect = container.querySelectorAll('select')[0];
    fireEvent.change(baseSelect, { target: { value: 'USD' } });
    await user.click(screen.getByRole('button', { name: /save settings/i }));

    await waitFor(() => expect(api.updateMe).toHaveBeenCalledWith({ base_currency: 'USD' }));
    await waitFor(() => expect(props.onSaved).toHaveBeenCalled());
  });

  it('renders the API error and stays open on failure', async () => {
    api.updateMe.mockRejectedValueOnce(new Error('rate limited'));
    const user = userEvent.setup();
    const { container } = setup();
    const baseSelect = container.querySelectorAll('select')[0];
    fireEvent.change(baseSelect, { target: { value: 'USD' } });
    await user.click(screen.getByRole('button', { name: /save settings/i }));
    await waitFor(() => expect(screen.getByText('rate limited')).toBeInTheDocument());
  });
});

describe('SettingsModal — aesthetic toggle', () => {
  it('clicking an aesthetic option calls setAesthetic with its id', async () => {
    const user = userEvent.setup();
    const { props } = setup();

    await user.click(screen.getByRole('button', { name: /editorial/i }));
    expect(props.setAesthetic).toHaveBeenCalledWith('editorial');

    await user.click(screen.getByRole('button', { name: /forest/i }));
    expect(props.setAesthetic).toHaveBeenCalledWith('forest');
  });

  it('the active aesthetic shows a check icon', () => {
    const { container } = setup({ aesthetic: 'forest' });
    // The check icon lives inside the aesthetic button. Find the row
    // by text and assert it contains an svg.
    const forestBtn = screen.getByRole('button', { name: /forest/i });
    expect(forestBtn.querySelector('svg')).not.toBeNull();
    const technicalBtn = screen.getByRole('button', { name: /technical/i });
    expect(technicalBtn.querySelector('svg')).toBeNull();
  });
});

describe('SettingsModal — date format', () => {
  it('changing a preset shows a live preview through previewDateFormat', async () => {
    const { container } = setup();

    const presetSelect = container.querySelectorAll('select')[1];
    fireEvent.change(presetSelect, { target: { value: 'YYYY-MM-DD' } });

    // The preview line below the dropdown shows today rendered in the
    // chosen pattern. Match the year — robust regardless of when the
    // tests run.
    const year = String(new Date().getFullYear());
    await waitFor(() =>
      expect(screen.getByText(new RegExp(`Today shows as: ${year}-`))).toBeInTheDocument(),
    );
  });

  it('selecting "custom" reveals the pattern input and previews it', async () => {
    const user = userEvent.setup();
    const { container } = setup();
    const presetSelect = container.querySelectorAll('select')[1];
    fireEvent.change(presetSelect, { target: { value: 'custom' } });

    await waitFor(() => expect(container.querySelector('input.input')).not.toBeNull());
    const customField = container.querySelector('input.input');
    expect(customField.value).toBe('DD.MM.YYYY');

    await user.clear(customField);
    await user.type(customField, 'YYYY/MM/DD');
    const year = String(new Date().getFullYear());
    await waitFor(() =>
      expect(screen.getByText(new RegExp(`Today shows as: ${year}/`))).toBeInTheDocument(),
    );
  });

  it('opens straight into custom mode when the saved pattern is non-preset', () => {
    const { container } = setup({ dateFormat: 'DD.MM.YYYY' });
    // Custom pattern input is visible, seeded with the saved value.
    const customField = container.querySelector('input.input');
    expect(customField.value).toBe('DD.MM.YYYY');
  });

  it('Save persists the chosen pattern through setDateFormat', async () => {
    const user = userEvent.setup();
    const { container, props } = setup();
    fireEvent.change(container.querySelectorAll('select')[1], { target: { value: 'DD/MM/YYYY' } });
    await user.click(screen.getByRole('button', { name: /save settings/i }));
    await waitFor(() => expect(props.setDateFormat).toHaveBeenCalledWith('DD/MM/YYYY'));
  });
});

describe('SettingsModal — chrome', () => {
  it('Close button calls onClose', async () => {
    const user = userEvent.setup();
    const { props } = setup();
    await user.click(screen.getByRole('button', { name: /^close$/i }));
    expect(props.onClose).toHaveBeenCalledOnce();
  });
});
