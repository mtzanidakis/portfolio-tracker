// Thin fetch wrapper. Token is kept in localStorage.

const TOKEN_KEY = 'pt-token';

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || '';
}
export function setToken(t) {
  localStorage.setItem(TOKEN_KEY, t);
}
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

async function request(method, path, body) {
  const headers = { 'Accept': 'application/json' };
  const tok = getToken();
  if (tok) headers['Authorization'] = 'Bearer ' + tok;
  if (body !== undefined) headers['Content-Type'] = 'application/json';

  const r = await fetch(path, {
    method,
    headers,
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
  version:       ()     => request('GET',    '/api/v1/version'),
  me:            ()     => request('GET',    '/api/v1/me'),
  updateMe:      (p)    => request('PATCH',  '/api/v1/me', p),

  accounts:      ()     => request('GET',    '/api/v1/accounts'),
  createAccount: (p)    => request('POST',   '/api/v1/accounts', p),
  updateAccount: (id,p) => request('PATCH',  `/api/v1/accounts/${id}`, p),
  deleteAccount: (id)   => request('DELETE', `/api/v1/accounts/${id}`),

  assets:        (q='') => request('GET',    '/api/v1/assets' + (q ? '?q=' + encodeURIComponent(q) : '')),
  upsertAsset:   (p)    => request('POST',   '/api/v1/assets', p),

  transactions:  (qs='') => request('GET',    '/api/v1/transactions' + (qs || '')),
  createTx:      (p)    => request('POST',   '/api/v1/transactions', p),
  deleteTx:      (id)   => request('DELETE', `/api/v1/transactions/${id}`),

  holdings:      ()     => request('GET',    '/api/v1/holdings'),
  allocations:   (g='asset') => request('GET', '/api/v1/allocations?group=' + g),
  performance:   (tf='6M') => request('GET', '/api/v1/performance?tf=' + tf),
};
