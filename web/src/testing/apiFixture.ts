import { vi } from 'vitest';
import type { History } from '../data/model';
import example from '../../../docs/contracts/web/v1/history.example.json';

export const productHistory = example as History;
export const catalog = { items: [{ id: '1', label: 'Arroz branco · 5 kg' }, { id: '2', label: 'Café · 500 g' }], nextCursor: null };
export const groups = [
  { id: 'cesta-basica', label: 'Cesta básica', description: 'Seleção de alimentos básicos. Não representa uma cesta básica oficial.' },
  { id: 'hortifruti', label: 'Hortifruti', description: 'Frutas, verduras e legumes frescos monitorados.' },
  { id: 'doces', label: 'Doces', description: 'Chocolates, balas e outros doces monitorados.' },
  { id: 'carnes', label: 'Carnes', description: 'Cortes de carne monitorados.' },
];
export const globalHistory: History = { ...productHistory, title: 'Variação dos produtos monitorados', unit: 'percent', priceBasis: null, shopPrices: [],
  points: [{ date: '2026-10-27', value: 0 }, { date: '2026-10-28', value: null }, { date: '2026-10-29', value: 803 }], latestPriceMinor: null };

export function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
}

export function mockCore(history?: History) {
  return vi.fn(async (path: string) => {
    if (path === '/api/v1/groups') return jsonResponse(groups);
    if (path.startsWith('/api/v1/products')) return jsonResponse(catalog);
    const query = new URL(path, 'http://localhost').searchParams;
    const result = history ?? (query.get('product') === 'global' ? globalHistory : productHistory);
    const group = groups.find((option) => option.id === query.get('group'));
    return jsonResponse(group && query.get('product') === 'global' ? { ...result, title: `Variação de preços · ${group.label}` } : result);
  });
}
