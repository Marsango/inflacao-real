import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ProductSearch } from './ProductSearch';
import { catalog, jsonResponse } from '../testing/apiFixture';

beforeEach(() => vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(catalog))));
afterEach(() => vi.unstubAllGlobals());

it('makes no catalogue request on mount or for a short or punctuation-only query', () => {
  render(<ProductSearch product="global" select={vi.fn()} />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'a' } });
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: '***' } });
  expect(fetch).not.toHaveBeenCalled();
});

it('debounces typing and requests just 20 results without following pages', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ ...catalog, nextCursor: '2' })));
  const select = vi.fn();
  render(<ProductSearch product="global" select={select} />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'ca' } });
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'café' } });
  fireEvent.click(await screen.findByRole('button', { name: 'Café · 500 g' }));
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(fetch).toHaveBeenCalledWith('/api/v1/products?q=caf%C3%A9&limit=20', expect.any(Object));
  expect(select).toHaveBeenCalledWith('2');
  expect(screen.getByLabelText('Buscar produto')).toHaveValue('');
});

it('explains that more matches require a more specific search', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ ...catalog, nextCursor: '2' })));
  render(<ProductSearch product="global" select={vi.fn()} />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'arroz' } });
  expect(await screen.findByText(/refine a busca/)).toBeVisible();
});

it('aborts obsolete requests and ignores late responses', async () => {
  let resolveOld!: (response: Response) => void;
  const core = vi.fn().mockImplementationOnce(() => new Promise<Response>((resolve) => { resolveOld = resolve; }))
    .mockResolvedValue(jsonResponse(catalog));
  vi.stubGlobal('fetch', core);
  render(<ProductSearch product="global" select={vi.fn()} />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'arroz' } });
  await waitFor(() => expect(core).toHaveBeenCalledTimes(1));
  const signal = core.mock.calls[0][1].signal as AbortSignal;
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'café' } });
  expect(signal.aborted).toBe(true);
  await screen.findByRole('button', { name: 'Café · 500 g' });
  await act(async () => { resolveOld(jsonResponse({ items: [{ id: '3', label: 'Resultado antigo' }], nextCursor: null })); });
  expect(screen.queryByText('Resultado antigo')).not.toBeInTheDocument();
});

it('keeps the selected product when no matches are found and can return to the overview', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ items: [], nextCursor: null })));
  const select = vi.fn();
  render(<ProductSearch product="9007199254740993" label="Arroz 5 kg" select={select} />);
  fireEvent.change(screen.getByLabelText('Buscar produto'), { target: { value: 'inexistente' } });
  expect(await screen.findByText(/Nenhum produto encontrado/)).toBeVisible();
  expect(screen.getByText('Arroz 5 kg')).toBeVisible();
  expect(select).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Visão geral' }));
  expect(select).toHaveBeenCalledWith('global');
});
