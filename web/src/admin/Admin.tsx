import { useState } from 'react';
import { useAdmin } from './useAdmin';
import { Login } from './Login';
import { Review } from './Review';
import { Groups } from './Groups';
import { CatalogReview } from './CatalogReview';
import type { Candidate } from './api';
import { AuditLog } from './AuditLog';
import './admin.css';

export function Admin() {
  const admin = useAdmin();
  const [selected, setSelected] = useState<Candidate | null>(null);
  return <main className="page-width admin"><a href="/">Nossa inflação</a><h1>Administração de equivalências</h1>
    {admin.error && <p role="alert">{admin.error}</p>}
    {!admin.session ? <Login login={admin.login} busy={admin.busy} /> : <>
      <p>Responsável: {admin.session.username}</p><button disabled={admin.busy} onClick={() => void admin.logout()}>Sair</button>{' '}
      <button disabled={admin.busy} onClick={() => { setSelected(null); void admin.refresh(); }}>Atualizar catálogo</button>
      {!admin.snapshot ? <p role="status">Carregando catálogo…</p> : <AdminCatalog admin={admin} selected={selected} select={setSelected} />}
    </>}</main>;
}
function AdminCatalog({ admin, selected, select }: { admin: ReturnType<typeof useAdmin>; selected: Candidate | null; select: (value: Candidate | null) => void }) {
  const snapshot = admin.snapshot!;
  return <><p>Versão da revisão: {snapshot.revision}. Agrupamento automático: {snapshot.genericEnabled ? 'ativo' : 'inativo'}.</p>
    {selected && <Review key={`${snapshot.revision}-${selected.first}-${selected.second}`} candidate={selected} products={snapshot.products} busy={admin.busy} pending={() => select(null)} decide={async decision => { if (await admin.decide(decision)) select(null); }} />}
    <CatalogReview snapshot={snapshot} select={select} /><GenericPreview admin={admin} select={select} />
    <Groups products={snapshot.products} audit={snapshot.audit} decide={admin.decide} busy={admin.busy} />
    <AuditLog records={snapshot.audit} /></>;
}
function GenericPreview({ admin, select }: { admin: ReturnType<typeof useAdmin>; select: (candidate: Candidate) => void }) {
  const [reason, setReason] = useState('');
  const snapshot = admin.snapshot!;
  return <section><h2>Prévia de agrupamento genérico ({snapshot.generic.length} pares)</h2>
    <p>Revise a prévia antes de ativar. A ativação aplica estas regras ao histórico e às próximas observações. Desativar interrompe novas decisões; use a reversão para desfazer grupos existentes.</p>
    <ul>{snapshot.generic.map(pair => <li key={`${pair.first}-${pair.second}`}>#{pair.first} e #{pair.second}: {pair.reason} <button onClick={() => select(pair)}>Conferir par genérico</button></li>)}</ul>
    <label>Motivo da ativação ou desativação<input value={reason} onChange={event => setReason(event.target.value)} /></label>
    <button disabled={admin.busy || !reason.trim()} onClick={() => void admin.decide({ action: snapshot.genericEnabled ? 'deactivate' : 'activate', reason })}>{snapshot.genericEnabled ? 'Desativar regras automáticas' : 'Ativar regras e aplicar prévia'}</button></section>;
}
