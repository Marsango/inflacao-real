import { useDiagnostics } from './useDiagnostics';
import type { Scraper } from './api';

export function Diagnostics({ scraper, csrf }: { scraper: Scraper; csrf: string }) {
  const { rows, error, severity, setSeverity, before, setBefore, busy, reload, remove } = useDiagnostics(scraper, csrf);
  return <section aria-label={`Diagnósticos de ${scraper.scraper_name}`}><h3>Diagnósticos de {scraper.scraper_name}</h3>
    <p>Inclui execuções anteriores. A exclusão dos detalhes preserva a saúde da última execução.</p>
    <label>Gravidade <select value={severity} onChange={event => { setSeverity(event.target.value); setBefore(''); }}><option value="">Todas</option><option value="error">Erros</option><option value="warning">Avisos</option></select></label>{' '}
    <button disabled={busy} onClick={() => void remove()}>Excluir todos após análise</button>{' '}
    <button onClick={reload}>Atualizar diagnósticos</button>
    {error && <p role="alert">{error}</p>}{!rows && !error && <p role="status">Carregando diagnósticos…</p>}
    {rows?.length === 0 && <p>Nenhum diagnóstico encontrado.</p>}
    <ul>{rows?.map(row => <li key={row.id}><p>{row.severity === 'error' ? 'Erro' : 'Aviso'} — {row.category} — {new Date(row.started_at).toLocaleString('pt-BR', { timeZone: 'America/Sao_Paulo' })} (Brasília)</p>
      <p>{row.message}</p><small>Execução: {row.run_id}</small>{' '}<button disabled={busy} onClick={() => void remove(row.id)}>Excluir diagnóstico</button></li>)}</ul>
    {rows?.length === 50 && <button onClick={() => setBefore(rows[rows.length - 1].id)}>Mais antigos</button>}
    {before && <button onClick={() => setBefore('')}>Mais recentes</button>}
  </section>;
}
