import { afterEach, expect, it, vi } from 'vitest';
import { ApiError, loadHistory, loadProducts } from './api';
import { jsonResponse, productHistory } from '../testing/apiFixture';

afterEach(() => vi.unstubAllGlobals());

it('follows catalog pagination and preserves string identities', async () => {
  const core = vi.fn().mockResolvedValueOnce(jsonResponse({ items: [{ id: '9007199254740993', label: 'Arroz' }], nextCursor: '9007199254740993' }))
    .mockResolvedValueOnce(jsonResponse({ items: [{ id: '9007199254740994', label: 'Café' }], nextCursor: null }));
  vi.stubGlobal('fetch', core);
  const products = await loadProducts(new AbortController().signal);
  expect(products.map((product) => product.id)).toEqual(['9007199254740993', '9007199254740994']);
  expect(core).toHaveBeenLastCalledWith('/api/v1/products?after=9007199254740993', expect.any(Object));
});

it('forwards only contract filters and returns authoritative values', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(productHistory)));
  const signal = new AbortController().signal;
  const result = await loadHistory({ product: '1', start: '2026-10-27', end: '2026-10-29' }, signal);
  expect(result).toEqual(productHistory);
  expect(fetch).toHaveBeenCalledWith('/api/v1/history?product=1&start=2026-10-27&end=2026-10-29', { signal, headers: { Accept: 'application/json' } });
});

it('centralizes HTTP failures', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ error: 'product_not_found' }, 404)));
  await expect(loadHistory({ product: '99', start: '2026-10-27', end: '2026-10-29' }, new AbortController().signal)).rejects.toEqual(new ApiError(404));
});
