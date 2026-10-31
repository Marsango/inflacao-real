package insights

import "math/big"

func comparable(context ContextSeries) bool {
	if len(context.Points) == 0 {
		return false
	}
	baseline := amount(context.Points[0].Amount)
	last := amount(context.Points[len(context.Points)-1].Amount)
	return baseline != nil && baseline.Sign() > 0 && last != nil
}

func prepare(product ProductSeries) ProductSeries {
	shops := map[string][]ContextSeries{}
	for _, context := range product.Contexts {
		includeContext(&product, shops, context)
	}
	product.Points, product.Changes = matchedPoints(product.Contexts, shops)
	return product
}

func matchedPoints(contexts []ContextSeries, shops map[string][]ContextSeries) ([]PricePoint, []PricePoint) {
	prices, changes := []PricePoint{}, []PricePoint{}
	if len(contexts) == 0 {
		return prices, changes
	}
	for position, point := range contexts[0].Points {
		prices = append(prices, PricePoint{point.Date, rationalText(shopAverage(shops, position, false))})
		changes = append(changes, PricePoint{point.Date, rationalText(shopAverage(shops, position, true))})
	}
	return prices, changes
}

func rationalText(value *big.Rat) *string {
	if value == nil {
		return nil
	}
	text := value.RatString()
	return &text
}

func shopAverage(shops map[string][]ContextSeries, position int, changes bool) *big.Rat {
	if len(shops) == 0 {
		return nil
	}
	total := new(big.Rat)
	for _, contexts := range shops {
		total = addAmount(total, contextAverage(contexts, position, changes))
	}
	return divide(total, len(shops))
}

func contextAverage(contexts []ContextSeries, position int, changes bool) *big.Rat {
	total := new(big.Rat)
	for _, context := range contexts {
		total = addAmount(total, contextValue(context, position, changes))
	}
	return divide(total, len(contexts))
}

func contextValue(context ContextSeries, position int, changes bool) *big.Rat {
	value := amount(context.Points[position].Amount)
	if changes {
		return relative(value, amount(context.Points[0].Amount))
	}
	return value
}

func overviewValue(series []ProductSeries, position int) *big.Rat {
	total, count := new(big.Rat), 0
	for _, product := range series {
		total, count = includeProductValue(total, count, product, position)
	}
	return divide(total, count)
}

func includeProductValue(total *big.Rat, count int, product ProductSeries, position int) (*big.Rat, int) {
	if product.IncludedStreams == 0 {
		return total, count
	}
	return addAmount(total, amount(product.Changes[position].Amount)), count + 1
}

func includeContext(product *ProductSeries, shops map[string][]ContextSeries, context ContextSeries) {
	if !comparable(context) {
		return
	}
	shops[context.ShopID] = append(shops[context.ShopID], context)
	product.IncludedStreams++
}

func prepareSeries(series []ProductSeries) []ProductSeries {
	result := make([]ProductSeries, 0, len(series))
	for _, product := range series {
		result = append(result, prepare(product))
	}
	return result
}

func addAmount(total, value *big.Rat) *big.Rat {
	if total == nil || value == nil {
		return nil
	}
	return total.Add(total, value)
}

func divide(total *big.Rat, count int) *big.Rat {
	if total == nil || count == 0 {
		return nil
	}
	return total.Quo(total, big.NewRat(int64(count), 1))
}
