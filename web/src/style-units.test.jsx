import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';
import { AssetLogo } from './components/AssetLogo.jsx';

// style-units.js is loaded globally via test/setup.js, same as main.jsx
// does in production.
describe('numeric inline styles', () => {
  it('appends px to dimensional properties', () => {
    const { container } = render(<div style={{ width: 32, padding: 24, marginTop: 4 }} />);
    const s = container.firstChild.style;
    expect(s.width).toBe('32px');
    expect(s.padding).toBe('24px');
    expect(s.marginTop).toBe('4px');
  });

  it('leaves unitless properties alone', () => {
    const { container } = render(
      <div style={{ opacity: 0.5, lineHeight: 1.7, fontWeight: 600, zIndex: 3, flex: 1 }} />,
    );
    const s = container.firstChild.style;
    expect(s.opacity).toBe('0.5');
    expect(s.lineHeight).toBe('1.7');
    expect(s.fontWeight).toBe('600');
    expect(s.zIndex).toBe('3');
    expect(s.flexGrow).toBe('1');
  });

  it('does not mutate the caller\'s style object', () => {
    const shared = { width: 10 };
    render(<div style={shared} />);
    expect(shared.width).toBe(10);
  });

  it('sizes AssetLogo images', () => {
    const asset = { symbol: 'BTC', type: 'crypto', logo_url: 'https://x/btc.png' };
    const { container } = render(<AssetLogo asset={asset} size={40} />);
    const img = container.querySelector('img');
    expect(img.style.width).toBe('40px');
    expect(img.style.height).toBe('40px');
  });
});
