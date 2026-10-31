import type { History } from '../data/model';
import { formatDate } from '../shared/dates';
import { money, priceLabel } from '../shared/format';

export function ShopPrices({ history }: { history: History }) {
  if (history.unit !== 'BRL' || history.shopPrices.length === 0) return null;
  return <section aria-labelledby="shop-prices-title"><h3 id="shop-prices-title">Comparar preços entre lojas</h3>
    <p>Preços anunciados por loja e contexto de compra. Indisponibilidade não entra como variação de preço.</p>
    <div className="table-scroll"><table><caption>{priceLabel(history.priceBasis)} por loja e data</caption>
      <thead><tr><th scope="col">Loja e contexto</th><th scope="col">Comparação no período</th>
        {history.shopPrices[0].points.map((point) => <th key={point.date} scope="col">{formatDate(point.date)}</th>)}</tr></thead>
      <tbody>{history.shopPrices.map((shop, index) => <tr key={`${shop.shopId}-${index}`}>
        <th scope="row"><a href={shop.productUrl} target="_blank" rel="noreferrer">{shop.shopName}</a><br />{shop.context}</th>
        <td>{shop.included ? 'Incluído' : 'Sem preços comparáveis nas duas datas'}</td>
        {shop.points.map((point) => <td key={point.date}>{point.value === null ? 'Sem preço disponível' : money(point.value)}</td>)}
      </tr>)}</tbody>
    </table></div>
  </section>;
}
