import { useEffect, useState } from 'react';
import { loadProducts } from '../data/api';
import type { CatalogState } from '../data/model';

export function useCatalog() {
  const [state, setState] = useState<CatalogState>({ status: 'loading', products: [] });
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setState({ status: 'loading', products: [] });
    loadProducts(controller.signal).then(
      (products) => { if (!controller.signal.aborted) setState({ status: 'ready', products }); },
      () => { if (!controller.signal.aborted) setState({ status: 'error', products: [] }); },
    );
    return () => controller.abort();
  }, [revision]);
  return { ...state, retry: () => setRevision((value) => value + 1) };
}
