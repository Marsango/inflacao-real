package document

import (
	"encoding/json"

	"inflacao-real/core/internal/catalog"
)

type Envelope struct {
	ScraperID    string `json:"scraper_id"`
	SubmissionID string `json:"submission_id"`
	ShopID       string `json:"shop_id"`
	RunID        string `json:"run_id"`
	Collector    struct{ Name, Version string }
	Observations []json.RawMessage
}

type Observation struct {
	ID               string `json:"observation_id"`
	ObservedAt       string `json:"observed_at"`
	CollectionMethod string `json:"collection_method"`
	SourceURL        string `json:"source_url"`
	ProductURL       string `json:"product_url"`
	Product          catalog.Product
	Context          struct {
		SellerID   *string `json:"seller_id"`
		StoreID    *string `json:"store_id"`
		PostalCode *string `json:"postal_code"`
		City       *string
		State      *string
		Channel    string
	}
	Availability string
	Offers       []Offer
}

type Offer struct {
	Amount          *string
	Currency        *string
	Basis           *string
	ReferenceAmount *string `json:"reference_amount"`
	Conditions      struct {
		PaymentMethod      *string `json:"payment_method"`
		Installments       *int
		MembershipRequired *bool   `json:"membership_required"`
		MembershipProgram  *string `json:"membership_program"`
		CouponRequired     *bool   `json:"coupon_required"`
		MinimumQuantity    *string `json:"minimum_quantity"`
		Description        *string
	}
}

// Submission carries queue metadata and the original validated JSON.
type Submission struct {
	ScraperID     string
	SubmissionID  string
	ShopID        string
	SchemaVersion string
	CreatedAt     string
	Payload       []byte
}
