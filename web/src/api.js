// Thin fetch wrapper. Browser clients authenticate via the pt_session
// cookie set by POST /api/v1/login; ptagent / automation use Bearer
// tokens (see skill/SKILL.md).
//
// For state-changing requests we attach an X-CSRF-Token header whose
// value is read directly from the pt_csrf cookie (which the server sets
// as non-HttpOnly specifically so JS can read it). That's the
// double-submit-cookie pattern and it satisfies the server middleware.

const CSRF_COOKIE = 'pt_csrf';
const CSRF_HEADER = 'X-CSRF-Token';

function readCookie(name) {
  const prefix = name + '=';
  for (const part of document.cookie.split(';')) {
    const trimmed = part.trim();
    if (trimmed.startsWith(prefix)) return decodeURIComponent(trimmed.slice(prefix.length));
  }
  return '';
}

function isUnsafe(method) {
  const m = method.toUpperCase();
  return m === 'POST' || m === 'PATCH' || m === 'PUT' || m === 'DELETE';
}

async function request(method, path, body) {
  const headers = { 'Accept': 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (isUnsafe(method)) {
    const csrf = readCookie(CSRF_COOKIE);
    if (csrf) headers[CSRF_HEADER] = csrf;
  }

  const r = await fetch(path, {
    method,
    headers,
    credentials: 'include',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (r.status === 401) {
    const err = new Error('unauthorized');
    err.status = 401;
    throw err;
  }
  if (!r.ok) {
    let msg = r.statusText;
    try { msg = (await r.json()).error || msg; } catch { /* ignore */ }
    const err = new Error(msg);
    err.status = r.status;
    throw err;
  }
  if (r.status === 204) return null;
  return r.json();
}

export const api = {
  // --- auth ---
  login:          (email, password) => request('POST',  '/api/v1/login', { email, password }),
  logout:         ()                => request('POST',  '/api/v1/logout'),
  changePassword: (current, next)   => request('POST',  '/api/v1/password', { current, new: next }),

  // --- profile ---
  version:   ()  => request('GET',   '/api/v1/version'),
  me:        ()  => request('GET',   '/api/v1/me'),
  updateMe:  (p) => request('PATCH', '/api/v1/me', p),

  // --- self-service tokens ---
  listTokens:   ()       => request('GET',    '/api/v1/me/tokens'),
  createToken:  (name)   => request('POST',   '/api/v1/me/tokens', { name }),
  revokeToken:  (id)     => request('DELETE', `/api/v1/me/tokens/${id}`),

  // --- resources ---
  accounts:      ()     => request('GET',    '/api/v1/accounts'),
  createAccount: (p)    => request('POST',   '/api/v1/accounts', p),
  updateAccount: (id,p) => request('PATCH',  `/api/v1/accounts/${id}`, p),
  deleteAccount: (id)   => request('DELETE', `/api/v1/accounts/${id}`),

  assets:        (q='') => request('GET',    '/api/v1/assets' + (q ? '?q=' + encodeURIComponent(q) : '')),
  upsertAsset:   (p)    => request('POST',   '/api/v1/assets', p),
  deleteAsset:   (sym)  => request('DELETE', `/api/v1/assets/${encodeURIComponent(sym)}`),
  lookupAsset:   (symbol, provider='yahoo') =>
    request('GET', '/api/v1/assets/lookup?symbol=' + encodeURIComponent(symbol) +
      '&provider=' + encodeURIComponent(provider)),

  transactions:  (qs='') => request('GET',   '/api/v1/transactions' + (qs || '')),
  // transactionsPage is the paginated variant: returns {items, nextCursor}.
  // The server echoes X-Next-Cursor via response header so the body
  // stays a plain array (keeps ptagent and the GET-all helper working).
  transactionsPage: async ({ q = '', side = '', symbol = '', accountId = 0,
                             sort = '', order = '', cursor = '', limit = 50 } = {}) => {
    const params = new URLSearchParams();
    if (limit)     params.set('limit', limit);
    if (cursor)    params.set('cursor', cursor);
    if (q)         params.set('q', q);
    if (side)      params.set('side', side);
    if (symbol)    params.set('symbol', symbol);
    if (accountId) params.set('account_id', accountId);
    if (sort)      params.set('sort', sort);
    if (order)     params.set('order', order);
    const csrf = readCookie(CSRF_COOKIE);
    const headers = { Accept: 'application/json' };
    if (csrf) headers[CSRF_HEADER] = csrf;
    const r = await fetch('/api/v1/transactions?' + params.toString(), {
      method: 'GET',
      credentials: 'include',
      headers,
    });
    if (!r.ok) {
      let msg = r.statusText;
      try { msg = (await r.json()).error || msg; } catch { /* ignore */ }
      const err = new Error(msg);
      err.status = r.status;
      throw err;
    }
    return {
      items: await r.json(),
      nextCursor: r.headers.get('X-Next-Cursor') || '',
    };
  },
  txSummary:     ()      => request('GET',   '/api/v1/transactions/summary'),
  createTx:      (p)     => request('POST',  '/api/v1/transactions', p),
  updateTx:      (id, p) => request('PATCH', `/api/v1/transactions/${id}`, p),
  deleteTx:      (id)    => request('DELETE', `/api/v1/transactions/${id}`),

  // --- FX ---
  fxRate: (from, to, at) =>
    request('GET', `/api/v1/fx/rate?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}` +
      (at ? `&at=${encodeURIComponent(at)}` : '')),

  holdings:      ()          => request('GET', '/api/v1/holdings'),
  allocations:   (g='asset') => request('GET', '/api/v1/allocations?group=' + g),
  performance:   (tf='6M')   => request('GET', '/api/v1/performance?tf=' + tf),
  refreshPrices: ()          => request('POST', '/api/v1/prices/refresh'),
};
