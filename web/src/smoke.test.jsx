// Smoke test for the test toolchain: vitest runs, oxc's JSX transform
// produces preact-flavoured output, happy-dom provides a DOM, and
// @testing-library/jest-dom's matchers are wired up. The localStorage
// + cookie cleanup in test/setup.js also gets exercised here.
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/preact';

describe('test toolchain', () => {
  it('runs vitest', () => {
    expect(1 + 1).toBe(2);
  });

  it('renders Preact JSX in the DOM env', () => {
    render(<div>hello</div>);
    expect(screen.getByText('hello')).toBeInTheDocument();
  });

  it('exposes a working Web Storage API', () => {
    window.localStorage.setItem('k', 'v');
    expect(window.localStorage.getItem('k')).toBe('v');
  });
});
