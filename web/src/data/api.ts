import type { History, HistoryRequest, ProductGroup, ProductPage } from './model';

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
  if (request.group) query.set('group', request.group);
  return get<History>(`history?${query}`, signal);
}

export function searchProducts(search: string, signal: AbortSignal, group = ''): Promise<ProductPage> {
  const query = new URLSearchParams({ q: search, limit: '20' });
  if (group) query.set('group', group);
  return get<ProductPage>(`products?${query}`, signal);
}

export function loadGroups(signal: AbortSignal): Promise<ProductGroup[]> {
  return get<ProductGroup[]>('groups', signal);
}
