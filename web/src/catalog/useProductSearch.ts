import { useEffect, useState } from 'react';
import { searchProducts } from '../data/api';
import type { SearchState } from '../data/model';

type Result = { key: string; state: SearchState };

function searchable(query: string) {
  return (query.match(/[\p{L}\p{N}]/gu)?.length ?? 0) >= 2;
}

function requestSearch(query: string, signal: AbortSignal, complete: (state: SearchState) => void) {
  searchProducts(query, signal).then(
    (page) => { if (!signal.aborted) complete({ status: 'ready', page }); },
    () => { if (!signal.aborted) complete({ status: 'error' }); },
  );
}

export function useProductSearch(input: string) {
  const query = input.trim();
  const [result, setResult] = useState<Result>();
  const [revision, setRevision] = useState(0);
  const key = JSON.stringify({ query, revision });
  useEffect(() => {
    if (!searchable(query)) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => requestSearch(query, controller.signal,
      (state) => setResult({ key, state })), 300);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [key]);
  const state: SearchState = result?.key === key ? result.state : { status: searchable(query) ? 'loading' : 'idle' };
  return { state, retry: () => setRevision((value) => value + 1) };
}
