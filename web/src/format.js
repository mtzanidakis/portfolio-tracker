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

// --- Date format ----------------------------------------------------------
//
// Users can pick a pattern in Settings. 'browser' means "use the browser
// locale" (e.g. en-US → MM/DD/YYYY, en-GB/el → DD/MM/YYYY). Anything else
// is a token pattern we render ourselves.
//
// Tokens (longest-match-first so MMMM beats MMM, YYYY beats YY):
//   YYYY  4-digit year           YY  2-digit year
//   MMMM  full month name        MMM 3-letter month name
//   MM    2-digit month          M   month (no pad)
//   DD    2-digit day            D   day (no pad)

const MONTHS_LONG  = ['January','February','March','April','May','June',
                      'July','August','September','October','November','December'];
const MONTHS_SHORT = ['Jan','Feb','Mar','Apr','May','Jun',
                      'Jul','Aug','Sep','Oct','Nov','Dec'];

let userDateFormat = 'browser';

export function setDateFormat(pattern) {
  userDateFormat = pattern || 'browser';
}

export function getDateFormat() {
  return userDateFormat;
}

const TOKEN_RE = /YYYY|YY|MMMM|MMM|MM|M|DD|D/g;

function applyPattern(date, pattern) {
  const y = date.getFullYear();
  const m = date.getMonth();
  const d = date.getDate();
  const pad = (n) => (n < 10 ? '0' + n : '' + n);
  return pattern.replace(TOKEN_RE, (t) => {
    switch (t) {
      case 'YYYY': return String(y);
      case 'YY':   return String(y).slice(-2);
      case 'MMMM': return MONTHS_LONG[m];
      case 'MMM':  return MONTHS_SHORT[m];
      case 'MM':   return pad(m + 1);
      case 'M':    return String(m + 1);
      case 'DD':   return pad(d);
      case 'D':    return String(d);
      default:     return t;
    }
  });
}

// Render a formatted date. When opts are passed (e.g. Chart.jsx axis
// labels) we forward to toLocaleDateString so callers keep full control;
// those short axis strings should look the same regardless of the user's
// full-date pattern. Otherwise we apply the user's saved pattern, or the
// browser locale when it's 'browser'.
export function fmtDate(d, opts) {
  const date = new Date(d);
  if (opts) return date.toLocaleDateString(undefined, opts);
  if (userDateFormat && userDateFormat !== 'browser') {
    return applyPattern(date, userDateFormat);
  }
  return date.toLocaleDateString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' });
}

// A small sample so the Settings modal can show "Today shows as: …"
// without the caller having to know the pattern internals.
export function previewDateFormat(pattern) {
  const today = new Date();
  if (!pattern || pattern === 'browser') {
    return today.toLocaleDateString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' });
  }
  return applyPattern(today, pattern);
}

// Reverse of fmtDate's pattern path: parse a string the user typed in
// their chosen pattern back into a Date. Returns null when the input
// doesn't match. 'browser' falls back to the Date constructor since we
// can't reliably reverse Intl-formatted dates.
const PARSE_TOKEN_RE = /YYYY|YY|MMMM|MMM|MM|M|DD|D/g;
function escapeRegex(s) { return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'); }

export function parseDate(input, pattern) {
  if (!input) return null;
  if (!pattern || pattern === 'browser') {
    const d = new Date(input);
    return isNaN(d.getTime()) ? null : d;
  }

  const tokens = [];
  let regex = '';
  let lastIdx = 0;
  PARSE_TOKEN_RE.lastIndex = 0;
  let m;
  while ((m = PARSE_TOKEN_RE.exec(pattern)) !== null) {
    if (m.index > lastIdx) regex += escapeRegex(pattern.slice(lastIdx, m.index));
    const t = m[0];
    tokens.push(t);
    switch (t) {
      case 'YYYY': regex += '(\\d{4})'; break;
      case 'YY':   regex += '(\\d{2})'; break;
      case 'MMMM': regex += '(' + MONTHS_LONG.join('|')  + ')'; break;
      case 'MMM':  regex += '(' + MONTHS_SHORT.join('|') + ')'; break;
      case 'MM':   regex += '(\\d{2})'; break;
      case 'M':    regex += '(\\d{1,2})'; break;
      case 'DD':   regex += '(\\d{2})'; break;
      case 'D':    regex += '(\\d{1,2})'; break;
    }
    lastIdx = m.index + t.length;
  }
  if (lastIdx < pattern.length) regex += escapeRegex(pattern.slice(lastIdx));

  const match = input.match(new RegExp('^' + regex + '$'));
  if (!match) return null;

  let year = -1, month = -1, day = -1;
  for (let i = 0; i < tokens.length; i++) {
    const v = match[i + 1];
    switch (tokens[i]) {
      case 'YYYY': year = parseInt(v, 10); break;
      case 'YY':   year = 2000 + parseInt(v, 10); break;
      case 'MMMM': month = MONTHS_LONG.indexOf(v); break;
      case 'MMM':  month = MONTHS_SHORT.indexOf(v); break;
      case 'MM':
      case 'M':    month = parseInt(v, 10) - 1; break;
      case 'DD':
      case 'D':    day = parseInt(v, 10); break;
    }
  }
  if (year < 0 || month < 0 || day < 0) return null;
  if (month > 11 || day < 1 || day > 31) return null;
  const d = new Date(year, month, day);
  // Reject roll-overs like Feb 30 → Mar 2.
  if (d.getFullYear() !== year || d.getMonth() !== month || d.getDate() !== day) return null;
  return d;
}
