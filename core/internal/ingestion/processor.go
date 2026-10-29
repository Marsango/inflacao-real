package ingestion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type envelope struct {
	ScraperID    string `json:"scraper_id"`
	SubmissionID string `json:"submission_id"`
	ShopID       string `json:"shop_id"`
	RunID        string `json:"run_id"`
	Collector    struct{ Name, Version string }
	Observations []json.RawMessage
}

type observation struct {
	ID               string `json:"observation_id"`
	ObservedAt       string `json:"observed_at"`
	CollectionMethod string `json:"collection_method"`
	SourceURL        string `json:"source_url"`
	ProductURL       string `json:"product_url"`
	Product          sourceProduct
	Context          struct {
		SellerID   *string `json:"seller_id"`
		StoreID    *string `json:"store_id"`
		PostalCode *string `json:"postal_code"`
		City       *string
		State      *string
		Channel    string
	}
	Availability string
	Offers       []offer
}

type packageSize struct {
	Quantity string
	Unit     string
	Count    int
}

type sourceProduct struct {
	SourceID string `json:"source_product_id"`
	Name     string
	Brand    *string
	GTIN     *string
	Package  *packageSize
}

type offer struct {
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

// permanentError denotes input that cannot succeed by retrying unchanged data.
type permanentError struct{ error }

func permanent(format string, args ...any) error {
	return permanentError{fmt.Errorf(format, args...)}
}

func decodeEnvelope(schema *jsonschema.Schema, raw []byte) (envelope, error) {
	if _, err := validatePayload(schema, raw); err != nil {
		return envelope{}, permanent("invalid stored payload: %w", err)
	}
	// Normalizes schema-valid integer spellings such as 1.0 and 1e0 before
	// decoding Go integers. Money and quantities remain exact strings.
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return envelope{}, permanent("decode payload: %w", err)
	}
	var canonical bytes.Buffer
	writeCanonical(&canonical, doc)
	var e envelope
	if err := json.Unmarshal(canonical.Bytes(), &e); err != nil {
		return envelope{}, permanent("decode envelope: %w", err)
	}
	return e, nil
}

func decodeObservation(raw []byte) (observation, error) {
	var o observation
	if err := json.Unmarshal(raw, &o); err != nil {
		return o, permanent("decode observation: %w", err)
	}
	if o.Product.GTIN != nil && !validGTIN(*o.Product.GTIN) {
		return o, permanent("observation %s: invalid GTIN check digit", o.ID)
	}
	for _, offer := range o.Offers {
		// The schema guarantees nonnegative decimal strings with two places.
		if offer.ReferenceAmount != nil && moneyMinor(*offer.ReferenceAmount) < moneyMinor(*offer.Amount) {
			return o, permanent("observation %s: reference amount is less than price", o.ID)
		}
	}
	return o, nil
}

func validGTIN(value string) bool {
	switch len(value) {
	case 8, 12, 13, 14:
	default:
		return false
	}
	sum, weight := 0, 1
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
		sum += int(value[i]-'0') * weight
		weight = 4 - weight
	}
	return sum%10 == 0
}

func moneyMinor(value string) int64 {
	var result int64
	for _, c := range value {
		if c != '.' {
			result = result*10 + int64(c-'0')
		}
	}
	return result
}

// writeCanonical implements JCS for schema-validated v1 data: object keys are
// fixed ASCII names, and JSON numbers are bounded integers (prices are strings).
// It is intentionally not a general-purpose JCS implementation.
func writeCanonical(out *bytes.Buffer, value any) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			writeCanonical(out, key)
			out.WriteByte(':')
			writeCanonical(out, v[key])
		}
		out.WriteByte('}')
	case []any:
		out.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				out.WriteByte(',')
			}
			writeCanonical(out, item)
		}
		out.WriteByte(']')
	case string:
		out.WriteByte('"')
		for _, r := range v {
			switch r {
			case '"', '\\':
				out.WriteByte('\\')
				out.WriteRune(r)
			case '\b':
				out.WriteString(`\b`)
			case '\t':
				out.WriteString(`\t`)
			case '\n':
				out.WriteString(`\n`)
			case '\f':
				out.WriteString(`\f`)
			case '\r':
				out.WriteString(`\r`)
			default:
				if r < 0x20 {
					fmt.Fprintf(out, `\u%04x`, r)
				} else {
					out.WriteRune(r)
				}
			}
		}
		out.WriteByte('"')
	case float64:
		out.WriteString(strconv.FormatFloat(v, 'f', 0, 64))
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case nil:
		out.WriteString("null")
	default:
		panic("unexpected value in validated v1 JSON")
	}
}

func observationFingerprint(shopID string, canonicalObservation []byte) []byte {
	var out bytes.Buffer
	out.WriteString(`{"observation":`)
	out.Write(canonicalObservation)
	out.WriteString(`,"shop_id":`)
	writeCanonical(&out, shopID)
	out.WriteByte('}')
	sum := sha256.Sum256(out.Bytes())
	return sum[:]
}

func persistEnvelope(ctx context.Context, tx *sql.Tx, payloadID int64, e envelope) error {
	var registered bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM shops WHERE id=$1)`, e.ShopID).Scan(&registered); err != nil {
		return err
	}
	if !registered {
		// A missing registration can be fixed without changing the submission.
		return fmt.Errorf("shop %s is not registered", e.ShopID)
	}
	// Initially serialize catalog decisions within each payload transaction.
	// This prevents competing workers from both creating the same new product,
	// including when a candidate has no barcode. Acquire before identity locks.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(694120031)`); err != nil {
		return err
	}
	// A stable order prevents two rebatches taking identity/listing locks in
	// opposite orders. Deadlocks from other writers are retried normally.
	sort.Slice(e.Observations, func(i, j int) bool { return bytes.Compare(e.Observations[i], e.Observations[j]) < 0 })
	for _, raw := range e.Observations {
		o, err := decodeObservation(raw)
		if err != nil {
			return err
		}
		fingerprint := observationFingerprint(e.ShopID, raw)
		result, err := tx.ExecContext(ctx, `INSERT INTO observation_identities
			(scraper_id, observation_id, fingerprint, payload_id, run_id, collector_name, collector_version)
			VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			e.ScraperID, o.ID, fingerprint, payloadID, e.RunID, e.Collector.Name, e.Collector.Version)
		if err != nil {
			return err
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if inserted == 0 {
			var existing []byte
			if err := tx.QueryRowContext(ctx, `SELECT fingerprint FROM observation_identities
				WHERE scraper_id=$1 AND observation_id=$2`, e.ScraperID, o.ID).Scan(&existing); err != nil {
				return err
			}
			if !bytes.Equal(existing, fingerprint) {
				return permanent("observation %s: conflicting content for existing identity", o.ID)
			}
			continue
		}
		productID, matchMethod, err := matchProduct(ctx, tx, e.ShopID, o.Product)
		if err != nil {
			return err
		}
		var listingID int64
		// A late upload must not overwrite a listing's newer presentation.
		err = tx.QueryRowContext(ctx, `INSERT INTO shop_products
			(shop_id,source_product_id,name,brand,gtin,product_url,metadata_observed_at,product_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7::timestamptz,$8)
			ON CONFLICT (shop_id,source_product_id) DO UPDATE SET
			name=EXCLUDED.name,brand=EXCLUDED.brand,gtin=EXCLUDED.gtin,
			product_url=EXCLUDED.product_url,metadata_observed_at=EXCLUDED.metadata_observed_at,
			product_id=EXCLUDED.product_id
			WHERE shop_products.metadata_observed_at IS NULL
				OR shop_products.metadata_observed_at<=EXCLUDED.metadata_observed_at
			RETURNING id`, e.ShopID, o.Product.SourceID, o.Product.Name, o.Product.Brand, o.Product.GTIN,
			o.ProductURL, o.ObservedAt, productID).Scan(&listingID)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `SELECT id FROM shop_products WHERE shop_id=$1 AND source_product_id=$2`,
				e.ShopID, o.Product.SourceID).Scan(&listingID)
		}
		if err != nil {
			return err
		}
		if err := persistOffers(ctx, tx, payloadID, listingID, productID, matchMethod, e.ScraperID, o); err != nil {
			return err
		}
	}
	return nil
}

func persistOffers(ctx context.Context, tx *sql.Tx, payloadID, listingID, productID int64, matchMethod, scraperID string, o observation) error {
	var quantity, unit, count any
	if p := o.Product.Package; p != nil {
		quantity, unit, count = p.Quantity, p.Unit, p.Count
	}
	offers := o.Offers
	if len(offers) == 0 {
		offers = []offer{{}}
	}
	for i, offer := range offers {
		c := offer.Conditions
		_, err := tx.ExecContext(ctx, `INSERT INTO price_observations (
			payload_id,shop_product_id,scraper_id,observation_id,observed_at,source_url,
			collection_method,package_quantity,package_unit,package_count,seller_id,store_id,
			postal_code,city,state,channel,availability,offer_count,offer_index,
			price_amount,price_currency,price_basis,reference_amount,payment_method,installments,
			membership_required,membership_program,coupon_required,minimum_quantity,condition_description,
			product_id,product_match_method
		) VALUES ($1,$2,$3,$4,$5::timestamptz,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,
			$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32)`,
			payloadID, listingID, scraperID, o.ID, o.ObservedAt, o.SourceURL,
			o.CollectionMethod, quantity, unit, count, o.Context.SellerID, o.Context.StoreID,
			o.Context.PostalCode, o.Context.City, o.Context.State, o.Context.Channel,
			o.Availability, len(o.Offers), i, offer.Amount, offer.Currency, offer.Basis,
			offer.ReferenceAmount, c.PaymentMethod, c.Installments, c.MembershipRequired,
			c.MembershipProgram, c.CouponRequired, c.MinimumQuantity, c.Description, productID, matchMethod)
		if err != nil {
			return err
		}
	}
	return nil
}

func isPermanent(err error) bool {
	var inputError permanentError
	return errors.As(err, &inputError)
}
