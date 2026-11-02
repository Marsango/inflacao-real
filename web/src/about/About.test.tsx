import { render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { App } from '../App';
import { Footer } from '../layout/Footer';

afterEach(() => { window.history.replaceState(null, '', '/'); vi.unstubAllGlobals(); });

it.each(['/sobre', '/sobre/'])('loads the about page directly at %s without fetching history', (path) => {
  window.history.replaceState(null, '', path);
  vi.stubGlobal('fetch', vi.fn());
  render(<App />);
  expect(screen.getByRole('heading', { level: 1, name: 'Sobre a Nossa inflação' })).toBeVisible();
  expect(screen.getByRole('region', { name: 'Sobre o projeto' })).toHaveAttribute('id', 'projeto');
  expect(screen.getByRole('region', { name: 'Sobre os dados e a metodologia' })).toHaveAttribute('id', 'metodologia');
  expect(screen.getByText(/O recorte monitorado não representa a inflação nacional/)).toBeVisible();
  expect(screen.getByRole('link', { name: '← Explorar preços' })).toHaveAttribute('href', '/');
  expect(fetch).not.toHaveBeenCalled();
});

it('replaces expanding footer sections with links and keeps the contribution link', () => {
  render(<Footer />);
  const footer = screen.getByRole('contentinfo');
  expect(within(footer).getByRole('link', { name: 'Sobre os dados e a metodologia' })).toHaveAttribute('href', '/sobre#metodologia');
  expect(within(footer).getByRole('link', { name: 'Sobre o projeto' })).toHaveAttribute('href', '/sobre#projeto');
  expect(within(footer).getByRole('link', { name: 'Contribua para o projeto' })).toHaveAttribute('href', 'https://github.com/Marsango/nossa-inflacao');
  expect(footer.querySelector('details')).toBeNull();
});
