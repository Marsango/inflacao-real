import type { History, HistoryRequest, ProductPage } from './model';

export class ApiError extends Error {
  constructor(readonly status: number) {
    super(`Core request failed (${status})`);
  }
}

async function get<Response>(path: string, signal: AbortSignal): Promise<Response> {
  const response = await fetch(`/api/v1/${path}`, { signal, headers: { Accept: 'application/json' } });
  if (!response.ok) throw new ApiError(response.status);
  return response.json() as Promise<Response>;
}

export function loadHistory(request: HistoryRequest, signal: AbortSignal): Promise<History> {
  const query = new URLSearchParams({ product: request.product, start: request.start, end: request.end });
  return get<History>(`history?${query}`, signal);
}

export function searchProducts(search: string, signal: AbortSignal): Promise<ProductPage> {
  const query = new URLSearchParams({ q: search, limit: '20' });
  return get<ProductPage>(`products?${query}`, signal);
}
