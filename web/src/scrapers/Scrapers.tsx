import { useScrapers } from './useScrapers';
import type { Scraper } from './api';
import { Diagnostics } from './Diagnostics';

const labels = { healthy: 'Saudável', warning: 'Atenção', error: 'Erro', unknown: 'Sem relatório' };
export function Scrapers({ csrf }: { csrf: string }) {
  const { rows, selected, select, error, refresh } = useScrapers();
  return <section><h2>Coletores</h2><p>Saúde da coleta informada na última execução. O processamento dos preços no servidor é separado.</p>
    <button onClick={() => void refresh()}>Atualizar coletores</button>{error && <p role="alert">{error}</p>}
    {!rows && !error && <p role="status">Carregando coletores…</p>}
    {rows?.length === 0 && <p>Nenhum coletor disponível.</p>}
    <ul>{rows?.map(row => <li key={`${row.scraper_id}/${row.shop_id}`}><ScraperSummary row={row} /> <button onClick={() => select(row)}>Revisar {row.scraper_name}</button></li>)}</ul>
    {selected && <Diagnostics key={`${selected.scraper_id}/${selected.shop_id}`} scraper={selected} csrf={csrf} />}
  </section>;
}
function ScraperSummary({ row }: { row: Scraper }) {
  return <><strong>{row.scraper_name}</strong> — {labels[row.status]}<p>{row.scraper_id} / {row.shop_id}</p>
    <p>Última execução: {row.started_at ? new Date(row.started_at).toLocaleString('pt-BR', { timeZone: 'America/Sao_Paulo' }) : 'Não informada'} (horário de Brasília).</p>
    {row.started_at && <p>Observações: {row.observations}; erros: {row.errors}; avisos: {row.warnings}; detalhes omitidos: {row.omitted}.</p>}</>;
}
