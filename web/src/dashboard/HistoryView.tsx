import type { History, LoadState } from '../data/model';
import type { Period } from '../shared/dates';
import { formatDate, timeZone } from '../shared/dates';
import { chartValue, money, percentage, priceLabel } from '../shared/format';
import { Chart } from './Chart';

function Summary({ history }: { history: History }) {
  return <div className="summary">
    {history.unit === 'BRL' && <div><span>{priceLabel(history.priceBasis)} · último no período</span><strong>{history.latestPriceMinor === null ? 'Indisponível' : money(history.latestPriceMinor)}</strong></div>}
    <div><span>Variação no período</span><strong>{history.changeBasisPoints === null ? 'Indisponível' : percentage(history.changeBasisPoints)}</strong></div>
    {history.comparisonStart && history.comparisonEnd && <p>Comparação: {formatDate(history.comparisonStart)} a {formatDate(history.comparisonEnd)}</p>}
  </div>;
}

function DataTable({ history }: { history: History }) {
  return <details><summary>Ver dados em tabela</summary><div className="table-scroll"><table>
    <caption>{history.title} — série consolidada</caption>
    <thead><tr><th scope="col">Data</th><th scope="col">{history.unit === 'BRL' ? priceLabel(history.priceBasis) : 'Variação (%)'}</th></tr></thead>
    <tbody>{history.points.map((point) => <tr key={point.date}><td>{formatDate(point.date)}</td>
      <td>{point.value === null ? 'Sem preço comparável' : chartValue(point.value, history.unit)}</td></tr>)}</tbody>
  </table></div></details>;
}

function Context({ history }: { history: History }) {
  const collected = history.collectedAt && new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeStyle: 'short', timeZone }).format(new Date(history.collectedAt));
  return <div className="context"><p>{history.coverage}</p>
    {history.unknownConditions && <p className="notice" role="status">Condições de compra não informadas em parte das ofertas. São preços anunciados e podem exigir condições não verificadas.</p>}
    <p>{collected ? `Última coleta no período: ${collected} (horário de Brasília).` : 'Sem coleta no período.'}</p>
    <DataTable history={history} />
  </div>;
}

function HistoryContent({ history }: { history: History }) {
  const observed = history.points.filter((point) => point.value !== null);
  if (observed.length === 0) return <><p className="notice" role="status">Nenhum preço comparável neste período. Escolha outro intervalo ou produto.</p><Context history={history} /></>;
  return <>
    {history.stale && <p className="notice" role="status">Dados desatualizados. A última coleta disponível pode não refletir os preços atuais.</p>}
    {observed.length < 2 ? <p className="notice" role="status">Histórico insuficiente para mostrar uma tendência. Amplie o período.</p> : <Chart history={history} />}
    <Context history={history} />
  </>;
}

export function HistoryView({ state, period, retry }: { state: LoadState; period: Period; retry: () => void }) {
  if (state.status === 'loading') return <p className="notice" role="status">Carregando histórico…</p>;
  if (state.status === 'error') return <div className="notice" role="alert"><p>Não foi possível carregar o histórico.</p><button onClick={retry}>Tentar novamente</button></div>;
  if (!state.history) return null;
  return <section className="history" aria-labelledby="history-title">
    <div className="history-top"><div className="history-heading"><h2 id="history-title">{state.history.title}</h2>
      <p>{formatDate(period.start)} a {formatDate(period.end)}</p></div><Summary history={state.history} /></div>
    <HistoryContent history={state.history} />
  </section>;
}
