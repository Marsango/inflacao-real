import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { Dashboard } from './Dashboard';
import { monthsBefore, today } from '../shared/dates';
import { geometry, lineSegments } from './chartGeometry';
import { catalog, groups, jsonResponse, mockCore, productHistory } from '../testing/apiFixture';

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
  fireEvent.click(screen.getByText('Metodologia deste histórico'));
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
  core.mockImplementation(async (path) => path === '/api/v1/groups' ? jsonResponse(groups) : jsonResponse({}, 500));
  vi.stubGlobal('fetch', core);
  render(<Dashboard />);
  expect(await screen.findByRole('alert')).toHaveTextContent('Não foi possível carregar o histórico');
  core.mockImplementation(mockCore());
  fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
  expect(await screen.findByRole('img')).toBeVisible();
  expect(window.location.search).toBe('');
});

it('recovers a failed catalog independently', async () => {
  const core = mockCore();
  const fallback = mockCore();
  core.mockImplementation(async (path) => path.includes('/products') ? jsonResponse({}, 500) : fallback(path));
  vi.stubGlobal('fetch', core);
  render(<Dashboard />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'cafe' } });
  expect(await screen.findByRole('alert')).toHaveTextContent('Não foi possível buscar produtos');
  expect(screen.getByLabelText('Buscar produto')).toBeEnabled();
  core.mockImplementation(async () => jsonResponse(catalog));
  fireEvent.click(screen.getByText('Tentar buscar novamente'));
  expect(await screen.findByRole('button', { name: 'Café · 500 g' })).toBeVisible();
});

it('selects a category, resets the product and preserves the selected dates', async () => {
  window.history.replaceState(null, '', '/?product=2&start=2026-10-27&end=2026-10-29');
  render(<Dashboard />);
  fireEvent.click(await screen.findByRole('button', { name: 'Cesta básica' }));
  expect(await screen.findByRole('heading', { name: 'Variação de preços · Cesta básica' })).toBeVisible();
  expect(screen.getByRole('button', { name: 'Cesta básica' })).toHaveAttribute('aria-pressed', 'true');
  expect(screen.getByText(/Não representa uma cesta básica oficial/)).toBeVisible();
  expect(window.location.search).toContain('group=cesta-basica');
  expect(window.location.search).toContain('product=global');
  expect(fetch).toHaveBeenCalledWith('/api/v1/history?product=global&start=2026-10-27&end=2026-10-29&group=cesta-basica', expect.any(Object));
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'arroz' } });
  fireEvent.click(await screen.findByRole('button', { name: 'Arroz branco · 5 kg' }));
  expect(fetch).toHaveBeenCalledWith('/api/v1/products?q=arroz&limit=20&group=cesta-basica', expect.any(Object));
  expect(await screen.findByRole('heading', { name: productHistory.title })).toBeVisible();
  expect(fetch).toHaveBeenLastCalledWith('/api/v1/history?product=1&start=2026-10-27&end=2026-10-29&group=cesta-basica', expect.any(Object));
});

it('restores a shared category and can return to all products', async () => {
  window.history.replaceState(null, '', '/?group=hortifruti&start=2026-10-27&end=2026-10-29');
  render(<Dashboard />);
  expect(await screen.findByRole('heading', { name: 'Variação de preços · Hortifruti' })).toBeVisible();
  expect(screen.getByRole('button', { name: 'Hortifruti' })).toHaveAttribute('aria-pressed', 'true');
  expect(vi.mocked(fetch).mock.calls.some(([path]) => String(path).includes('/products'))).toBe(false);
  fireEvent.click(screen.getByRole('button', { name: 'Todos os produtos' }));
  expect(await screen.findByRole('heading', { name: 'Variação dos produtos monitorados' })).toBeVisible();
  expect(fetch).toHaveBeenLastCalledWith('/api/v1/history?product=global&start=2026-10-27&end=2026-10-29', expect.any(Object));
});

it('retries category loading independently of history', async () => {
  const fallback = mockCore();
  const core = mockCore();
  core.mockImplementation(async (path) => path === '/api/v1/groups' ? jsonResponse({}, 500) : fallback(path));
  vi.stubGlobal('fetch', core);
  render(<Dashboard />);
  expect(await screen.findByRole('alert')).toHaveTextContent('Não foi possível carregar as categorias');
  expect(await screen.findByRole('heading', { name: 'Variação dos produtos monitorados' })).toBeVisible();
  core.mockImplementation(fallback);
  fireEvent.click(screen.getByRole('button', { name: 'Tentar carregar categorias novamente' }));
  expect(await screen.findByRole('button', { name: 'Carnes' })).toBeVisible();
  fireEvent.click(screen.getByRole('button', { name: 'Doces' }));
  expect(await screen.findByRole('heading', { name: 'Variação de preços · Doces' })).toBeVisible();
});

it('restores category selection after browser navigation', async () => {
  render(<Dashboard />);
  await screen.findByRole('button', { name: 'Carnes' });
  window.history.replaceState(null, '', '/?group=carnes&product=global&start=2026-10-27&end=2026-10-29');
  fireEvent.popState(window);
  expect(await screen.findByRole('heading', { name: 'Variação de preços · Carnes' })).toBeVisible();
  expect(screen.getByRole('button', { name: 'Carnes' })).toHaveAttribute('aria-pressed', 'true');
});

it('shows empty history without inventing a collection timestamp', async () => {
  vi.stubGlobal('fetch', mockCore({ ...productHistory, availablePeriod: null, points: [], collectedAt: null, latestPriceMinor: null, changeBasisPoints: null, counts: { ...productHistory.counts, products: 0, pricedDays: 0 } }));
  render(<Dashboard />);
  expect(await screen.findByText(/Nenhum preço disponível/)).toBeVisible();
  expect(screen.getByText('Sem coleta no período.')).toBeVisible();
});

it('shows insufficient history and stale data from core', async () => {
  vi.stubGlobal('fetch', mockCore({ ...productHistory, availablePeriod: { start: '2026-10-27', end: '2026-10-27' }, points: [productHistory.points[0]], changeBasisPoints: null, comparisonStart: null, comparisonEnd: null, stale: true, counts: { ...productHistory.counts, pricedDays: 1 } }));
  render(<Dashboard />);
  expect(await screen.findByText(/Histórico insuficiente/)).toBeVisible();
  expect(screen.getByText(/Dados desatualizados/)).toBeVisible();
  expect(screen.getByRole('img')).toBeVisible();
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

it('shows available shop prices when no matched variation exists', async () => {
  const points = [{ date: '2026-10-27', value: null }, { date: '2026-10-29', value: 2690 }];
  const excluded = { ...productHistory.shopPrices[0], included: false, points };
  const history = { ...productHistory, points: productHistory.points,
    shopPrices: [excluded], changeBasisPoints: null, comparisonStart: null, comparisonEnd: null,
    counts: { ...productHistory.counts, pricedDays: 2, includedProducts: 0, includedShops: 0, includedStreams: 0 } };
  vi.stubGlobal('fetch', mockCore(history));
  render(<Dashboard />);
  expect(await screen.findByRole('heading', { name: 'Comparar preços entre lojas' })).toBeVisible();
  expect(screen.getByRole('link', { name: /Ver oferta de Mercado Exemplo/ })).toHaveAttribute('href', excluded.productUrl);
  expect(screen.getByText('Fora da variação')).toBeVisible();
  expect(screen.getAllByText('Sem preço disponível')[0]).toBeVisible();
  expect(screen.getByText(/R\$\s*26,90/, { selector: 'strong' })).toBeVisible();
  expect(screen.getByRole('img')).toBeVisible();
  expect(screen.getByText(/Menor preço disponível por dia/)).toBeVisible();
  expect(screen.getByText(/Esses valores não permitem calcular uma variação comparável/)).toBeVisible();
});

it('shows São Paulo pickup coverage, retailer and public product link from core', async () => {
  const context = 'São Paulo/SP · CEP 04263100 · Retirada';
  const coverage = 'Localidades observadas: Curitiba/PR; São Paulo/SP.';
  const productUrl = 'https://www.lojahirota.com.br/produto/498/arroz-camil-tipo-1-5kg';
  const shop = { ...productHistory.shopPrices[0], shopId: 'hirota', shopName: 'Hirota', context, productUrl };
  vi.stubGlobal('fetch', mockCore({ ...productHistory, coverage, shopPrices: [shop] }));
  render(<Dashboard />);
  expect(await screen.findByRole('link', { name: /Ver oferta de Hirota/ })).toHaveAttribute('href', productUrl);
  expect(screen.getByText(context)).toBeVisible();
  expect(screen.getAllByText(coverage)[0]).toBeVisible();
});

it('shows available dates below the graph when selecting past and future endpoints', async () => {
  window.history.replaceState(null, '', '/?start=2026-10-27&end=2026-10-29');
  render(<Dashboard />);
  await screen.findByRole('img');
  fireEvent.change(screen.getByLabelText('De'), { target: { value: '2026-10-01' } });
  await screen.findByRole('img');
  fireEvent.change(screen.getByLabelText('Até'), { target: { value: '2027-01-01' } });
  const chart = await screen.findByRole('img');
  const explanation = screen.getByText(/preços disponíveis apenas entre 27\/10\/2026 e 29\/10\/2026/);
  expect(chart).toBeVisible();
  expect(explanation).toBeVisible();
  expect(chart.compareDocumentPosition(explanation) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(screen.getByText('Comparação: 27/10/2026 a 29/10/2026')).toBeVisible();
  expect(screen.getByLabelText('De')).toHaveValue('2026-10-01');
  expect(screen.getByLabelText('Até')).toHaveValue('2027-01-01');
  expect(fetch).toHaveBeenLastCalledWith('/api/v1/history?product=global&start=2026-10-01&end=2027-01-01', expect.any(Object));
});

it('shows a single overview day without claiming a calculated percentage', async () => {
  const single = { ...productHistory, unit: 'percent' as const, availablePeriod: { start: '2026-10-27', end: '2026-10-27' },
    points: [{ date: '2026-10-27', value: 0 }], changeBasisPoints: null, comparisonStart: null, comparisonEnd: null,
    counts: { ...productHistory.counts, pricedDays: 1 }, shopPrices: [] };
  vi.stubGlobal('fetch', mockCore(single));
  render(<Dashboard />);
  const chart = await screen.findByRole('img');
  expect(chart).toBeVisible();
  expect(screen.getByText(/Base de referência/)).toHaveTextContent('Variação não calculável');
  expect(screen.getByText(/preços disponíveis apenas em 27\/10\/2026/)).toBeVisible();
  expect(screen.queryByText(/Comparação:/)).not.toBeInTheDocument();
  expect(chart.querySelectorAll('circle')).toHaveLength(1);
  expect(chart.querySelectorAll('.chart-labels > text')).toHaveLength(1);
});

it('centers a single chart point with finite coordinates', () => {
  const point = { date: '2026-10-27', value: 2490 };
  const { x, y, plot } = geometry([point], { width: 500, height: 300 });
  expect(x(point.date)).toBe((plot.left + plot.right) / 2);
  expect(Number.isFinite(y(point.value))).toBe(true);
});


it('shows Pão de Açúcar delivery coverage and public product link from core', async () => {
  const context = 'São Paulo/SP · CEP 04115090 · Entrega';
  const productUrl = 'https://www.paodeacucar.com/produto/172/arroz-integral-tipo-1-tio-joao-pacote-500g';
  const shop = { ...productHistory.shopPrices[0], shopId: 'pao-de-acucar',
    shopName: 'Pão de Açúcar', context, productUrl };
  vi.stubGlobal('fetch', mockCore({ ...productHistory, shopPrices: [shop] }));
  render(<Dashboard />);
  expect(await screen.findByRole('link', { name: /Ver oferta de Pão de Açúcar/ })).toHaveAttribute('href', productUrl);
  expect(screen.getByText(context)).toBeVisible();
});
