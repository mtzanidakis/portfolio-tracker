import { describe, it, expect, beforeEach } from 'vitest';
import {
  fmtMoney, fmtPct, fmtNum,
  fmtDate, setDateFormat, getDateFormat, previewDateFormat, parseDate,
} from './format.js';

// fmtMoney delegates to Intl.NumberFormat. We assert the shape rather
// than exact strings because Intl output (locale-driven NBSP, currency
// symbol placement) varies a hair across runtimes; pinning to en-US in
// format.js keeps things stable enough that we can match symbols.
describe('fmtMoney', () => {
  it('returns an em-dash for null/undefined/NaN', () => {
    expect(fmtMoney(null, 'USD')).toBe('—');
    expect(fmtMoney(undefined, 'USD')).toBe('—');
    expect(fmtMoney(NaN, 'USD')).toBe('—');
  });

  it('formats a USD value with two decimals by default', () => {
    expect(fmtMoney(1234.5, 'USD')).toBe('$1,234.50');
  });

  it('uses zero decimals for JPY (no fractional yen)', () => {
    expect(fmtMoney(12345, 'JPY')).toBe('¥12,345');
  });

  it('respects an explicit decimals override', () => {
    expect(fmtMoney(1.2345, 'USD', { decimals: 4 })).toBe('$1.2345');
    expect(fmtMoney(1.2345, 'USD', { decimals: 0 })).toBe('$1');
  });

  it('prefixes negatives with a literal minus and absolutes the value', () => {
    expect(fmtMoney(-50, 'USD')).toBe('-$50.00');
  });

  it('adds + for positive values when sign:true', () => {
    expect(fmtMoney(50, 'USD', { sign: true })).toBe('+$50.00');
    // Sign flag does not double up on negatives.
    expect(fmtMoney(-50, 'USD', { sign: true })).toBe('-$50.00');
  });

  it('does not add + for zero with sign:true', () => {
    // n>0 is the gate, so zero stays bare.
    expect(fmtMoney(0, 'USD', { sign: true })).toBe('$0.00');
  });

  it('falls back to USD when currency is empty', () => {
    expect(fmtMoney(10, '')).toBe('$10.00');
  });
});

describe('fmtPct', () => {
  it('returns an em-dash for null/undefined/NaN', () => {
    expect(fmtPct(null)).toBe('—');
    expect(fmtPct(undefined)).toBe('—');
    expect(fmtPct(NaN)).toBe('—');
  });

  it('formats a percentage with two decimals by default and a + sign', () => {
    expect(fmtPct(12.5)).toBe('+12.50%');
  });

  it('omits the + sign when sign:false', () => {
    expect(fmtPct(12.5, { sign: false })).toBe('12.50%');
  });

  it('keeps the literal minus on negatives regardless of sign flag', () => {
    expect(fmtPct(-3.14)).toBe('-3.14%');
    expect(fmtPct(-3.14, { sign: false })).toBe('-3.14%');
  });

  it('does not add + for zero', () => {
    expect(fmtPct(0)).toBe('0.00%');
  });

  it('honours a custom decimals count', () => {
    expect(fmtPct(7.1234, { decimals: 4 })).toBe('+7.1234%');
    expect(fmtPct(7.1234, { decimals: 0 })).toBe('+7%');
  });
});

describe('fmtNum', () => {
  it('returns an em-dash for null/undefined/NaN', () => {
    expect(fmtNum(null)).toBe('—');
    expect(fmtNum(undefined)).toBe('—');
    expect(fmtNum(NaN)).toBe('—');
  });

  it('formats with up to four decimals by default and grouping', () => {
    expect(fmtNum(1234.5678)).toBe('1,234.5678');
    // Trailing zeros are stripped because minimumFractionDigits is 0.
    expect(fmtNum(1)).toBe('1');
  });

  it('honours an explicit decimals cap', () => {
    expect(fmtNum(1.234567, 2)).toBe('1.23');
    expect(fmtNum(1.5, 0)).toBe('2');
  });
});

// fmtDate / setDateFormat / previewDateFormat all share format.js'
// module-level userDateFormat. We reset it between tests so order
// independence is guaranteed.
describe('date format API', () => {
  beforeEach(() => setDateFormat('browser'));

  it('round-trips through getDateFormat', () => {
    setDateFormat('YYYY-MM-DD');
    expect(getDateFormat()).toBe('YYYY-MM-DD');
  });

  it('treats an empty argument as browser', () => {
    setDateFormat('');
    expect(getDateFormat()).toBe('browser');
  });

  it('treats a null argument as browser', () => {
    setDateFormat(null);
    expect(getDateFormat()).toBe('browser');
  });
});

describe('fmtDate', () => {
  beforeEach(() => setDateFormat('browser'));

  // 2026-04-01 chosen so each token has a distinct two-digit value.
  const SAMPLE = new Date(2026, 3, 1);

  it('forwards opts to toLocaleDateString when supplied', () => {
    // Don't pin the locale output — assert it differs from the
    // pattern path (i.e. Intl handled it).
    const opts = { year: 'numeric', month: 'short', day: '2-digit' };
    const got = fmtDate(SAMPLE, opts);
    expect(typeof got).toBe('string');
    expect(got).toMatch(/2026/);
    expect(got).toMatch(/Apr/);
  });

  it('uses the browser locale when format is "browser"', () => {
    // Same: assert it produced *something* with the year. The pattern
    // path would have applied tokens, the browser path uses Intl.
    const got = fmtDate(SAMPLE);
    expect(got).toMatch(/2026/);
  });

  it('applies YYYY-MM-DD pattern', () => {
    setDateFormat('YYYY-MM-DD');
    expect(fmtDate(SAMPLE)).toBe('2026-04-01');
  });

  it('applies DD/MM/YYYY pattern (EU style)', () => {
    setDateFormat('DD/MM/YYYY');
    expect(fmtDate(SAMPLE)).toBe('01/04/2026');
  });

  it('applies MM/DD/YYYY pattern (US style)', () => {
    setDateFormat('MM/DD/YYYY');
    expect(fmtDate(SAMPLE)).toBe('04/01/2026');
  });

  it('honours every supported token', () => {
    setDateFormat('YYYY YY MMMM MMM MM M DD D');
    expect(fmtDate(SAMPLE)).toBe('2026 26 April Apr 04 4 01 1');
  });

  it('replaces tokens longest-match-first (MMMM beats MMM, YYYY beats YY)', () => {
    setDateFormat('MMMM');
    expect(fmtDate(SAMPLE)).toBe('April');
    setDateFormat('YYYY');
    expect(fmtDate(SAMPLE)).toBe('2026');
  });

  it('preserves non-token characters in the pattern', () => {
    setDateFormat('day D of MMM, YYYY');
    expect(fmtDate(SAMPLE)).toBe('day 1 of Apr, 2026');
  });

  it('accepts ISO strings and millisecond timestamps', () => {
    setDateFormat('YYYY-MM-DD');
    expect(fmtDate('2026-04-01T15:00:00Z')).toMatch(/^2026-04-0[12]$/);
    expect(fmtDate(SAMPLE.getTime())).toBe('2026-04-01');
  });
});

describe('parseDate', () => {
  it('returns null for empty input', () => {
    expect(parseDate('', 'DD/MM/YYYY')).toBeNull();
    expect(parseDate(null, 'DD/MM/YYYY')).toBeNull();
  });

  it('round-trips a DD/MM/YYYY string', () => {
    const d = parseDate('29/04/2026', 'DD/MM/YYYY');
    expect(d).toBeInstanceOf(Date);
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(3); // April
    expect(d.getDate()).toBe(29);
  });

  it('round-trips MM/DD/YYYY (US style)', () => {
    const d = parseDate('04/29/2026', 'MM/DD/YYYY');
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(3);
    expect(d.getDate()).toBe(29);
  });

  it('round-trips ISO YYYY-MM-DD', () => {
    const d = parseDate('2026-04-29', 'YYYY-MM-DD');
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(3);
    expect(d.getDate()).toBe(29);
  });

  it('round-trips a textual month pattern', () => {
    const d = parseDate('29 Apr 2026', 'D MMM YYYY');
    expect(d.getMonth()).toBe(3);
    expect(d.getDate()).toBe(29);
  });

  it('rejects strings that do not match the pattern', () => {
    expect(parseDate('29-04-2026', 'DD/MM/YYYY')).toBeNull();
    expect(parseDate('1/4/2026', 'DD/MM/YYYY')).toBeNull(); // padding required
    expect(parseDate('garbage', 'DD/MM/YYYY')).toBeNull();
  });

  it('rejects calendar roll-overs like Feb 30', () => {
    expect(parseDate('30/02/2026', 'DD/MM/YYYY')).toBeNull();
  });

  it('round-trips through fmtDate', () => {
    setDateFormat('DD/MM/YYYY');
    const SAMPLE = new Date(2026, 3, 1);
    const formatted = fmtDate(SAMPLE);
    const parsed = parseDate(formatted, 'DD/MM/YYYY');
    expect(parsed.getFullYear()).toBe(SAMPLE.getFullYear());
    expect(parsed.getMonth()).toBe(SAMPLE.getMonth());
    expect(parsed.getDate()).toBe(SAMPLE.getDate());
  });
});

describe('previewDateFormat', () => {
  it('renders today via the browser locale when pattern is empty/browser', () => {
    const a = previewDateFormat('');
    const b = previewDateFormat('browser');
    const c = previewDateFormat();
    // All three take the locale branch — same result for today.
    expect(a).toBe(b);
    expect(b).toBe(c);
    expect(a).toMatch(/\d/);
  });

  it('renders today via a custom pattern', () => {
    const today = new Date();
    const want = String(today.getFullYear());
    expect(previewDateFormat('YYYY')).toBe(want);
  });

  it('does not mutate the global userDateFormat', () => {
    setDateFormat('YYYY-MM-DD');
    previewDateFormat('DD/MM/YYYY');
    expect(getDateFormat()).toBe('YYYY-MM-DD');
  });
});
