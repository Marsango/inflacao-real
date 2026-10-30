import { useEffect, useState } from 'react';
import { loadHistory } from '../data/api';
import type { HistoryRequest, LoadState } from '../data/model';
import { validPeriod } from '../shared/dates';

export function useHistory(request: HistoryRequest) {
  const [result, setResult] = useState<{ key: string; state: LoadState }>();
  const [revision, setRevision] = useState(0);
  const { product, start, end } = request;
  const key = JSON.stringify({ product, start, end, revision });
  useEffect(() => {
    if (!validPeriod({ start, end })) return;
    const controller = new AbortController();
    loadHistory({ product, start, end }, controller.signal).then(
      (history) => { if (!controller.signal.aborted) setResult({ key, state: { status: 'ready', history } }); },
      () => { if (!controller.signal.aborted) setResult({ key, state: { status: 'error' } }); },
    );
    return () => controller.abort();
  }, [key]);
  const state: LoadState = result?.key === key ? result.state : { status: 'loading' };
  return { state, retry: () => setRevision((value) => value + 1) };
}
