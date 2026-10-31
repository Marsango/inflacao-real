package insights

import (
	"fmt"
	"time"
)

const methodology = "Painel de contextos com preços comparáveis nas duas datas solicitadas, com base inicial positiva. " +
	"Cada contexto usa a última observação diária e o menor preço elegível em BRL, sem restrições explícitas de pagamento, clube, cupom ou compra múltipla. " +
	"Prioriza condições comprovadamente livres no intervalo; na ausência delas, aceita condições não informadas, sinalizadas na interface. " +
	"Exige embalagens equivalentes ou uma única base explícita (kg, L ou unidade). " +
	"A variação é a média das variações dos contextos por loja e depois das lojas por produto; a visão geral dá peso igual aos produtos elegíveis. " +
	"O gráfico de preços do produto usa médias dos mesmos contextos por loja, com pesos iguais por loja. " +
	"Preços disponíveis por loja são exibidos separadamente, inclusive contextos excluídos da comparação. " +
	"Contextos sem preço nas pontas são excluídos; lacunas intermediárias do painel fixo ficam vazias, sem interpolação. " +
	"Condições não informadas não garantem acesso ao preço ou equivalência das condições ao longo do tempo. " +
	"O painel depende do intervalo selecionado e o recorte monitorado não representa a inflação nacional."

func Build(series []ProductSeries, global bool, shopCount int, now time.Time) History {
	history := emptyHistory()
	if len(series) == 0 {
		return history
	}
	series = prepareSeries(series)
	history.Counts = Coverage{Products: len(series), Shops: shopCount, Days: len(series[0].Points)}
	collectMetadata(&history, series)
	history.Points = makePoints(series, global)
	if !global {
		setProduct(&history, series[0])
	}
	summarize(&history, series, global)
	finishContext(&history, now)
	return history
}

func finishContext(history *History, now time.Time) {
	history.Stale = history.CollectedAt != nil && now.Sub(*history.CollectedAt) > 48*time.Hour
	history.Coverage = fmt.Sprintf("%d de %d produto(s), %d de %d loja(s), %d de %d contexto(s) incluídos na comparação; %d de %d dias com cobertura do painel.",
		history.Counts.IncludedProducts, history.Counts.Products, history.Counts.IncludedShops, history.Counts.Shops,
		history.Counts.IncludedStreams, history.Counts.Streams, history.Counts.PricedDays, history.Counts.Days)
}

func collectMetadata(history *History, series []ProductSeries) {
	shops := map[string]bool{}
	for _, product := range series {
		collectProductMetadata(history, product)
		includeShops(shops, product.Contexts)
	}
	history.Counts.IncludedShops = len(shops)
}

func includeShops(shops map[string]bool, contexts []ContextSeries) {
	for _, context := range contexts {
		includeShop(shops, context)
	}
}

func collectProductMetadata(history *History, product ProductSeries) {
	history.Counts.Streams += product.Streams
	history.Counts.IncludedStreams += product.IncludedStreams
	if product.IncludedStreams > 0 {
		history.Counts.IncludedProducts++
	}
	history.UnknownConditions = history.UnknownConditions || product.UnknownConditions
	if product.CollectedAt == nil {
		return
	}
	if history.CollectedAt == nil || product.CollectedAt.After(*history.CollectedAt) {
		history.CollectedAt = product.CollectedAt
	}
}

func makePoints(series []ProductSeries, global bool) []Point {
	points := make([]Point, 0, len(series[0].Points))
	for position, point := range series[0].Points {
		points = append(points, makePoint(series, point, position, global))
	}
	return points
}

func summarize(history *History, series []ProductSeries, global bool) {
	for _, point := range history.Points {
		countPricedDay(history, point)
	}
	if history.Counts.IncludedProducts == 0 || history.Counts.Days < 2 {
		return
	}
	last := history.Counts.Days - 1
	history.ComparisonStart, history.ComparisonEnd = &history.Points[0].Date, &history.Points[last].Date
	history.ChangeBasisPoints = history.Points[last].Value
	if !global {
		history.ChangeBasisPoints = rounded(amount(series[0].Changes[last].Amount))
	}
}

func emptyHistory() History {
	return History{Title: "Variação dos produtos monitorados", Unit: "percent", Points: []Point{}, ShopPrices: []ShopHistory{},
		Methodology: methodology, MethodologyVersion: "matched-shops-v2", TimeZone: "America/Sao_Paulo", Coverage: "Nenhuma observação no período."}
}

func includeShop(shops map[string]bool, context ContextSeries) {
	if comparable(context) {
		shops[context.ShopID] = true
	}
}

func makePoint(series []ProductSeries, point PricePoint, position int, global bool) Point {
	if global {
		return Point{point.Date, rounded(overviewValue(series, position))}
	}
	return Point{point.Date, rounded(amount(point.Amount))}
}

func countPricedDay(history *History, point Point) {
	if point.Value != nil {
		history.Counts.PricedDays++
	}
}
