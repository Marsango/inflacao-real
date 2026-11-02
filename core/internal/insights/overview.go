package insights

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

type OverviewMetadata struct {
	Counts            Coverage   `json:"counts"`
	UnknownConditions bool       `json:"unknownConditions"`
	CollectedAt       *time.Time `json:"collectedAt"`
	AvailablePeriod   *Period    `json:"availablePeriod"`
	Locations         []string   `json:"locations"`
}

type OverviewCalculator struct {
	metadata OverviewMetadata
	totals   map[string]*big.Rat
	streams  map[string]int
}

func NewOverview(metadata OverviewMetadata) *OverviewCalculator {
	return &OverviewCalculator{metadata: metadata, totals: map[string]*big.Rat{}, streams: map[string]int{}}
}

// Add keeps exact rational arithmetic. SQL combines numerators only when their
// denominators (baseline × contexts per shop × shops per product) are identical.
func (calculator *OverviewCalculator) Add(date, numerator, denominator string, streams int) error {
	value, valid := new(big.Rat).SetString(numerator)
	divisor, positive := new(big.Rat).SetString(denominator)
	if !valid || !positive || divisor.Sign() <= 0 {
		return fmt.Errorf("invalid overview weight")
	}
	value.Quo(value, divisor)
	total := calculator.totals[date]
	if total == nil {
		total = new(big.Rat)
	}
	calculator.totals[date] = total.Add(total, value)
	calculator.streams[date] += streams
	return nil
}

func (calculator *OverviewCalculator) Build(start, end string, now time.Time) History {
	history := calculator.history()
	if history.Counts.Products == 0 {
		return history
	}
	if history.AvailablePeriod != nil {
		start, end = history.AvailablePeriod.Start, history.AvailablePeriod.End
	}
	calculator.fillPoints(&history, calendarDates(start, end))
	setOverviewComparison(&history)
	finishContext(&history, now)
	history.Coverage += calculator.locations()
	return history
}

func (calculator *OverviewCalculator) fillPoints(history *History, dates []string) {
	for _, date := range dates {
		calculator.appendPoint(history, date)
	}
	history.Counts.Days = len(dates)
}

func (calculator *OverviewCalculator) appendPoint(history *History, date string) {
	point := calculator.point(date)
	history.Points = append(history.Points, point)
	countPricedDay(history, point)
}

func (calculator *OverviewCalculator) locations() string {
	sort.Strings(calculator.metadata.Locations)
	return " Localidades observadas: " + strings.Join(calculator.metadata.Locations, "; ") + "."
}

func (calculator *OverviewCalculator) history() History {
	history := emptyHistory()
	history.Counts = calculator.metadata.Counts
	history.UnknownConditions = calculator.metadata.UnknownConditions
	history.CollectedAt = calculator.metadata.CollectedAt
	history.AvailablePeriod = calculator.metadata.AvailablePeriod
	return history
}

func (calculator *OverviewCalculator) point(date string) Point {
	counts := calculator.metadata.Counts
	if counts.IncludedStreams == 0 || calculator.streams[date] != counts.IncludedStreams {
		return Point{Date: date}
	}
	value := new(big.Rat).Set(calculator.totals[date])
	value.Quo(value, big.NewRat(int64(counts.IncludedProducts), 1))
	value.Sub(value, big.NewRat(10000, 1))
	return Point{Date: date, Value: rounded(value)}
}

func setOverviewComparison(history *History) {
	if history.Counts.IncludedProducts == 0 || len(history.Points) < 2 {
		return
	}
	last := len(history.Points) - 1
	history.ComparisonStart, history.ComparisonEnd = &history.Points[0].Date, &history.Points[last].Date
	history.ChangeBasisPoints = history.Points[last].Value
}
