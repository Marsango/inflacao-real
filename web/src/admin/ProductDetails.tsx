import type { Product } from './api';

export function ProductDetails({ product }: { product: Product }) {
  const size = product.package;
  return <section className="admin-product" aria-label={`Produto ${product.id}`}><h3>#{product.id} · {product.name}</h3>
    <dl><dt>Marca</dt><dd>{product.brand ?? 'Não informada'}</dd><dt>Código de barras</dt><dd>{product.barcode ?? 'Não informado'}</dd>
      <dt>Embalagem</dt><dd>{size ? `${size.Count} × ${size.Quantity} ${size.Unit}` : 'Não informada'}</dd>
      <dt>Base de venda</dt><dd>{product.bases.join(', ') || 'Não informada'}</dd>
      <dt>Cobertura</dt><dd>{product.observations} observações · {product.listings.length} anúncios</dd>
      <dt>Período coletado</dt><dd>{observedDate(product.firstObserved)} a {observedDate(product.lastObserved)}</dd>
      <dt>Representante</dt><dd>#{product.representative}</dd></dl>
    <ul>{product.listings.map((listing, index) => <li key={index}><a href={listing.url} target="_blank" rel="noreferrer">{listing.shop} · {listing.name}</a>
      {' · '}{listing.brand ?? 'Marca não informada'}{' · '}{listing.barcode ?? 'Código não informado'}</li>)}</ul></section>;
}

function observedDate(value?: string | null) { return value ? new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeZone: 'America/Sao_Paulo' }).format(new Date(value)) : 'Não informado'; }
