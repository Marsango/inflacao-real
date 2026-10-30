import type { History } from '../data/model';
import { chartValue, priceLabel } from '../shared/format';
import { formatDate } from '../shared/dates';
import { geometry, lineSegments } from './chartGeometry';
import { useChartSize, type ChartSize } from './useChartSize';

function Axes({ history, size }: { history: History; size: ChartSize }) {
  const { x, y, ticks, plot } = geometry(history.points, size);
  const ends = [history.points[0], history.points.at(-1)!];
  return <g className="chart-labels">
    {ticks.map((tick) => <g key={tick}><line x1={plot.left} x2={plot.right} y1={y(tick)} y2={y(tick)} />
      <text x={plot.left - 14} y={y(tick) + 4} textAnchor="end">{chartValue(Math.round(tick), history.unit)}</text></g>)}
    {ends.map((point, index) => <text key={`${point.date}-${index}`} x={x(point.date)} y={size.height - 8}
      textAnchor={index === 0 ? 'start' : 'end'}>{formatDate(point.date)}</text>)}
  </g>;
}

export function Chart({ history }: { history: History }) {
  const { container, size } = useChartSize();
  const { x, y } = geometry(history.points, size);
  const missing = history.points.some((point) => point.value === null);
  return <figure className="chart">
    <div className="chart-plot" ref={container}><svg viewBox={`0 0 ${size.width} ${size.height}`} role="img" aria-labelledby="chart-title chart-description">
      <title id="chart-title">{history.title} — {history.unit === 'BRL' ? priceLabel(history.priceBasis) : 'variação em porcentagem'}</title>
      <desc id="chart-description">Histórico por data. Interrupções indicam ausência de preço comparável. Os valores estão disponíveis na tabela abaixo.</desc>
      <Axes history={history} size={size} />
      <path className="price-line" d={lineSegments(history.points, x, y)} />
      {history.points.map((point) => point.value === null ? null : <circle key={point.date} cx={x(point.date)} cy={y(point.value)} r="3.5">
        <title>{formatDate(point.date)}: {chartValue(point.value, history.unit)}</title></circle>)}
    </svg></div>
    <figcaption>{history.unit === 'BRL' ? priceLabel(history.priceBasis) : 'Variação acumulada no período (%)'} · Datas no eixo horizontal.{missing && ' Há uma lacuna no gráfico: sem preço comparável no período.'}</figcaption>
  </figure>;
}
