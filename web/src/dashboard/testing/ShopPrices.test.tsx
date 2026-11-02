import { render, screen, within } from '@testing-library/react';
import { expect, it } from 'vitest';
import { ShopPrices } from '../ShopPrices';
import { productHistory } from '../../testing/apiFixture';
import { formatDate } from '../../shared/dates';

function longHistory() {
  const points = Array.from({ length: 1000 }, (_, index) => ({
    date: new Date(Date.UTC(2024, 0, 1 + index)).toISOString().slice(0, 10), value: 2490 + index,
  }));
  const availablePeriod = { start: points[0].date, end: points[999].date };
  return { ...productHistory, availablePeriod, points, shopPrices: [{ ...productHistory.shopPrices[0], points }] };
}

it('keeps only the effective endpoints in a thousand-day comparison', () => {
  const history = longHistory();
  render(<ShopPrices history={history} />);
  expect(screen.getAllByRole('columnheader')).toHaveLength(5);
  expect(screen.getByRole('columnheader', { name: `${formatDate(history.points[0].date)} Início` })).toBeVisible();
  expect(screen.getByRole('columnheader', { name: `${formatDate(history.points[999].date)} Fim` })).toBeVisible();
  expect(screen.queryByText(formatDate(history.points[500].date))).not.toBeInTheDocument();
  expect(screen.getAllByRole('cell')).toHaveLength(4);
});

it('renders one date for a single-day history and no false variation', () => {
  const availablePeriod = { start: '2026-10-27', end: '2026-10-27' };
  const shop = { ...productHistory.shopPrices[0], included: false, points: [productHistory.points[0]] };
  render(<ShopPrices history={{ ...productHistory, availablePeriod, shopPrices: [shop] }} />);
  expect(screen.getAllByRole('columnheader')).toHaveLength(4);
  expect(screen.getByRole('columnheader', { name: '27/10/2026 Data disponível' })).toBeVisible();
  expect(screen.getByText('Fora da variação')).toBeVisible();
});

it('keeps separate offers and missing endpoint prices without substituting nearby prices', () => {
  const first = { ...productHistory.shopPrices[0], context: 'Curitiba/PR · CEP 80730201 · Entrega' };
  const second = { ...first, included: false, points: [productHistory.points[1]] };
  render(<ShopPrices history={{ ...productHistory, shopPrices: [first, second] }} />);
  const rows = screen.getAllByRole('row');
  expect(rows).toHaveLength(3);
  expect(within(rows[1]).getByText('Incluído na variação')).toBeVisible();
  expect(within(rows[2]).getAllByText('Sem preço disponível')).toHaveLength(2);
  expect(within(rows[2]).getByText('Fora da variação')).toBeVisible();
  expect(screen.getAllByText(first.context)).toHaveLength(2);
});

it('provides a separate offer link with an announced new tab and a focusable scroll region', () => {
  render(<ShopPrices history={productHistory} />);
  const link = screen.getByRole('link', { name: 'Ver oferta de Mercado Exemplo (abre em nova aba)' });
  expect(link).toHaveAttribute('href', productHistory.shopPrices[0].productUrl);
  expect(link).toHaveAttribute('target', '_blank');
  expect(link).toHaveAttribute('rel', 'noreferrer');
  expect(screen.getByRole('region', { name: 'Preços por loja' })).toHaveAttribute('tabindex', '0');
});

it('does not render shop comparisons for an overview or empty shop list', () => {
  const view = render(<ShopPrices history={{ ...productHistory, unit: 'percent' }} />);
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  view.rerender(<ShopPrices history={{ ...productHistory, shopPrices: [] }} />);
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
});
