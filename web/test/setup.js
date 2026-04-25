// Per-test bootstrap: jest-dom matchers + reset of the bits the app
// touches outside React state (localStorage, document.cookie). Without
// the reset, tests that read App's persisted preferences would leak
// state into each other.
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/preact';

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
