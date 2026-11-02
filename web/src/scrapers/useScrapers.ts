import { useEffect, useState } from 'react';
import { fetchScrapers, type Scraper } from './api';
export function useScrapers() {
  const [rows, setRows] = useState<Scraper[] | null>(null);
  const [selected, select] = useState<Scraper | null>(null);
  const [error, setError] = useState('');
  async function refresh() {
    setError('');
    try { setRows(await fetchScrapers()); }
    catch { setError('Não foi possível carregar os coletores. Confira sua sessão e tente novamente.'); }
  }
  useEffect(() => { void refresh(); }, []);
  return { rows, selected, select, error, refresh };
}
