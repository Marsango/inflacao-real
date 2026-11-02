import { Controls } from './Controls';
import { HistoryView } from './HistoryView';
import { useFilters } from './useFilters';
import { useHistory } from './useHistory';
import { validPeriod } from '../shared/dates';
import { Footer } from '../layout/Footer';
import { Header } from '../layout/Header';

export function Dashboard() {
  const { filters, update } = useFilters();
  const { state, retry } = useHistory(filters);
  const invalid = !validPeriod(filters);
  return <><Header />
    <main className="page-width"><div className="intro"><h1>Histórico de preços</h1>
      <p className="demo-note">Preços coletados no varejo brasileiro</p></div>
      <Controls productLabel={state.status === 'ready' ? state.history?.title : undefined} filters={filters} update={update} invalid={invalid} />
      {!invalid && <HistoryView state={state} period={filters} retry={retry} />}
    </main><Footer /></>;
}
