import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, test, vi } from 'vitest';
import { Admin } from '../Admin';

const products = [1, 2].map(id => ({ id, name: `Café ${id}`, brand: 'Pilão', barcode: null, package: { Quantity: '500', Unit: 'g', Count: 1 }, bases: ['package'], listings: [{ name: `Café ${id}`, brand: 'Pilão', barcode: null, url: 'https://example.com', shop: 'Mercado' }], observations: 2, representative: id }));
const snapshot = { revision: 4, genericEnabled: false, products, generic: [], audit: [], candidates: [{ first: 1, second: 2, kind: 'fuzzy', score: .91, differences: ['vacuo'], reason: 'Conferir embalagem', verificationRequired: true }] };
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
afterEach(() => vi.unstubAllGlobals());

function mockApi(loggedIn = true) {
  const request = vi.fn(async (path: string, options?: RequestInit) => {
    if (options?.method === 'POST' && path.endsWith('/session')) return json({ username: 'owner', csrf: 'csrf-token' });
    if (path.endsWith('/session')) return loggedIn ? json({ username: 'owner', csrf: 'csrf-token' }) : json({}, 401);
    if (path.endsWith('/reviews')) return new Response(null, { status: 204 });
    return json(snapshot);
  });
  vi.stubGlobal('fetch', request); return request;
}

test('login uses Portuguese labels and opens review catalog', async () => {
  const request = mockApi(false); render(<Admin />);
  fireEvent.change(screen.getByLabelText('Usuário'), { target: { value: 'owner' } });
  fireEvent.change(screen.getByLabelText('Senha'), { target: { value: 'secret' } });
  fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
  expect(await screen.findByText('Sugestões pendentes (1)')).toBeInTheDocument();
  expect(request).toHaveBeenCalledWith('/api/v1/admin/session', expect.objectContaining({ method: 'POST', body: JSON.stringify({ username: 'owner', password: 'secret' }) }));
});

test('fuzzy review exposes words and requires explanation and source verification', async () => {
  const request = mockApi(); render(<Admin />);
  fireEvent.click(await screen.findByRole('button', { name: 'Revisar par' }));
  expect(screen.getByText(/Palavras diferentes: vacuo/)).toBeInTheDocument();
  expect(screen.getByText(/não é uma probabilidade/)).toBeInTheDocument();
  const approve = screen.getByRole('button', { name: 'Aprovar equivalência' });
  expect(approve).toBeDisabled();
  fireEvent.change(screen.getByLabelText('Explicação da decisão'), { target: { value: 'Embalagem conferida nas fontes' } });
  expect(approve).toBeDisabled();
  fireEvent.click(screen.getByRole('checkbox')); fireEvent.click(approve);
  await waitFor(() => expect(request).toHaveBeenCalledWith('/api/v1/admin/reviews', expect.objectContaining({ headers: expect.objectContaining({ 'X-CSRF-Token': 'csrf-token' }), body: expect.stringContaining('"revision":4') })));
});

test('pending leaves memberships untouched', async () => {
  const request = mockApi(); render(<Admin />);
  fireEvent.click(await screen.findByRole('button', { name: 'Revisar par' }));
  fireEvent.click(screen.getByRole('button', { name: 'Deixar pendente' }));
  expect(screen.queryByRole('heading', { name: 'Revisar equivalência' })).not.toBeInTheDocument();
  expect(request.mock.calls.some(([path]) => path.endsWith('/reviews'))).toBe(false);
});

test('rejection persists an explained decision without requiring approval verification', async () => {
  const request = mockApi(); render(<Admin />);
  fireEvent.click(await screen.findByRole('button', { name: 'Revisar par' }));
  const reject = screen.getByRole('button', { name: 'Rejeitar' });
  expect(reject).toBeDisabled();
  fireEvent.change(screen.getByLabelText('Explicação da decisão'), { target: { value: 'Embalagens diferentes nas fontes' } });
  fireEvent.click(reject);
  await waitFor(() => expect(request).toHaveBeenCalledWith('/api/v1/admin/reviews', expect.objectContaining({ body: expect.stringContaining('"action":"reject"') })));
});

test('stale review shows a recovery message and keeps the pair open', async () => {
  const request = mockApi();
  request.mockImplementation(async (path: string) => path.endsWith('/reviews') ? json({}, 409) : path.endsWith('/session') ? json({ username: 'owner', csrf: 'csrf-token' }) : json(snapshot));
  render(<Admin />); fireEvent.click(await screen.findByRole('button', { name: 'Revisar par' }));
  fireEvent.change(screen.getByLabelText('Explicação da decisão'), { target: { value: 'Conferido' } });
  fireEvent.click(screen.getByRole('checkbox')); fireEvent.click(screen.getByRole('button', { name: 'Aprovar equivalência' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('O catálogo mudou');
  expect(screen.getByRole('heading', { name: 'Revisar equivalência' })).toBeInTheDocument();
  expect(request.mock.calls.filter(([path]) => path.endsWith('/reviews'))).toHaveLength(1);
});

test('expired sessions return to the login form', async () => {
  const request = mockApi();
  request.mockImplementation(async (path: string) => path.endsWith('/session') ? json({ username: 'owner', csrf: 'csrf-token' }) : json({}, 401));
  render(<Admin />);
  expect(await screen.findByRole('alert')).toHaveTextContent('A sessão pode ter expirado');
  expect(screen.getByLabelText('Usuário')).toBeInTheDocument();
});

test('group reversal requires a reason and sends the member ID', async () => {
  const request = mockApi();
  request.mockImplementation(async (path: string) => path.endsWith('/session') ? json({ username: 'owner', csrf: 'csrf-token' }) : path.endsWith('/reviews') ? new Response(null, { status: 204 }) : json({ ...snapshot, candidates: [], products: products.map(product => ({ ...product, representative: 1 })) }));
  render(<Admin />); await screen.findByText('Grupo #1 · Café 1');
  const remove = screen.getAllByRole('button', { name: 'Remover do grupo', hidden: true })[0];
  expect(remove).toBeDisabled();
  fireEvent.change(screen.getByLabelText('Motivo da alteração'), { target: { value: 'Preparação incompatível' } });
  fireEvent.click(remove);
  await waitFor(() => expect(request).toHaveBeenCalledWith('/api/v1/admin/reviews', expect.objectContaining({ body: expect.stringContaining('"action":"remove","first":1') })));
});
