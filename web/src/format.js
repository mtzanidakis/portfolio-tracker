// Display formatters. All accept a currency code ("USD", "EUR", ...).

const JPY_DECIMALS = 0;

export function fmtMoney(n, currency, { sign = false, decimals } = {}) {
  if (n === null || n === undefined || isNaN(n)) return '—';
  const d = decimals ?? (currency === 'JPY' ? JPY_DECIMALS : 2);
  const abs = Math.abs(n);
  const f = new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: currency || 'USD',
    minimumFractionDigits: d,
    maximumFractionDigits: d,
  });
  const s = f.format(abs);
  if (n < 0) return '-' + s;
  if (sign && n > 0) return '+' + s;
  return s;
}

export function fmtPct(n, { sign = true, decimals = 2 } = {}) {
  if (n === null || n === undefined || isNaN(n)) return '—';
  const v = n.toFixed(decimals) + '%';
  return sign && n > 0 ? '+' + v : v;
}

export function fmtNum(n, decimals = 4) {
  if (n === null || n === undefined || isNaN(n)) return '—';
  return n.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: decimals });
}

export function fmtDate(d, opts = { month: 'short', day: 'numeric' }) {
  return new Date(d).toLocaleDateString('en-US', opts);
}
