package insights

import "time"

type PricePoint struct {
	Date   string  `json:"date"`
	Amount *string `json:"amount"`
}

type ContextSeries struct {
	ID         string       `json:"id"`
	ShopID     string       `json:"shopId"`
	ShopName   string       `json:"shopName"`
	Label      string       `json:"label"`
	Location   string       `json:"location"`
	ProductURL string       `json:"productUrl"`
	Points     []PricePoint `json:"points"`
}

type ShopHistory struct {
	ShopID     string  `json:"shopId"`
	ShopName   string  `json:"shopName"`
	Context    string  `json:"context"`
	ProductURL string  `json:"productUrl"`
	Points     []Point `json:"points"`
	Included   bool    `json:"included"`
}

type ProductSeries struct {
	Contexts        []ContextSeries
	Changes         []PricePoint
	IncludedStreams int

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
	IncludedProducts int `json:"includedProducts"`
	IncludedShops    int `json:"includedShops"`
	IncludedStreams  int `json:"includedStreams"`

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
	ShopPrices      []ShopHistory `json:"shopPrices"`
	AvailablePeriod *Period       `json:"availablePeriod"`

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
