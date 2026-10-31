package insights

func setProduct(history *History, product ProductSeries) {
	history.Title, history.Unit, history.PriceBasis = product.Name, "BRL", product.PriceBasis
	for _, context := range product.Contexts {
		history.ShopPrices = append(history.ShopPrices, shopHistory(context))
	}
	history.LatestPriceMinor = latestAvailable(product.Contexts)
}

func shopHistory(context ContextSeries) ShopHistory {
	result := ShopHistory{ShopID: context.ShopID, ShopName: context.ShopName, Context: context.Label,
		ProductURL: context.ProductURL, Included: comparable(context), Points: []Point{}}
	for _, point := range context.Points {
		result.Points = append(result.Points, Point{point.Date, rounded(amount(point.Amount))})
	}
	return result
}

// Use the cheapest available context on the most recent day with a price.
func latestAvailable(contexts []ContextSeries) *int64 {
	if len(contexts) == 0 {
		return nil
	}
	position := len(contexts[0].Points) - 1
	for position >= 0 && lowestAvailable(contexts, position) == nil {
		position--
	}
	if position < 0 {
		return nil
	}
	return lowestAvailable(contexts, position)
}

func lowestAvailable(contexts []ContextSeries, position int) *int64 {
	var lowest *int64
	for _, context := range contexts {
		lowest = lowerPrice(lowest, rounded(amount(context.Points[position].Amount)))
	}
	return lowest
}

func lowerPrice(lowest, price *int64) *int64 {
	if price != nil && (lowest == nil || *price < *lowest) {
		return price
	}
	return lowest
}
