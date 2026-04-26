import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { api } from './api.js';

// Helper that stages a JSON response. The mocked Response has just
// enough surface for api.js (status, ok, statusText, json(), headers).
function jsonResponse(body, { status = 200, statusText = 'OK', headers = {} } = {}) {
  return {
    status,
    statusText,
    ok: status >= 200 && status < 300,
    json: async () => body,
    headers: new Headers(headers),
  };
}

// Each test starts with a fresh fetch spy so we can introspect what
// api.js sent and pin a fixed reply.
let fetchMock;

beforeEach(() => {
  fetchMock = vi.fn().mockResolvedValue(jsonResponse({}));
  globalThis.fetch = fetchMock;
});

afterEach(() => {
  vi.restoreAllMocks();
});

// --- request() helper paths -----------------------------------------------

describe('request() helper', () => {
  it('sends GET with Accept header and credentials:include, no CSRF', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ ok: 1 }));
    const res = await api.accounts();
    expect(res).toEqual({ ok: 1 });

    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe('/api/v1/accounts');
    expect(init.method).toBe('GET');
    expect(init.credentials).toBe('include');
    expect(init.headers['Accept']).toBe('application/json');
    expect(init.headers['X-CSRF-Token']).toBeUndefined();
    expect(init.headers['Content-Type']).toBeUndefined();
    expect(init.body).toBeUndefined();
  });

  it('attaches Content-Type and JSON body on POST', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ id: 7 }));
    await api.createAccount({ name: 'Brokerage', currency: 'EUR' });

    const [, init] = fetchMock.mock.calls[0];
    expect(init.method).toBe('POST');
    expect(init.headers['Content-Type']).toBe('application/json');
    expect(init.body).toBe(JSON.stringify({ name: 'Brokerage', currency: 'EUR' }));
  });

  it('reads pt_csrf cookie and sends X-CSRF-Token on unsafe methods', async () => {
    document.cookie = 'pt_csrf=abc-123; path=/';
    fetchMock.mockResolvedValueOnce(jsonResponse({}));
    await api.createAccount({ name: 'x' });

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBe('abc-123');
  });

  it('does not send X-CSRF-Token if the cookie is absent', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({}));
    await api.createAccount({ name: 'x' });

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBeUndefined();
  });

  it('attaches X-CSRF-Token on PATCH and DELETE too', async () => {
    document.cookie = 'pt_csrf=tok; path=/';
    fetchMock.mockResolvedValue(jsonResponse({}));
    await api.updateAccount(1, { name: 'x' });
    await api.deleteAccount(1);

    expect(fetchMock.mock.calls[0][1].method).toBe('PATCH');
    expect(fetchMock.mock.calls[0][1].headers['X-CSRF-Token']).toBe('tok');
    expect(fetchMock.mock.calls[1][1].method).toBe('DELETE');
    expect(fetchMock.mock.calls[1][1].headers['X-CSRF-Token']).toBe('tok');
  });

  it('decodes percent-encoded cookie values', async () => {
    document.cookie = 'pt_csrf=hello%20world; path=/';
    fetchMock.mockResolvedValueOnce(jsonResponse({}));
    await api.createAccount({});

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBe('hello world');
  });

  it('picks the right cookie when multiple are set', async () => {
    document.cookie = 'other=ignored; path=/';
    document.cookie = 'pt_csrf=picked; path=/';
    document.cookie = 'pt_session=alsoignored; path=/';
    fetchMock.mockResolvedValueOnce(jsonResponse({}));
    await api.createAccount({});

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBe('picked');
  });

  it('throws Error{status:401} on a 401 response', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({}, { status: 401, statusText: 'Unauthorized' }));
    await expect(api.me()).rejects.toMatchObject({ status: 401, message: 'unauthorized' });
  });

  it('extracts error.message from a JSON error body on non-OK', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'invalid input' }, { status: 400, statusText: 'Bad Request' }));
    await expect(api.createAccount({})).rejects.toMatchObject({
      status: 400,
      message: 'invalid input',
    });
  });

  it('falls back to statusText when the error body is not JSON', async () => {
    fetchMock.mockResolvedValueOnce({
      status: 500,
      statusText: 'Internal Server Error',
      ok: false,
      json: async () => { throw new Error('not json'); },
      headers: new Headers(),
    });
    await expect(api.accounts()).rejects.toMatchObject({
      status: 500,
      message: 'Internal Server Error',
    });
  });

  it('returns null on 204 No Content', async () => {
    fetchMock.mockResolvedValueOnce({
      status: 204, statusText: 'No Content', ok: true,
      json: async () => { throw new Error('no body'); },
      headers: new Headers(),
    });
    document.cookie = 'pt_csrf=t; path=/';
    const res = await api.deleteAccount(1);
    expect(res).toBeNull();
  });
});

// --- endpoint URLs / methods ---------------------------------------------
//
// Each endpoint is small enough to verify both URL and method in one
// place. Body shapes are checked where the helper munges them
// (e.g. login, transactionsPage).

describe('endpoint shapes', () => {
  function call(spec) {
    fetchMock.mockResolvedValueOnce(jsonResponse({}));
    return spec();
  }
  function assertCall(method, path, body) {
    const [gotPath, init] = fetchMock.mock.calls.at(-1);
    expect(gotPath).toBe(path);
    expect(init.method).toBe(method);
    if (body === undefined) {
      expect(init.body).toBeUndefined();
    } else {
      expect(init.body).toBe(JSON.stringify(body));
    }
  }

  it('auth endpoints', async () => {
    await call(() => api.login('a@b', 'pw'));
    assertCall('POST', '/api/v1/login', { email: 'a@b', password: 'pw' });

    await call(() => api.logout());
    assertCall('POST', '/api/v1/logout', undefined);

    await call(() => api.changePassword('old', 'new'));
    assertCall('POST', '/api/v1/password', { current: 'old', new: 'new' });
  });

  it('profile endpoints', async () => {
    await call(() => api.version()); assertCall('GET', '/api/v1/version');
    await call(() => api.me());      assertCall('GET', '/api/v1/me');
    await call(() => api.updateMe({ name: 'X' }));
    assertCall('PATCH', '/api/v1/me', { name: 'X' });
  });

  it('self-service tokens', async () => {
    await call(() => api.listTokens());            assertCall('GET',    '/api/v1/me/tokens');
    await call(() => api.createToken('default'));  assertCall('POST',   '/api/v1/me/tokens', { name: 'default' });
    await call(() => api.revokeToken(42));         assertCall('POST',   '/api/v1/me/tokens/42/revoke', undefined);
    await call(() => api.deleteToken(42));         assertCall('DELETE', '/api/v1/me/tokens/42', undefined);
  });

  it('account endpoints', async () => {
    await call(() => api.accounts());                 assertCall('GET',    '/api/v1/accounts');
    await call(() => api.createAccount({ name:'x' })); assertCall('POST',  '/api/v1/accounts', { name: 'x' });
    await call(() => api.updateAccount(3, { name:'y' })); assertCall('PATCH', '/api/v1/accounts/3', { name: 'y' });
    await call(() => api.deleteAccount(3));            assertCall('DELETE', '/api/v1/accounts/3', undefined);
  });

  it('asset endpoints (and percent-encodes symbols with slashes)', async () => {
    await call(() => api.assets());                assertCall('GET',  '/api/v1/assets');
    await call(() => api.assets('aapl'));          assertCall('GET',  '/api/v1/assets?q=aapl');
    await call(() => api.assets('a b'));           assertCall('GET',  '/api/v1/assets?q=a%20b');
    await call(() => api.upsertAsset({ symbol:'AAPL' })); assertCall('POST', '/api/v1/assets', { symbol: 'AAPL' });
    // BTC-USD is the kind of symbol that must round-trip through the URL safely.
    await call(() => api.deleteAsset('BTC-USD'));  assertCall('DELETE', '/api/v1/assets/BTC-USD', undefined);
    await call(() => api.assetPrice('BTC-USD'));   assertCall('GET',    '/api/v1/assets/BTC-USD/price');
    await call(() => api.lookupAsset('AAPL'));
    assertCall('GET', '/api/v1/assets/lookup?symbol=AAPL&provider=yahoo');
    await call(() => api.lookupAsset('bitcoin', 'coingecko'));
    assertCall('GET', '/api/v1/assets/lookup?symbol=bitcoin&provider=coingecko');
  });

  it('transaction endpoints', async () => {
    await call(() => api.transactions());           assertCall('GET',  '/api/v1/transactions');
    await call(() => api.transactions('?limit=5')); assertCall('GET',  '/api/v1/transactions?limit=5');
    await call(() => api.txSummary());              assertCall('GET',  '/api/v1/transactions/summary');
    await call(() => api.createTx({ qty: 1 }));     assertCall('POST', '/api/v1/transactions', { qty: 1 });
    await call(() => api.updateTx(9, { qty: 2 })); assertCall('PATCH', '/api/v1/transactions/9', { qty: 2 });
    await call(() => api.deleteTx(9));              assertCall('DELETE', '/api/v1/transactions/9', undefined);
  });

  it('FX rate endpoint with and without "at" timestamp', async () => {
    await call(() => api.fxRate('USD', 'EUR'));
    assertCall('GET', '/api/v1/fx/rate?from=USD&to=EUR');
    await call(() => api.fxRate('USD', 'EUR', '2026-04-01'));
    assertCall('GET', '/api/v1/fx/rate?from=USD&to=EUR&at=2026-04-01');
  });

  it('holdings / allocations / performance / refresh', async () => {
    await call(() => api.holdings());            assertCall('GET',  '/api/v1/holdings');
    await call(() => api.allocations());         assertCall('GET',  '/api/v1/allocations?group=asset');
    await call(() => api.allocations('account')); assertCall('GET', '/api/v1/allocations?group=account');
    await call(() => api.performance());          assertCall('GET', '/api/v1/performance?tf=6M');
    await call(() => api.performance('1Y'));      assertCall('GET', '/api/v1/performance?tf=1Y');
    await call(() => api.refreshPrices());        assertCall('POST', '/api/v1/prices/refresh', undefined);
  });

  it('import endpoints', async () => {
    await call(() => api.importAnalyze('ghostfolio', { rows: [] }));
    assertCall('POST', '/api/v1/import/ghostfolio/analyze', { rows: [] });
    await call(() => api.importApply({ Source: 'ghostfolio' }));
    assertCall('POST', '/api/v1/import/apply', { Source: 'ghostfolio' });
  });

  it('exportURL builds a URL string, not a request', () => {
    expect(api.exportURL()).toBe('/api/v1/export?format=json');
    expect(api.exportURL('csv')).toBe('/api/v1/export?format=csv');
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

// --- transactionsPage: the only endpoint that doesn't go through request()
// because it needs to read response headers (X-Next-Cursor).
describe('transactionsPage', () => {
  it('builds the query string from supplied params (skipping empties)', async () => {
    fetchMock.mockResolvedValueOnce({
      ...jsonResponse([], { headers: { 'X-Next-Cursor': '' } }),
    });
    await api.transactionsPage({
      q: 'aapl', side: 'buy', symbol: 'AAPL', accountId: 7,
      sort: 'date', order: 'desc', cursor: 'cur', limit: 25,
    });
    const [path] = fetchMock.mock.calls[0];
    // URLSearchParams may reorder; check via parse.
    const qs = new URL(path, 'http://x').searchParams;
    expect(qs.get('q')).toBe('aapl');
    expect(qs.get('side')).toBe('buy');
    expect(qs.get('symbol')).toBe('AAPL');
    expect(qs.get('account_id')).toBe('7');
    expect(qs.get('sort')).toBe('date');
    expect(qs.get('order')).toBe('desc');
    expect(qs.get('cursor')).toBe('cur');
    expect(qs.get('limit')).toBe('25');
  });

  it('omits empty params and uses a default limit of 50', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([], { headers: { 'X-Next-Cursor': '' } }));
    await api.transactionsPage();
    const [path] = fetchMock.mock.calls[0];
    const qs = new URL(path, 'http://x').searchParams;
    expect(qs.get('limit')).toBe('50');
    expect(qs.get('q')).toBeNull();
    expect(qs.get('cursor')).toBeNull();
    expect(qs.get('account_id')).toBeNull();
  });

  it('returns the response body as items and the X-Next-Cursor header', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(
      [{ id: 1 }, { id: 2 }],
      { headers: { 'X-Next-Cursor': 'next-token' } },
    ));
    const { items, nextCursor } = await api.transactionsPage();
    expect(items).toEqual([{ id: 1 }, { id: 2 }]);
    expect(nextCursor).toBe('next-token');
  });

  it('returns "" for nextCursor when the header is absent', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([]));
    const { nextCursor } = await api.transactionsPage();
    expect(nextCursor).toBe('');
  });

  it('forwards the CSRF cookie even on GET (mirrors request() behaviour)', async () => {
    document.cookie = 'pt_csrf=tx-tok; path=/';
    fetchMock.mockResolvedValueOnce(jsonResponse([]));
    await api.transactionsPage();
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBe('tx-tok');
  });

  it('throws with status + message extracted from JSON body on non-OK', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(
      { error: 'bad cursor' }, { status: 400, statusText: 'Bad Request' },
    ));
    await expect(api.transactionsPage()).rejects.toMatchObject({
      status: 400,
      message: 'bad cursor',
    });
  });
});
