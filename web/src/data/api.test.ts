import { afterEach, expect, it, vi } from 'vitest';
import { ApiError, loadHistory, searchProducts } from './api';
import { jsonResponse, productHistory } from '../testing/apiFixture';

afterEach(() => vi.unstubAllGlobals());

it('requests one bounded search page without following its cursor', async () => {
  const page = { items: [{ id: '9007199254740993', label: 'Café' }], nextCursor: '9007199254740993' };
  const core = vi.fn().mockResolvedValue(jsonResponse(page));
  vi.stubGlobal('fetch', core);
  const products = await searchProducts('café união', new AbortController().signal);
  expect(products).toEqual(page);
  expect(core).toHaveBeenCalledTimes(1);
  expect(core).toHaveBeenCalledWith('/api/v1/products?q=caf%C3%A9+uni%C3%A3o&limit=20', expect.any(Object));
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
