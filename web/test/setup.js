// Per-test bootstrap: jest-dom matchers + reset of the bits the app
// touches outside React state (localStorage, document.cookie). Without
// the reset, tests that read App's persisted preferences would leak
// state into each other.
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/preact';

// Node 26 defines its own `localStorage` accessor on globalThis, gated
// behind --localstorage-file and therefore undefined without it. Under
// vitest happy-dom is installed *onto* globalThis (window === globalThis),
// so Node's accessor shadows the one happy-dom provides — sessionStorage
// comes through fine, localStorage does not. Restore it from happy-dom's
// own Storage so the app's persisted preferences behave in tests.
if (!window.localStorage && typeof window.Storage === 'function') {
  Object.defineProperty(window, 'localStorage', {
    value: new window.Storage(),
    configurable: true,
    writable: true,
  });
}

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  // Wipe every cookie the DOM env currently has set.
  for (const c of window.document.cookie.split(';')) {
    const name = c.split('=')[0].trim();
    if (name) {
      window.document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
    }
  }
});
