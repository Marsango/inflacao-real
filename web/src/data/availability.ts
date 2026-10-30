import type { History } from './model';

export function variationStatus(history: History) {
  if (history.counts.pricedDays === 0) return 'Sem dias comparáveis';
  if (history.counts.pricedDays === 1) return 'Histórico insuficiente';
  return 'Não calculável';
}

export function variationExplanation(history: History) {
  if (history.counts.products === 0) return 'Nenhum preço comparável neste período. Escolha outro intervalo ou produto.';
  if (history.counts.pricedDays === 1) return 'Há preços comparáveis em apenas um dia. A variação precisa de pelo menos dois dias no período. Amplie o período ou aguarde novas coletas.';
  if (history.counts.pricedDays > 1) return 'Não foi possível calcular a variação com os preços disponíveis. A comparação precisa de uma base positiva e um resultado representável.';
  if (history.unit === 'percent') return 'A visão geral precisa de preços comparáveis para todos os produtos do painel no mesmo dia. Nenhum dia deste período tem cobertura completa. Busque um produto para ver seu histórico.';
  return 'Nenhum preço comparável neste período. As coletas podem ter preços ausentes, condições de compra ou embalagens incompatíveis. Escolha outro intervalo ou produto.';
}
