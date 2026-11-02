import type { History, ShopHistory } from '../data/model';
import { formatDate } from '../shared/dates';
import { money, priceLabel } from '../shared/format';

type Endpoints = { start: string; end: string };

function Price({ shop, date }: { shop: ShopHistory; date: string }) {
  const value = shop.points.find(point => point.date === date)?.value;
  return <td className="shop-price">{value == null ? <span className="missing-price">Sem preço disponível</span> : money(value)}</td>;
}

function ShopRow({ shop, dates }: { shop: ShopHistory; dates: Endpoints }) {
  return <tr>
    <th scope="row"><strong>{shop.shopName}</strong><span className="shop-context">{shop.context}</span></th>
    <Price shop={shop} date={dates.start} />
    {dates.start !== dates.end && <Price shop={shop} date={dates.end} />}
    <td><span className={`comparison-status ${shop.included ? 'included' : ''}`}>
      {shop.included ? 'Incluído na variação' : 'Fora da variação'}
    </span></td>
    <td><a className="offer-link" href={shop.productUrl} target="_blank" rel="noreferrer"
      aria-label={`Ver oferta de ${shop.shopName} (abre em nova aba)`}>Ver oferta <span aria-hidden="true">↗</span></a></td>
  </tr>;
}

function ComparisonTable({ history, dates }: { history: History; dates: Endpoints }) {
  return <div className="table-scroll" role="region" aria-label="Preços por loja" tabIndex={0}>
    <table className="shop-prices-table"><caption>{priceLabel(history.priceBasis)} por loja</caption>
      <thead><tr><th scope="col">Loja e local de compra</th>
        <th scope="col">{formatDate(dates.start)}<span className="date-position">{dates.start === dates.end ? 'Data disponível' : 'Início'}</span></th>
        {dates.start !== dates.end && <th scope="col">{formatDate(dates.end)}<span className="date-position">Fim</span></th>}
        <th scope="col">Comparação no período</th><th scope="col">No site da loja</th>
      </tr></thead>
      <tbody>{history.shopPrices.map((shop, index) => <ShopRow key={`${shop.shopId}-${index}`} shop={shop} dates={dates} />)}</tbody>
    </table>
  </div>;
}

export function ShopPrices({ history }: { history: History }) {
  if (history.unit !== 'BRL' || history.shopPrices.length === 0) return null;
  const dates = history.availablePeriod ?? history.period;
  return <section className="shop-prices" aria-labelledby="shop-prices-title"><h3 id="shop-prices-title">Comparar preços entre lojas</h3>
    <p>Preços anunciados na primeira e na última data com dados no período. Cada linha representa uma oferta e seu local de compra.</p>
    <p className="table-scroll-hint">Role a tabela para o lado para ver todos os preços e ofertas.</p>
    <ComparisonTable history={history} dates={dates} />
    <p className="comparison-note">Ofertas sem preços comparáveis nas duas datas ficam fora da variação. Indisponibilidade não é preço zero.</p>
  </section>;
}
