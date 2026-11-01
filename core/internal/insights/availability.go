package insights

func availablePeriod(series []ProductSeries) *Period {
	var period *Period
	for _, product := range series {
		productPeriod := productAvailability(product.Contexts)
		period = extendPeriod(period, productPeriod)
	}
	return period
}

func productAvailability(contexts []ContextSeries) *Period {
	var period *Period
	for _, context := range contexts {
		period = extendPeriod(period, contextAvailability(context))
	}
	return period
}

func contextAvailability(context ContextSeries) *Period {
	var period *Period
	for _, point := range context.Points {
		period = extendPeriod(period, pointAvailability(point))
	}
	return period
}

func pointAvailability(point PricePoint) *Period {
	if point.Amount == nil {
		return nil
	}
	return &Period{Start: point.Date, End: point.Date}
}

func extendPeriod(period, available *Period) *Period {
	if available == nil {
		return period
	}
	if period == nil {
		return &Period{Start: available.Start, End: available.End}
	}
	if available.Start < period.Start {
		period.Start = available.Start
	}
	if available.End > period.End {
		period.End = available.End
	}
	return period
}

func trimSeries(series []ProductSeries, period *Period) []ProductSeries {
	if period == nil {
		return series
	}
	result := make([]ProductSeries, 0, len(series))
	for _, product := range series {
		product.Contexts = trimContexts(product.Contexts, *period)
		result = append(result, product)
	}
	return result
}

func trimContexts(contexts []ContextSeries, period Period) []ContextSeries {
	result := make([]ContextSeries, 0, len(contexts))
	for _, context := range contexts {
		context.Points = trimPoints(context.Points, period)
		result = append(result, context)
	}
	return result
}

func trimPoints(points []PricePoint, period Period) []PricePoint {
	result := []PricePoint{}
	for _, point := range points {
		result = appendAvailablePoint(result, point, period)
	}
	return result
}

func appendAvailablePoint(points []PricePoint, point PricePoint, period Period) []PricePoint {
	if point.Date < period.Start || point.Date > period.End {
		return points
	}
	return append(points, point)
}
