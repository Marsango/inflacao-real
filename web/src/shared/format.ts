import type { PriceBasis } from '../data/model';

const numberFormatter = new Intl.NumberFormat('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const signedFormatter = new Intl.NumberFormat('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2, signDisplay: 'exceptZero' });

export function money(minorUnits: number): string {
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(minorUnits / 100);
}

export function percentage(basisPoints: number): string {
  return `${signedFormatter.format(basisPoints / 100)}%`;
}

export function chartValue(value: number, unit: 'BRL' | 'percent'): string {
  if (unit === 'BRL') return money(value);
  return `${numberFormatter.format(value / 100)}%`;
}

export function priceLabel(basis: PriceBasis | null): string {
  if (basis === 'kg') return 'Preço por kg (R$)';
  if (basis === 'l') return 'Preço por L (R$)';
  if (basis === 'unit') return 'Preço por unidade (R$)';
  return 'Preço por embalagem (R$)';
}
