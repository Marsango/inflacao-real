import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, test, vi } from 'vitest';
import { Scrapers } from './Scrapers';
const scraper = { scraper_id: 'collector', shop_id: 'mambo', scraper_name: 'Mambo', status: 'warning', started_at: '2026-11-01T12:00:00Z', observations: 20, errors: 0, warnings: 1, omitted: 0 };
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });
function mockReports() {
  let deleted = false;
  return vi.fn(async (path: string, options?: RequestInit) => {
    if (path.endsWith('/scrapers')) return Response.json([scraper]);
    if (options?.method === 'DELETE') { deleted = true; return new Response(null, { status: 204 }); }
    return Response.json(deleted ? [] : [{ id: '1', run_id: 'run', severity: 'warning', category: 'collection', message: 'Package mismatch', started_at: scraper.started_at }]);
  });
}
test('shows retailer names and deletes diagnostics with CSRF while preserving health', async () => {
  const request = mockReports();
  vi.stubGlobal('fetch', request); vi.spyOn(window, 'confirm').mockReturnValue(true);
  render(<Scrapers csrf="token" />);
  fireEvent.click(await screen.findByRole('button', { name: 'Revisar Mambo' }));
  expect(await screen.findByText('Package mismatch')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Excluir diagnóstico' }));
  await waitFor(() => expect(screen.getByText('Nenhum diagnóstico encontrado.')).toBeInTheDocument());
  expect(request).toHaveBeenCalledWith(expect.stringContaining('id=1'), expect.objectContaining({ method: 'DELETE', headers: expect.objectContaining({ 'X-CSRF-Token': 'token' }) }));
  expect(screen.getByText(/Atenção/)).toBeInTheDocument();
});
test('offers recovery after a failed request', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 401 })));
  render(<Scrapers csrf="token" />);
  expect(await screen.findByRole('alert')).toHaveTextContent('Confira sua sessão');
  expect(screen.getByRole('button', { name: 'Atualizar coletores' })).toBeInTheDocument();
});
