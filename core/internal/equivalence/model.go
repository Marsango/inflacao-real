// Package equivalence manages reversible catalog equivalence, never source identities.
package equivalence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"inflacao-real/core/internal/catalog"
)

var ErrStale = errors.New("stale catalog review")
var ErrInvalid = errors.New("invalid equivalence decision")

type Listing struct {
	Name    string  `json:"name"`
	Brand   *string `json:"brand"`
	URL     string  `json:"url"`
	Shop    string  `json:"shop"`
	Barcode *string `json:"barcode"`
}
type Product struct {
	FirstObserved  *string              `json:"firstObserved"`
	LastObserved   *string              `json:"lastObserved"`
	ID             int64                `json:"id"`
	Name           string               `json:"name"`
	Brand          *string              `json:"brand"`
	Barcode        *string              `json:"barcode"`
	Package        *catalog.PackageSize `json:"package"`
	Bases          []string             `json:"bases"`
	Listings       []Listing            `json:"listings"`
	Observations   int64                `json:"observations"`
	Loose          bool                 `json:"loose"`
	Representative int64                `json:"representative"`
}
type Candidate struct {
	First                int64    `json:"first"`
	Second               int64    `json:"second"`
	Kind                 string   `json:"kind"`
	Score                float64  `json:"score"`
	Differences          []string `json:"differences"`
	Reason               string   `json:"reason"`
	VerificationRequired bool     `json:"verificationRequired"`
}
type Snapshot struct {
	Revision       int64           `json:"revision"`
	GenericEnabled bool            `json:"genericEnabled"`
	Products       []Product       `json:"products"`
	Candidates     []Candidate     `json:"candidates"`
	Generic        []Candidate     `json:"generic"`
	Audit          json.RawMessage `json:"audit"`
}
type Decision struct {
	Revision       int64  `json:"revision"`
	Action         string `json:"action"`
	First          int64  `json:"first"`
	Second         int64  `json:"second"`
	Representative int64  `json:"representative"`
	Reason         string `json:"reason"`
	SourceVerified bool   `json:"sourceVerified"`
}
type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
