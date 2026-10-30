import { Controls } from './Controls';
import { HistoryView } from './HistoryView';
import { useFilters } from './useFilters';
import { useHistory } from './useHistory';
import { useCatalog } from './useCatalog';
import { validPeriod } from '../shared/dates';
import { Footer } from '../layout/Footer';

export function Dashboard() {
  const { filters, update } = useFilters();
  const { state, retry } = useHistory(filters);
  const catalog = useCatalog();
  const invalid = !validPeriod(filters);
  return <><header><div className="page-width header-content"><a className="brand" href="/">Nossa inflação</a><span>Monitoramento independente de preços</span></div></header>
    <main className="page-width"><div className="intro"><h1>Histórico de preços</h1>
      <p className="demo-note">Preços coletados no varejo brasileiro</p></div>
      <Controls catalog={catalog} retryCatalog={catalog.retry} filters={filters} update={update} invalid={invalid} />
      {!invalid && <HistoryView state={state} period={filters} retry={retry} />}
    </main><Footer history={!invalid && state.status === 'ready' ? state.history : undefined} /></>;
}
