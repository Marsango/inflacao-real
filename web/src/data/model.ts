import type { Period } from '../shared/dates';

export type Product = { id: string; label: string };
export type Observation = { date: string; value: number | null };
export type PriceBasis = 'package' | 'kg' | 'l' | 'unit';
export type ShopHistory = {
  shopId: string; shopName: string; context: string; productUrl: string;
  points: Observation[]; included: boolean;
};
export type History = {
  shopPrices: ShopHistory[];
  unknownConditions: boolean;
  period: Period;
  availablePeriod: Period | null;
  timeZone: 'America/Sao_Paulo';
  priceBasis: PriceBasis | null;
  title: string;
  unit: 'BRL' | 'percent';
  points: Observation[];
  latestPriceMinor: number | null;
  changeBasisPoints: number | null;
  comparisonStart: string | null;
  comparisonEnd: string | null;
  collectedAt: string | null;
  stale: boolean;
  coverage: string;
  counts: { includedProducts: number; includedShops: number; includedStreams: number; products: number; shops: number; streams: number; days: number; pricedDays: number };
  methodology: string;
  methodologyVersion: string;
};
export type HistoryRequest = Period & { product: string };
export type LoadState = { status: 'loading' | 'error' | 'ready'; history?: History };
export type ProductPage = { items: Product[]; nextCursor: string | null };
export type SearchState = { status: 'idle' | 'loading' | 'error' | 'ready'; page?: ProductPage };
