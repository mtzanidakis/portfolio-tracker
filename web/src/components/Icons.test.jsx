import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';
import { Icon, BrandMark } from './Icons.jsx';

// Pulled from Icons.jsx — kept here so adding a new icon flags this
// test file as the place to extend coverage.
const KNOWN = [
  'chart', 'pie', 'activity', 'wallet', 'coins', 'trash', 'edit',
  'plus', 'moon', 'sun', 'eye', 'eyeOff', 'search',
  'arrowUp', 'arrowDown', 'close', 'more', 'check', 'menu', 'swap',
];

describe('Icon', () => {
  it('renders an svg with the default 16×16 viewBox', () => {
    const { container } = render(<Icon name="chart" />);
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    expect(svg.getAttribute('width')).toBe('16');
    expect(svg.getAttribute('height')).toBe('16');
    expect(svg.getAttribute('viewBox')).toBe('0 0 24 24');
    expect(svg.getAttribute('stroke')).toBe('currentColor');
  });

  it('honours the size prop', () => {
    const { container } = render(<Icon name="chart" size={32} />);
    const svg = container.querySelector('svg');
    expect(svg.getAttribute('width')).toBe('32');
    expect(svg.getAttribute('height')).toBe('32');
  });

  it.each(KNOWN)('renders concrete path geometry for %s', (name) => {
    const { container } = render(<Icon name={name} />);
    const svg = container.querySelector('svg');
    // Every known icon emits at least one <path> or <circle>.
    const drawables = svg.querySelectorAll('path, circle');
    expect(drawables.length).toBeGreaterThan(0);
  });

  it('renders an empty svg for an unknown name', () => {
    const { container } = render(<Icon name="not-a-real-icon" />);
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    expect(svg.children.length).toBe(0);
  });
});

describe('BrandMark', () => {
  it('renders an svg with the 64-unit viewBox and the three ascending bars', () => {
    const { container } = render(<BrandMark size={28} />);
    const svg = container.querySelector('svg');
    expect(svg.getAttribute('viewBox')).toBe('0 0 64 64');
    expect(svg.getAttribute('width')).toBe('28');
    expect(svg.querySelectorAll('rect').length).toBeGreaterThanOrEqual(4); // bg + 3 bars
    expect(svg.querySelector('path')).not.toBeNull();
  });

  it('defaults to a 28-unit size when no prop is passed', () => {
    const { container } = render(<BrandMark />);
    expect(container.querySelector('svg').getAttribute('width')).toBe('28');
  });
});
