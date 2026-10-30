package webapi

import (
	"context"
	"errors"

	"inflacao-real/core/internal/insights"
)

var ErrNotFound = errors.New("product not found")

type Product struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Products struct {
	Items      []Product `json:"items"`
	NextCursor *string   `json:"nextCursor"`
}

type HistoryRequest struct {
	Product string
	Start   string
	End     string
}

type ProductRequest struct {
	Query string
	After int64
	Limit int
}

type Store interface {
	Products(context.Context, ProductRequest) (Products, error)
	History(context.Context, HistoryRequest) (insights.History, error)
}
