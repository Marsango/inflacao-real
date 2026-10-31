import type { Audit } from './api';
const actions: Record<string, string> = { approve: 'Agrupar', reject: 'Rejeitar', remove: 'Remover membro', dissolve: 'Dissolver', representative: 'Escolher representante', activate: 'Ativar regras', deactivate: 'Desativar regras' };
export function AuditLog({ records }: { records: Audit[] }) {
  return <details><summary>Auditoria: últimas 100 decisões</summary>
    <ol>{records.map(record => <li key={record.id}><p>{new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeStyle: 'short', timeZone: 'America/Sao_Paulo' }).format(new Date(record.occurred_at))}
      {' · '}{record.actor === 'automatic' ? 'Automático' : record.actor}{' · '}{actions[record.action] ?? record.action}{' · revisão '}{record.revision}</p>
      <p>{record.reason}{record.rule_version && ` · Regra: ${record.rule_version}`}</p>
      <p>Produtos: {[record.decision.first, record.decision.second].filter(Boolean).join(', ') || 'Catálogo'}</p>
      <p>Composição anterior: {members(record.before_membership)}. Composição posterior: {members(record.after_membership)}.</p></li>)}</ol>
  </details>;
}
function members(memberships: Audit['before_membership']) { return memberships.map(member => `#${member.product_id} → #${member.representative_id}`).join('; ') || 'sem grupos'; }
