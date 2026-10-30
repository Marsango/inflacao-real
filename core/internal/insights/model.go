package insights

import "time"

type PricePoint struct {
	Date   string  `json:"date"`
	Amount *string `json:"amount"`
}

type ProductSeries struct {
	UnknownConditions bool
	PriceBasis        *string
	ID                string
	Name              string
	Points            []PricePoint
	CollectedAt       *time.Time
	Shops             int
	Streams           int
}

type Point struct {
	Date  string `json:"date"`
	Value *int64 `json:"value"`
}

type Coverage struct {
	Products   int `json:"products"`
	Shops      int `json:"shops"`
	Streams    int `json:"streams"`
	Days       int `json:"days"`
	PricedDays int `json:"pricedDays"`
}

type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type History struct {
	UnknownConditions  bool       `json:"unknownConditions"`
	Period             Period     `json:"period"`
	TimeZone           string     `json:"timeZone"`
	PriceBasis         *string    `json:"priceBasis"`
	Title              string     `json:"title"`
	Unit               string     `json:"unit"`
	Points             []Point    `json:"points"`
	LatestPriceMinor   *int64     `json:"latestPriceMinor"`
	ChangeBasisPoints  *int64     `json:"changeBasisPoints"`
	ComparisonStart    *string    `json:"comparisonStart"`
	ComparisonEnd      *string    `json:"comparisonEnd"`
	CollectedAt        *time.Time `json:"collectedAt"`
	Stale              bool       `json:"stale"`
	Coverage           string     `json:"coverage"`
	Counts             Coverage   `json:"counts"`
	Methodology        string     `json:"methodology"`
	MethodologyVersion string     `json:"methodologyVersion"`
}
