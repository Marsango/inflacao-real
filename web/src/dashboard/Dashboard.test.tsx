import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { Dashboard } from './Dashboard';
import { monthsBefore, today } from '../shared/dates';
import { lineSegments } from './chartGeometry';
import { catalog, jsonResponse, mockCore, productHistory } from '../testing/apiFixture';

beforeEach(() => {
  window.history.replaceState(null, '', '/');
  vi.stubGlobal('fetch', mockCore());
});
afterEach(() => vi.unstubAllGlobals());

it('loads the overview from core and provides an accessible table', async () => {
  render(<Dashboard />);
  expect(screen.getByRole('button', { name: 'Último mês' })).toHaveAttribute('aria-pressed', 'true');
  expect(await screen.findByRole('heading', { name: 'Variação dos produtos monitorados' })).toBeVisible();
  expect(screen.getByText('+8,03%')).toBeVisible();
  fireEvent.click(screen.getByText('Ver dados em tabela'));
  expect(screen.getByRole('table')).toBeVisible();
  expect(screen.getByText('Sem preço comparável')).toBeVisible();
  expect(fetch).toHaveBeenCalledWith(expect.stringContaining('history?product=global&start='), expect.objectContaining({ signal: expect.any(AbortSignal) }));
});

it('displays methodology and freshness supplied by core', async () => {
  render(<Dashboard />);
  await screen.findByRole('img');
  expect(screen.getByText(/Última coleta no período/)).toBeVisible();
  fireEvent.click(screen.getByText('Sobre os dados e a metodologia'));
  expect(screen.getByText(productHistory.methodology)).toBeVisible();
  expect(screen.queryByText(/Dados fictícios/)).not.toBeInTheDocument();
});

it('selects products from the catalog and forwards the period', async () => {
  render(<Dashboard />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'arroz' } });
  await screen.findByRole('button', { name: 'Arroz branco · 5 kg' });
  fireEvent.click(screen.getByRole('button', { name: 'Últimos 6 meses' }));
  fireEvent.click(screen.getByRole('button', { name: 'Arroz branco · 5 kg' }));
  expect(await screen.findByRole('heading', { name: productHistory.title })).toBeVisible();
  expect(screen.getByText(/R\$\s*26,90/, { selector: 'strong' })).toBeVisible();
  expect(window.location.search).toContain('product=1');
  expect(screen.getByLabelText('De')).toHaveValue(monthsBefore(today(), 6));
  expect(fetch).toHaveBeenLastCalledWith(`/api/v1/history?product=1&start=${monthsBefore(today(), 6)}&end=${today()}`, expect.any(Object));
});

it('validates dates and does not send invalid requests', async () => {
  render(<Dashboard />);
  await screen.findByRole('img');
  const requests = vi.mocked(fetch).mock.calls.length;
  fireEvent.change(screen.getByLabelText('De'), { target: { value: '2099-01-01' } });
  expect(screen.getByRole('alert')).toHaveTextContent('Informe datas válidas');
  expect(fetch).toHaveBeenCalledTimes(requests);
  expect(screen.queryByRole('img')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Último ano' }));
  expect(await screen.findByRole('img')).toBeVisible();
});

it('restores real product IDs and dates from a shared URL', async () => {
  window.history.replaceState(null, '', '/?product=2&start=2026-10-27&end=2026-10-29');
  render(<Dashboard />);
  await screen.findByRole('heading', { name: productHistory.title });
  expect(screen.getByText('Produto selecionado:')).toHaveTextContent(productHistory.title);
  expect(vi.mocked(fetch).mock.calls.some(([path]) => String(path).includes('/products'))).toBe(false);
  expect(fetch).toHaveBeenCalledWith('/api/v1/history?product=2&start=2026-10-27&end=2026-10-29', expect.any(Object));
});

it('retries failed requests without changing filters', async () => {
  const core = mockCore();
  core.mockImplementationOnce(async () => jsonResponse({ error: 'data_unavailable' }, 500));
  vi.stubGlobal('fetch', core);
  render(<Dashboard />);
  expect(await screen.findByRole('alert')).toHaveTextContent('Não foi possível carregar o histórico');
  fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
  expect(await screen.findByRole('img')).toBeVisible();
  expect(window.location.search).toBe('');
});

it('recovers a failed catalog independently', async () => {
  const core = mockCore();
  core.mockImplementation(async (path) => path.includes('/products') ? jsonResponse({}, 500) : jsonResponse(productHistory));
  vi.stubGlobal('fetch', core);
  render(<Dashboard />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'cafe' } });
  expect(await screen.findByRole('alert')).toHaveTextContent('Não foi possível buscar produtos');
  expect(screen.getByLabelText('Buscar produto')).toBeEnabled();
  core.mockImplementation(async () => jsonResponse(catalog));
  fireEvent.click(screen.getByText('Tentar buscar novamente'));
  expect(await screen.findByRole('button', { name: 'Café · 500 g' })).toBeVisible();
});

it('shows empty history without inventing a collection timestamp', async () => {
  vi.stubGlobal('fetch', mockCore({ ...productHistory, points: [], collectedAt: null, latestPriceMinor: null, changeBasisPoints: null, counts: { ...productHistory.counts, products: 0, pricedDays: 0 } }));
  render(<Dashboard />);
  expect(await screen.findByText(/Nenhum preço comparável/)).toBeVisible();
  expect(screen.getByText('Sem coleta no período.')).toBeVisible();
});

it('shows insufficient history and stale data from core', async () => {
  vi.stubGlobal('fetch', mockCore({ ...productHistory, points: [productHistory.points[0]], changeBasisPoints: null, stale: true, counts: { ...productHistory.counts, pricedDays: 1 } }));
  render(<Dashboard />);
  expect(await screen.findByText(/Histórico insuficiente/)).toBeVisible();
  expect(screen.getByText(/Dados desatualizados/)).toBeVisible();
  expect(screen.queryByRole('img')).not.toBeInTheDocument();
});

it('keeps loading while a request is pending and aborts on unmount', async () => {
  const core = vi.fn(() => new Promise<Response>(() => {}));
  vi.stubGlobal('fetch', core);
  const view = render(<Dashboard />);
  expect(screen.getByText('Carregando histórico…')).toBeVisible();
  view.unmount();
  expect(vi.mocked(fetch).mock.calls[0][1]?.signal?.aborted).toBe(true);
});

it('does not connect the chart across missing observations', () => {
  const points = [{ date: '1', value: 100 }, { date: '2', value: null }, { date: '3', value: 200 }];
  expect(lineSegments(points, Number, Number)).toBe('M1,100  M3,200');
});

it('labels per-kg prices and explains unverified buying conditions', async () => {
  vi.stubGlobal('fetch', mockCore({ ...productHistory, priceBasis: 'kg', unknownConditions: true }));
  render(<Dashboard />);
  expect(await screen.findByRole('img', { name: /Preço por kg/ })).toBeVisible();
  expect(screen.getByText(/Condições de compra não informadas/)).toBeVisible();
});
