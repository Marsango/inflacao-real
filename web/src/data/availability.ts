import type { History } from './model';

export function variationStatus(history: History) {
  if (history.counts.pricedDays === 0) return 'Sem dias comparáveis';
  if (history.counts.pricedDays === 1) return 'Histórico insuficiente';
  return 'Não calculável';
}

export function variationExplanation(history: History) {
  if (history.counts.products === 0) return 'Nenhum preço comparável neste período. Escolha outro intervalo ou produto.';
  if (history.counts.pricedDays === 1) return 'Há preços comparáveis em apenas um dia. A variação precisa de pelo menos dois dias no período. Amplie o período ou aguarde novas coletas.';
  if (history.counts.pricedDays > 1) return 'Não foi possível calcular a variação com os preços disponíveis. A comparação exige os mesmos contextos nas duas datas selecionadas, uma base positiva e um resultado representável.';
  if (history.unit === 'percent') return 'A visão geral usa produtos com preços comparáveis nas duas datas selecionadas. Não há painel comparável neste período. Busque um produto para ver os preços disponíveis.';
  return 'Não há preços comparáveis nas duas datas selecionadas. Consulte os preços disponíveis por loja ou escolha outro intervalo. Preços ausentes e condições ou embalagens incompatíveis ficam fora da comparação.';
}
