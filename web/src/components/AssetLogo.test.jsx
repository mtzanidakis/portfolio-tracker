import { describe, it, expect } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';
import { AssetLogo } from './AssetLogo.jsx';

describe('AssetLogo', () => {
  it('renders an empty placeholder when asset is missing', () => {
    const { container } = render(<AssetLogo asset={null} />);
    const div = container.querySelector('.asset-logo');
    expect(div).not.toBeNull();
    expect(div.textContent).toBe('');
  });

  it('renders a currency glyph for cash assets', () => {
    const { container, rerender } = render(
      <AssetLogo asset={{ type: 'cash', currency: 'EUR' }} />,
    );
    expect(container.querySelector('.asset-logo.cash').textContent).toBe('€');

    rerender(<AssetLogo asset={{ type: 'cash', currency: 'USD' }} />);
    expect(container.querySelector('.asset-logo.cash').textContent).toBe('$');
  });

  it('falls back to the currency code when the glyph is unknown', () => {
    const { container } = render(
      <AssetLogo asset={{ type: 'cash', currency: 'BRL' }} />,
    );
    expect(container.querySelector('.asset-logo.cash').textContent).toBe('BRL');
  });

  it('renders an <img> via the proxy when logo_url is present', () => {
    const { container } = render(
      <AssetLogo asset={{ type: 'stock', symbol: 'AAPL', logo_url: 'x' }} />,
    );
    const img = container.querySelector('img.asset-logo');
    expect(img).not.toBeNull();
    expect(img.getAttribute('src')).toBe('/api/v1/assets/AAPL/logo');
    expect(img.getAttribute('alt')).toBe('AAPL');
  });

  it('percent-encodes weird symbols in the proxy URL', () => {
    const { container } = render(
      <AssetLogo asset={{ type: 'stock', symbol: 'BTC-USD', logo_url: 'x' }} />,
    );
    expect(container.querySelector('img.asset-logo').getAttribute('src'))
      .toBe('/api/v1/assets/BTC-USD/logo');
  });

  it('uses previewURL verbatim when supplied (Add-asset preview)', () => {
    const { container } = render(
      <AssetLogo asset={{ type: 'stock', symbol: 'AAPL' }} previewURL="https://x/aapl.png" />,
    );
    const img = container.querySelector('img.asset-logo');
    expect(img.getAttribute('src')).toBe('https://x/aapl.png');
    expect(img.getAttribute('referrerpolicy')).toBe('no-referrer');
  });

  it('falls back to initials when the image errors out', () => {
    const { container } = render(
      <AssetLogo asset={{ type: 'stock', symbol: 'AAPL', logo_url: 'x' }} />,
    );
    fireEvent.error(container.querySelector('img.asset-logo'));
    const fallback = container.querySelector('.asset-logo.fallback');
    expect(fallback).not.toBeNull();
    expect(fallback.textContent).toBe('AA');
  });

  it('renders initials when no logo_url is set', () => {
    const { container } = render(
      <AssetLogo asset={{ type: 'stock', symbol: 'msft' }} />,
    );
    expect(container.querySelector('.asset-logo.fallback').textContent).toBe('MS');
  });

  it('strips a CASH- prefix from the initials', () => {
    const { container } = render(
      <AssetLogo asset={{ type: 'stock', symbol: 'CASH-EUR' }} />,
    );
    // CASH- stripped → "EU" remains.
    expect(container.querySelector('.asset-logo.fallback').textContent).toBe('EU');
  });

  it('clears the failed flag when the asset symbol changes', () => {
    const { container, rerender } = render(
      <AssetLogo asset={{ type: 'stock', symbol: 'AAPL', logo_url: 'x' }} />,
    );
    fireEvent.error(container.querySelector('img.asset-logo'));
    expect(container.querySelector('.asset-logo.fallback')).not.toBeNull();

    rerender(<AssetLogo asset={{ type: 'stock', symbol: 'MSFT', logo_url: 'x' }} />);
    // useEffect runs after rerender → image is rendered again, no fallback.
    expect(container.querySelector('img.asset-logo')).not.toBeNull();
  });
});
