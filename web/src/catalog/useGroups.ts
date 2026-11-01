import { useEffect, useState } from 'react';
import { loadGroups } from '../data/api';
import type { ProductGroup } from '../data/model';

type State = { status: 'loading' | 'error' | 'ready'; groups: ProductGroup[] };

export function useGroups() {
  const [state, setState] = useState<State>({ status: 'loading', groups: [] });
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setState({ status: 'loading', groups: [] });
    loadGroups(controller.signal).then(
      (groups) => { if (!controller.signal.aborted) setState({ status: 'ready', groups }); },
      () => { if (!controller.signal.aborted) setState({ status: 'error', groups: [] }); },
    );
    return () => controller.abort();
  }, [revision]);
  return { ...state, retry: () => setRevision((value) => value + 1) };
}
