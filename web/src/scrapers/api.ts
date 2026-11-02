import { adminRequest } from '../admin/api';

export interface Scraper {
  scraper_id: string; shop_id: string; scraper_name: string;
  status: 'healthy' | 'warning' | 'error' | 'unknown'; started_at: string | null;
  finished_at: string | null; observations: number | null; errors: number | null;
  warnings: number | null; omitted: number | null;
}
export interface Diagnostic {
  id: string; run_id: string; severity: string; category: string; message: string; started_at: string;
}
export function query(scraper: Scraper) {
  return new URLSearchParams({ scraper_id: scraper.scraper_id, shop_id: scraper.shop_id });
}
export const fetchScrapers = () => adminRequest<Scraper[]>('scrapers');
export function fetchDiagnostics(scraper: Scraper, severity: string, before: string) {
  const parameters = query(scraper);
  if (severity) parameters.set('severity', severity);
  if (before) parameters.set('before', before);
  return adminRequest<Diagnostic[]>(`scraper-diagnostics?${parameters}`);
}
export function deleteDiagnostics(scraper: Scraper, csrf: string, id?: string) {
  const parameters = query(scraper);
  if (id) parameters.set('id', id);
  return adminRequest<void>(`scraper-diagnostics?${parameters}`, 'DELETE', undefined, csrf);
}
