import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';

vi.mock('../api.js', () => ({
  api: {
    accounts:       vi.fn(),
    importAnalyze:  vi.fn(),
    importApply:    vi.fn(),
    exportURL:      vi.fn((fmt) => `/api/v1/export?format=${fmt}`),
  },
}));

import { api } from '../api.js';
import { ImportExportPage } from './ImportExportPage.jsx';

const PLAN = {
  source: 'ghostfolio',
  warnings: ['One activity skipped (unsupported type).'],
  accounts: [
    { source_id: 'a1', name: 'Brokerage X', currency: 'EUR',
      tx_count: 12, selected: true, map_to_id: 0 },
    { source_id: 'a2', name: 'Crypto wallet', currency: 'USD',
      tx_count: 5, selected: true, map_to_id: 0 },
  ],
  assets: [
    { source_id: 'yahoo::AAPL', symbol: 'AAPL', name: 'Apple Inc.',
      type: 'stock', currency: 'USD', provider: 'yahoo',
      tx_count: 8, selected: true, existing_match: false },
    { source_id: 'cg::bitcoin', symbol: 'BTC', name: 'Bitcoin',
      type: 'crypto', currency: 'USD', provider: 'coingecko',
      tx_count: 3, selected: true, existing_match: true },
  ],
  transactions: new Array(17),
};

const APPLY_RESULT = {
  accounts_created: 1, accounts_reused: 1,
  assets_created: 1, assets_reused: 1,
  transactions_created: 17,
  warnings: [],
};

const EXISTING_ACCOUNTS = [{ id: 99, name: 'Pre-existing', currency: 'EUR' }];

beforeEach(() => {
  vi.clearAllMocks();
  api.accounts.mockResolvedValue(EXISTING_ACCOUNTS);
  api.importAnalyze.mockResolvedValue(PLAN);
  api.importApply.mockResolvedValue(APPLY_RESULT);
});

// Build a File whose .text() resolves to the supplied JSON-able body.
// happy-dom's File supports .text(); we don't need a Blob shim.
function jsonFile(name, body) {
  return new File([JSON.stringify(body)], name, { type: 'application/json' });
}

// Drive the hidden <input type="file"> directly. Defining `files` via
// Object.defineProperty mirrors what real file pickers do.
function uploadFile(container, file) {
  const input = container.querySelector('input[type="file"]');
  Object.defineProperty(input, 'files', { value: [file], configurable: true });
  fireEvent.change(input);
}

describe('ImportExportPage — initial render', () => {
  it('starts on the Upload step with the export links exposed', () => {
    render(<ImportExportPage />);
    expect(screen.getByText('Source software')).toBeInTheDocument();
    // Two export rows.
    const links = screen.getAllByRole('link');
    expect(links.map(a => a.getAttribute('href'))).toEqual(
      expect.arrayContaining([
        '/api/v1/export?format=json',
        '/api/v1/export?format=csv',
      ]),
    );
  });
});

describe('ImportExportPage — analyze + review', () => {
  it('rejects files larger than 10 MB without calling the API', async () => {
    const { container } = render(<ImportExportPage />);
    // Build a sparse File that *reports* > 10 MB without actually
    // allocating it.
    const huge = new File(['x'], 'huge.json', { type: 'application/json' });
    Object.defineProperty(huge, 'size', { value: 11 * 1024 * 1024 });
    uploadFile(container, huge);

    await waitFor(() => expect(screen.getByText(/file too large/i)).toBeInTheDocument());
    expect(api.importAnalyze).not.toHaveBeenCalled();
  });

  it('renders an error and stays on Upload when JSON is invalid', async () => {
    const { container } = render(<ImportExportPage />);
    const bad = new File(['{not-json'], 'bad.json', { type: 'application/json' });
    uploadFile(container, bad);

    await waitFor(() => expect(screen.getByText(/not a valid json file/i)).toBeInTheDocument());
    expect(api.importAnalyze).not.toHaveBeenCalled();
  });

  it('analyses a valid file and advances to the Review step with plan data', async () => {
    const { container } = render(<ImportExportPage />);
    uploadFile(container, jsonFile('export.json', { activities: [] }));

    // Plan loaded into the review step.
    await waitFor(() => expect(screen.getByText(/Accounts \(2\)/)).toBeInTheDocument());
    expect(screen.getByText('Brokerage X')).toBeInTheDocument();
    expect(screen.getByText('Crypto wallet')).toBeInTheDocument();
    expect(screen.getByText(/Assets \(2\)/)).toBeInTheDocument();
    expect(screen.getByText('AAPL')).toBeInTheDocument();
    expect(screen.getByText('BTC')).toBeInTheDocument();

    // Warning from the plan rendered.
    expect(screen.getByText(/One activity skipped/)).toBeInTheDocument();

    // analyze called with the source id and parsed body.
    expect(api.importAnalyze).toHaveBeenCalledWith('ghostfolio', { activities: [] });

    // Existing accounts fetched on entering review.
    await waitFor(() => expect(api.accounts).toHaveBeenCalled());
  });

  it('surfaces the API error message on analyze failure', async () => {
    api.importAnalyze.mockRejectedValueOnce(new Error('parse failed: bad shape'));
    const { container } = render(<ImportExportPage />);
    uploadFile(container, jsonFile('export.json', {}));

    await waitFor(() => expect(screen.getByText(/parse failed: bad shape/)).toBeInTheDocument());
  });
});

describe('ImportExportPage — confirm + apply', () => {
  async function getToConfirm(user) {
    const { container } = render(<ImportExportPage />);
    uploadFile(container, jsonFile('export.json', {}));
    await waitFor(() => expect(screen.getByText(/Accounts \(2\)/)).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await waitFor(() => expect(screen.getByText(/Ready to import/)).toBeInTheDocument());
    return container;
  }

  it('summarises the create/reuse split correctly on the Confirm step', async () => {
    const user = userEvent.setup();
    await getToConfirm(user);
    // 2 accounts, both create (map_to_id=0). 2 assets, 1 reuse (BTC has existing_match).
    expect(screen.getByText('2', { selector: 'strong' })).toBeInTheDocument(); // 2 created accounts
    // 17 transactions to insert.
    expect(screen.getByText('17')).toBeInTheDocument();
  });

  it('apply calls importApply with the plan and advances to Done with the result counts', async () => {
    const user = userEvent.setup();
    await getToConfirm(user);
    await user.click(screen.getByRole('button', { name: /^Import$/ }));

    await waitFor(() => expect(api.importApply).toHaveBeenCalledOnce());
    // The plan passed to importApply still has both accounts selected
    // (we didn't change anything in review).
    const sentPlan = api.importApply.mock.calls[0][0];
    expect(sentPlan.accounts).toHaveLength(2);
    expect(sentPlan.assets).toHaveLength(2);

    // Done step renders the apply result.
    await waitFor(() => expect(screen.getByText(/Import complete/)).toBeInTheDocument());
    expect(screen.getByText(/1 account\(s\) created, 1 reused/)).toBeInTheDocument();
    expect(screen.getByText(/1 asset\(s\) created, 1 reused/)).toBeInTheDocument();
    expect(screen.getByText(/17 transaction\(s\) inserted/)).toBeInTheDocument();
  });

  it('clicking "Import more" returns to the Upload step and clears state', async () => {
    const user = userEvent.setup();
    await getToConfirm(user);
    await user.click(screen.getByRole('button', { name: /^Import$/ }));
    await waitFor(() => expect(screen.getByText(/Import complete/)).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /import more/i }));
    expect(screen.getByText('Source software')).toBeInTheDocument();
    expect(screen.queryByText(/Import complete/)).not.toBeInTheDocument();
  });

  it('renders the API error and stays on Confirm if importApply rejects', async () => {
    api.importApply.mockRejectedValueOnce(new Error('atomicity violation'));
    const user = userEvent.setup();
    await getToConfirm(user);

    await user.click(screen.getByRole('button', { name: /^Import$/ }));
    await waitFor(() => expect(screen.getByText(/atomicity violation/)).toBeInTheDocument());
    // Still on confirm — Import button reachable, no Done banner.
    expect(screen.queryByText(/Import complete/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Import$/ })).toBeInTheDocument();
  });
});

describe('ImportExportPage — review patches', () => {
  async function inReview(user) {
    const { container } = render(<ImportExportPage />);
    uploadFile(container, jsonFile('export.json', {}));
    await waitFor(() => expect(screen.getByText(/Accounts \(2\)/)).toBeInTheDocument());
    return container;
  }

  it('unchecking an account excludes it from the apply payload', async () => {
    const user = userEvent.setup();
    const container = await inReview(user);

    // The first account row's checkbox lives in the first <tr> of
    // the accounts <table>. Walk to it explicitly.
    const accTable = container.querySelectorAll('table.table')[0];
    const firstRowCheckbox = accTable.querySelectorAll('tbody tr')[0]
      .querySelector('input[type="checkbox"]');
    await user.click(firstRowCheckbox);

    await user.click(screen.getByRole('button', { name: 'Next' }));
    await waitFor(() => expect(screen.getByText(/Ready to import/)).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /^Import$/ }));

    await waitFor(() => expect(api.importApply).toHaveBeenCalled());
    // The plan still has both accounts (selected lives on the row);
    // but the Confirm step's "create/reuse" stats use the filtered
    // version. Assert the unselected flag round-tripped.
    const sentPlan = api.importApply.mock.calls[0][0];
    expect(sentPlan.accounts[0].selected).toBe(false);
    expect(sentPlan.accounts[1].selected).toBe(true);
  });
});
