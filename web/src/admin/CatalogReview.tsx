import { useState } from 'react';
import type { Candidate, Product, Snapshot } from './api';

export function CatalogReview({ snapshot, select }: { snapshot: Snapshot; select: (candidate: Candidate) => void }) {
  const [query, setQuery] = useState('');
  const [first, setFirst] = useState(0);
  const products = snapshot.products.filter(product => `${product.id} ${product.name} ${product.brand ?? ''}`.toLocaleLowerCase('pt-BR').includes(query.toLocaleLowerCase('pt-BR')));
  function manual(product: Product) { select({ first, second: product.id, kind: 'manual', score: 0, differences: [], reason: 'Seleção manual: confirme a comparabilidade nas fontes.', verificationRequired: true }); }
  return <section><h2>Buscar produtos e revisar pares</h2><label>Nome, marca ou código interno<input value={query} onChange={event => setQuery(event.target.value)} /></label>
    <p>Selecione dois produtos para revisão manual. Primeiro produto: {first || 'nenhum'}.</p>
    <ul>{products.slice(0, 50).map(product => <li key={product.id}>#{product.id} · {product.name}{' '}
      <button onClick={() => setFirst(product.id)}>Selecionar primeiro</button>{' '}
      <button disabled={!first || first === product.id} onClick={() => manual(product)}>Comparar com o primeiro</button></li>)}</ul>
    <h2>Sugestões pendentes ({snapshot.candidates.length})</h2>
    <ul>{snapshot.candidates.map(candidate => <li key={`${candidate.first}-${candidate.second}`}>
      #{candidate.first} e #{candidate.second} · {candidate.kind === 'exact' ? 'Exata' : `Aproximada (${Math.round(candidate.score * 100)}%)`}{' '}
      <button onClick={() => select(candidate)}>Revisar par</button></li>)}</ul></section>;
}
