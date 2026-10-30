import { monthsBefore, presets, today } from '../shared/dates';
import type { CatalogState } from '../data/model';
import type { Filters } from './useFilters';

type Props = { filters: Filters; update: (change: Partial<Filters>) => void; invalid: boolean };

function PeriodButtons({ filters, update }: Props) {
  const labels = ['Último mês', 'Últimos 3 meses', 'Últimos 6 meses', 'Último ano'];
  return <div className="presets" role="group" aria-label="Períodos rápidos">
    {presets.map((months, index) => <button key={months} type="button"
      aria-pressed={filters.preset === String(months) && filters.end === today() && filters.start === monthsBefore(today(), months)}
      onClick={() => update({ start: monthsBefore(today(), months), end: today(), preset: String(months) })}>
      {labels[index]}</button>)}
  </div>;
}

function DateInputs({ filters, update, invalid }: Props) {
  return <div className="dates">
    <label>De<input type="date" value={filters.start} aria-invalid={invalid}
      aria-describedby={invalid ? 'date-error' : undefined} onChange={(event) => update({ start: event.target.value, preset: 'custom' })} /></label>
    <label>Até<input type="date" value={filters.end} aria-invalid={invalid}
      aria-describedby={invalid ? 'date-error' : undefined} onChange={(event) => update({ end: event.target.value, preset: 'custom' })} /></label>
  </div>;
}

export function Controls(props: Props & { catalog: CatalogState; retryCatalog: () => void }) {
  return <section className="controls" aria-label="Filtros do histórico">
    <label className="product">Produto<select value={props.filters.product} disabled={props.catalog.status !== 'ready'}
      onChange={(event) => props.update({ product: event.target.value })}>
      <option value="global">Visão geral</option>
      {props.filters.product !== 'global' && !props.catalog.products.some((item) => item.id === props.filters.product) &&
        <option value={props.filters.product}>Produto #{props.filters.product}</option>}
      {props.catalog.products.map((product) => <option key={product.id} value={product.id}>{product.label}</option>)}
    </select></label>
    {props.catalog.status === 'loading' && <p role="status">Carregando produtos…</p>}
    {props.catalog.status === 'error' && <p role="alert">Não foi possível carregar os produtos. <button onClick={props.retryCatalog}>Tentar carregar produtos</button></p>}
    <fieldset><legend>Período</legend><PeriodButtons {...props} /><DateInputs {...props} /></fieldset>
    {props.invalid && <p className="notice" id="date-error" role="alert">Informe datas válidas. A data inicial deve ser anterior ou igual à data final, com intervalo de até 366 dias.</p>}
  </section>;
}
