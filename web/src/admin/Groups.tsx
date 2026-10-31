import { useState } from 'react';
import type { Audit, Decision, Product } from './api';
import { ProductDetails } from './ProductDetails';

export function Groups({ products, audit, decide, busy }: { products: Product[]; audit: Audit[]; decide: (decision: Decision) => Promise<unknown>; busy: boolean }) {
  const [reason, setReason] = useState('');
  const groups = products.filter(product => products.some(other => other.id !== product.id && other.representative === product.id));
  return <section><h2>Grupos e reversão</h2><label>Motivo da alteração<input value={reason} maxLength={2000} onChange={event => setReason(event.target.value)} /></label>
    {!groups.length && <p>Nenhum grupo aprovado ou automático.</p>}
    {groups.map(group => <details key={group.id}><summary>Grupo #{group.id} · {group.name}</summary><p>{groupReason(group.id, audit)}</p>
      <button disabled={busy || !reason.trim()} onClick={() => void decide({ action: 'dissolve', first: group.id, reason })}>Dissolver grupo</button>
      {products.filter(product => product.representative === group.id).map(product => <div key={product.id}><ProductDetails product={product} />
        <button disabled={busy || !reason.trim()} onClick={() => void decide({ action: 'remove', first: product.id, reason })}>Remover do grupo</button>
        <button disabled={busy || !reason.trim() || product.id === group.id} onClick={() => void decide({ action: 'representative', first: group.id, representative: product.id, reason })}>Usar como representante</button>
      </div>)}</details>)}</section>;
}

function groupReason(representative: number, records: Audit[]) {
  const record = records.find(record => record.action === 'approve' && record.after_membership.some(member => member.representative_id === representative));
  return record ? `${record.actor === 'automatic' ? 'Automático' : 'Aprovado pelo responsável'}: ${record.reason}${record.rule_version ? ` · ${record.rule_version}` : ''}` : 'Consulte a auditoria completa para a decisão original deste grupo.';
}
