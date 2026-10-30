package insights

import (
	"fmt"
	"time"
)

const methodology = "Painel fixo de anúncios e contextos observados no intervalo (loja, vendedor, localização e canal). " +
	"Cada dia usa a última observação de cada contexto e o menor preço comparável em BRL com pagamento livre, " +
	"sem restrições explícitas de pagamento, clube, cupom ou compra múltipla. Embalagens não equivalentes são excluídas. " +
	"Por contexto, priorizam-se condições comprovadamente livres no intervalo; quando não existem, usam-se preços anunciados com condições não informadas, sinalizados na interface. " +
	"Condições não informadas não garantem acesso ao preço nem equivalência das condições ao longo do tempo. " +
	"Embalagens conhecidas usam preço por pacote; sem embalagem conhecida, exige-se uma única base explícita (kg, L ou unidade). " +
	"O preço do produto é a média dos contextos, somente quando todos têm preço válido no dia. " +
	"Dias sem cobertura completa ficam vazios, sem interpolação. A visão geral tem pesos iguais por produto, " +
	"com base no primeiro dia de cobertura completa e preços positivos. O painel depende do intervalo selecionado. " +
	"O recorte monitorado não representa a inflação nacional."

func Build(series []ProductSeries, global bool, shopCount int, now time.Time) History {
	history := History{Title: "Variação dos produtos monitorados", Unit: "percent", Points: []Point{},
		Methodology: methodology, MethodologyVersion: "fixed-panel-v1", TimeZone: "America/Sao_Paulo", Coverage: "Nenhuma observação no período."}
	if len(series) == 0 {
		return history
	}
	history.Counts = Coverage{Products: len(series), Shops: shopCount, Days: len(series[0].Points)}
	collectMetadata(&history, series)
	history.Points = makePoints(series, global)
	if !global {
		history.Title, history.Unit, history.PriceBasis = series[0].Name, "BRL", series[0].PriceBasis
	}
	summarize(&history, series, global)
	finishContext(&history, shopCount, now)
	return history
}

func finishContext(history *History, shopCount int, now time.Time) {
	history.Stale = history.CollectedAt != nil && now.Sub(*history.CollectedAt) > 48*time.Hour
	history.Coverage = fmt.Sprintf("%d produto(s), %d loja(s), %d contexto(s); %d de %d dias com cobertura completa.",
		history.Counts.Products, shopCount, history.Counts.Streams, history.Counts.PricedDays, history.Counts.Days)
}

func collectMetadata(history *History, series []ProductSeries) {
	for _, product := range series {
		collectProductMetadata(history, product)
	}
}

func collectProductMetadata(history *History, product ProductSeries) {
	history.Counts.Streams += product.Streams
	history.UnknownConditions = history.UnknownConditions || product.UnknownConditions
	if product.CollectedAt == nil {
		return
	}
	if history.CollectedAt == nil || product.CollectedAt.After(*history.CollectedAt) {
		history.CollectedAt = product.CollectedAt
	}
}

func baselinePosition(series []ProductSeries) int {
	position := 0
	for position < len(series[0].Points) && averageRelative(series, position, position) == nil {
		position++
	}
	if position == len(series[0].Points) {
		return -1
	}
	return position
}

func makePoints(series []ProductSeries, global bool) []Point {
	points := make([]Point, 0, len(series[0].Points))
	baseline := baselinePosition(series)
	for position, point := range series[0].Points {
		points = append(points, makePoint(series, point, position, baseline, global))
	}
	return points
}

func makePoint(series []ProductSeries, point PricePoint, position, baseline int, global bool) Point {
	if !global {
		return Point{point.Date, rounded(amount(point.Amount))}
	}
	if baseline < 0 || position < baseline {
		return Point{point.Date, nil}
	}
	return Point{point.Date, rounded(averageRelative(series, position, baseline))}
}

func summarize(history *History, series []ProductSeries, global bool) {
	first, last := -1, -1
	for position := range history.Points {
		first, last = recordPoint(history, position, first, last)
	}
	if last < 0 {
		return
	}
	history.ComparisonStart, history.ComparisonEnd = &history.Points[first].Date, &history.Points[last].Date
	if !global {
		history.LatestPriceMinor = history.Points[last].Value
	}
	if first == last {
		return
	}
	history.ChangeBasisPoints = change(series, history.Points, first, last, global)
}

func change(series []ProductSeries, points []Point, first, last int, global bool) *int64 {
	if global {
		return points[last].Value
	}
	return rounded(relative(amount(series[0].Points[last].Amount), amount(series[0].Points[first].Amount)))
}

func recordPoint(history *History, position, first, last int) (int, int) {
	point := history.Points[position]
	if point.Value == nil {
		return first, last
	}
	history.Counts.PricedDays++
	if first < 0 {
		return position, position
	}
	return first, position
}
