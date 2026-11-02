package insights

import "time"

// ExpandSparse aligns contexts only within the shared eligible period. An all-null
// selection retains the requested calendar, matching the public coverage contract.
func ExpandSparse(series []ProductSeries, start, end string) []ProductSeries {
	period := availablePeriod(series)
	if period != nil {
		start, end = period.Start, period.End
	}
	dates := calendarDates(start, end)
	for position := range series {
		series[position].Contexts = expandContexts(series[position].Contexts, dates)
	}
	return series
}

func calendarDates(start, end string) []string {
	first, _ := time.Parse(time.DateOnly, start)
	last, _ := time.Parse(time.DateOnly, end)
	dates := []string{}
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		dates = append(dates, day.Format(time.DateOnly))
	}
	return dates
}

func expandContexts(contexts []ContextSeries, dates []string) []ContextSeries {
	for position := range contexts {
		contexts[position].Points = expandPoints(contexts[position].Points, dates)
	}
	return contexts
}

func expandPoints(points []PricePoint, dates []string) []PricePoint {
	amounts := make(map[string]*string, len(points))
	for _, point := range points {
		amounts[point.Date] = point.Amount
	}
	expanded := make([]PricePoint, len(dates))
	for position, date := range dates {
		expanded[position] = PricePoint{Date: date, Amount: amounts[date]}
	}
	return expanded
}
