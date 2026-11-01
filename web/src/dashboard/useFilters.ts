import { useEffect, useState } from 'react';
import { monthsBefore, today, validDate, type Period } from '../shared/dates';

export type Filters = Period & { product: string; preset: string; group: string };

function readFilters(): Filters {
  const query = new URLSearchParams(window.location.search);
  const end = query.get('end') ?? today();
  const start = query.get('start') ?? monthsBefore(today(), 1);
  const product = query.get('product') ?? 'global';
  return { product, group: query.get('group') ?? '',
    start: validDate(start) ? start : monthsBefore(today(), 1),
    end: validDate(end) ? end : today(),
    preset: query.get('preset') ?? (query.has('start') || query.has('end') ? 'custom' : '1') };
}

function writeFilters(filters: Filters): void {
  const url = new URL(window.location.href);
  Object.entries(filters).forEach(([key, value]) => url.searchParams.set(key, value));
  window.history.replaceState(null, '', url);
}

export function useFilters() {
  const [filters, setFilters] = useState(readFilters);
  useEffect(() => {
    const restore = () => setFilters(readFilters());
    window.addEventListener('popstate', restore);
    return () => window.removeEventListener('popstate', restore);
  }, []);
  function update(change: Partial<Filters>) {
    const next = { ...filters, ...change };
    setFilters(next);
    writeFilters(next);
  }
  return { filters, update };
}
