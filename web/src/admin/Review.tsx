import { useState, type FormEvent } from 'react';
import type { Candidate, Decision, Product } from './api';
import { ProductDetails } from './ProductDetails';

interface Props { candidate: Candidate; products: Product[]; busy: boolean; decide: (decision: Decision) => Promise<unknown>; pending: () => void }
export function Review({ candidate, products, busy, decide, pending }: Props) {
  const [reason, setReason] = useState('');
  const [verified, setVerified] = useState(false);
  const first = products.find(product => product.id === candidate.first)!;
  const second = products.find(product => product.id === candidate.second)!;
  function submit(event: FormEvent) { event.preventDefault(); void decide({ action: 'approve', first: first.id, second: second.id, reason, sourceVerified: verified }); }
  return <article className="admin-review"><h2>Revisar equivalência</h2><p>{candidate.kind === 'exact' ? 'Sugestão exata normalizada' : candidate.kind === 'generic' ? 'Regra de produto genérico' : 'Sugestão aproximada'} · {candidate.reason}</p>
    {candidate.kind === 'fuzzy' && <p>Similaridade de palavras: {Math.round(candidate.score * 100)}%. Esta pontuação não é uma probabilidade de identidade.</p>}
    <p>Palavras diferentes: {candidate.differences.join(', ') || 'Nenhuma'}</p>
    <div className="admin-pair"><ProductDetails product={first} /><ProductDetails product={second} /></div>
    <form onSubmit={submit}><label>Explicação da decisão<textarea required maxLength={2000} value={reason} onChange={event => setReason(event.target.value)} /></label>
      <label><input type="checkbox" checked={verified} onChange={event => setVerified(event.target.checked)} />Conferi as fontes e confirmei variantes e embalagem, incluindo informações omitidas.</label>
      {candidate.verificationRequired && <p>Verifique as fontes antes de aprovar. Informação omitida não confirma equivalência.</p>}
      <button disabled={busy || !reason.trim() || (candidate.verificationRequired && !verified)}>Aprovar equivalência</button>
      <button type="button" disabled={busy || !reason.trim()} onClick={() => void decide({ action: 'reject', first: first.id, second: second.id, reason })}>Rejeitar</button>
      <button type="button" onClick={pending}>Deixar pendente</button></form></article>;
}
