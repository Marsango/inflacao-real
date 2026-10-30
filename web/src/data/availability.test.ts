import { expect, it } from 'vitest';
import { variationExplanation, variationStatus } from './availability';
import { globalHistory, productHistory } from '../testing/apiFixture';

it('explains why a single comparable day cannot show a change', () => {
  const history = { ...productHistory, counts: { ...productHistory.counts, pricedDays: 1 } };
  expect(variationStatus(history)).toBe('Histórico insuficiente');
  expect(variationExplanation(history)).toContain('pelo menos dois dias');
});

it('distinguishes incomplete overview coverage from an empty dataset', () => {
  const history = { ...globalHistory, counts: { ...globalHistory.counts, pricedDays: 0 } };
  expect(variationStatus(history)).toBe('Sem dias comparáveis');
  expect(variationExplanation(history)).toContain('Nenhum dia deste período tem cobertura completa');
  expect(variationExplanation({ ...history, counts: { ...history.counts, products: 0 } })).toContain('Nenhum preço comparável');
});
