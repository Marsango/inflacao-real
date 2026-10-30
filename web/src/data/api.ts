import type { History, HistoryRequest, Product } from './model';

type ProductPage = { items: Product[]; nextCursor: string | null };

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

export async function loadProducts(signal: AbortSignal): Promise<Product[]> {
  const products: Product[] = [];
  let cursor: string | null = null;
  do {
    const query: string = cursor ? `?after=${encodeURIComponent(cursor)}` : '';
    const page: ProductPage = await get<ProductPage>(`products${query}`, signal);
    if (page.nextCursor !== null && page.nextCursor === cursor) throw new Error('Invalid catalog cursor');
    products.push(...page.items);
    cursor = page.nextCursor;
  } while (cursor !== null);
  return products;
}
