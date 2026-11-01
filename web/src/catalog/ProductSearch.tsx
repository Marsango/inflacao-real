import { useState } from 'react';
import type { Product, SearchState } from '../data/model';
import { useProductSearch } from './useProductSearch';

type ResultsProps = { state: SearchState; choose: (product: Product) => void; retry: () => void };
type Props = { product: string; label?: string; group?: string; select: (identity: string) => void };

function SearchResults({ state, choose, retry }: ResultsProps) {
  if (state.status === 'idle') return null;
  if (state.status === 'loading') return <p role="status">Buscando produtos…</p>;
  if (state.status === 'error') return <p role="alert">Não foi possível buscar produtos. <button type="button" onClick={retry}>Tentar buscar novamente</button></p>;
  const products = state.page?.items ?? [];
  if (products.length === 0) return <p role="status">Nenhum produto encontrado. Tente outro nome ou marca.</p>;
  return <div className="search-results">
    <p role="status">{products.length} produto(s) encontrado(s).{state.page?.nextCursor && ' Há mais resultados; refine a busca por nome, marca ou tamanho.'}</p>
    <ul aria-label="Produtos encontrados">{products.map((product) => <li key={product.id}>
      <button type="button" onClick={() => choose(product)}>{product.label}</button>
    </li>)}</ul>
  </div>;
}

export function ProductSearch({ product, label, group, select }: Props) {
  const [query, setQuery] = useState('');
  const { state, retry } = useProductSearch(query, group);
  const choose = (result: Product) => { select(result.id); setQuery(''); };
  return <div className="product" role="search" aria-label="Busca de produtos">
    <label>Buscar produto<input type="search" maxLength={100} value={query} autoComplete="off"
      placeholder="Ex.: café, arroz 5 kg, marca…" aria-describedby="product-search-help"
      onChange={(event) => setQuery(event.target.value)}
      onKeyDown={(event) => { if (event.key === 'Escape') setQuery(''); }} /></label>
    <p id="product-search-help">Digite pelo menos 2 letras ou números para buscar por nome ou marca.{group && ' A busca considera apenas a categoria selecionada.'}</p>
    <div className="product-selection"><button type="button" aria-pressed={product === 'global'}
      onClick={() => { select('global'); setQuery(''); }}>Visão geral</button>
      {product !== 'global' && <p>Produto selecionado: <strong>{label ?? `Produto #${product}`}</strong></p>}</div>
    <SearchResults state={state} choose={choose} retry={retry} />
  </div>;
}
