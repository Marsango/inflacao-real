export interface Listing { name: string; brand: string | null; barcode: string | null; url: string; shop: string }
export interface Product {
  id: number; name: string; brand: string | null; barcode: string | null;
  package: { Quantity: string; Unit: string; Count: number } | null;
  firstObserved?: string | null; lastObserved?: string | null; bases: string[]; listings: Listing[]; observations: number; representative: number;
}
export interface Candidate { first: number; second: number; kind: string; score: number; differences: string[]; reason: string; verificationRequired: boolean }
export interface Snapshot { revision: number; genericEnabled: boolean; products: Product[]; candidates: Candidate[]; generic: Candidate[]; audit: Audit[] }
export interface Session { username: string; csrf: string }
export interface Decision { action: string; first?: number; second?: number; representative?: number; reason: string; sourceVerified?: boolean }

export async function adminRequest<T>(path: string, method = 'GET', data?: unknown, csrf?: string): Promise<T> {
  const response = await fetch(`/api/v1/admin/${path}`, { method, credentials: 'same-origin',
    headers: { Accept: 'application/json', ...(data ? { 'Content-Type': 'application/json' } : {}), ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
    ...(data ? { body: JSON.stringify(data) } : {}) });
  if (!response.ok) throw new AdminError(response.status);
  return response.status === 204 ? undefined as T : response.json() as Promise<T>;
}
function errorMessage(status: number) {
  if (status === 409) return 'O catálogo mudou. Atualize a revisão antes de decidir.';
  if (status === 401) return 'Entre com as credenciais do responsável. A sessão pode ter expirado.';
  if (status === 429) return 'Muitas tentativas. Aguarde 15 minutos.';
  if (status === 422) return 'Decisão incompatível, excluída anteriormente ou sem verificação das fontes.';
  if (status === 404) return 'Administração indisponível ou desativada.';
  return 'Não foi possível concluir. Atualize e tente novamente.';
}

export interface Audit { id: number; occurred_at: string; actor: string; action: string; reason: string; rule_version: string | null; revision: number; before_membership: Membership[]; after_membership: Membership[]; decision: Decision }
export interface Membership { product_id: number; representative_id: number }

export class AdminError extends Error {
  constructor(readonly status: number) { super(errorMessage(status)); }
}
