package ingestion

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"inflacao-real/core/internal/catalog"
	"inflacao-real/core/internal/equivalence"
	"inflacao-real/core/internal/ingestion/document"
	"inflacao-real/core/internal/ingestion/history"
	"inflacao-real/core/internal/monitoring"
)

type payloadWriter struct {
	context     context.Context
	transaction *sql.Tx
	identity    int64
	envelope    document.Envelope
	targets     []monitoring.Target
}

type listingAssignment struct {
	listingID, productID int64
	matchMethod          string
}

func persistEnvelope(requestContext context.Context, transaction *sql.Tx, identity int64, envelope document.Envelope) error {
	store := monitoring.PostgresStore{DB: transaction}
	targets, err := store.List(requestContext)
	if err != nil {
		return err
	}
	writer := payloadWriter{requestContext, transaction, identity, envelope, targets}
	return writer.persist()
}

func (writer payloadWriter) persist() error {
	if err := writer.requireShop(); err != nil {
		return err
	}
	if _, err := writer.transaction.ExecContext(writer.context, catalogLockSQL); err != nil {
		return err
	}
	// Source-time ordering preserves changes within a rebatch; raw bytes break ties.
	observations := writer.envelope.Observations
	sort.Slice(observations, func(left, right int) bool { return history.Earlier(observations[left], observations[right]) })
	var err error
	for position := 0; position < len(observations) && err == nil; position++ {
		err = writer.persistObservation(observations[position])
	}
	if err != nil {
		return err
	}
	return writer.reconcile()
}

func (writer payloadWriter) reconcile() error {
	if _, err := writer.transaction.ExecContext(writer.context, `UPDATE catalog_decisions SET revision=revision+1`); err != nil {
		return err
	}
	return equivalence.Reconcile(writer.context, writer.transaction)
}

func (writer payloadWriter) requireShop() error {
	var registered bool
	row := writer.transaction.QueryRowContext(writer.context, registeredShopSQL, writer.envelope.ShopID)
	if err := row.Scan(&registered); err != nil {
		return err
	}
	if !registered {
		return fmt.Errorf("shop %s is not registered", writer.envelope.ShopID)
	}
	return nil
}

func (writer payloadWriter) persistObservation(raw []byte) error {
	observation, err := document.DecodeObservation(raw)
	if err != nil {
		return err
	}
	fingerprint := document.Fingerprint(writer.envelope.ShopID, raw)
	inserted, err := writer.reserveIdentity(observation.ID, fingerprint)
	if err != nil {
		return err
	}
	if !inserted {
		return writer.requireSameFingerprint(observation.ID, fingerprint)
	}
	return writer.persistNewObservation(observation)
}

func (writer payloadWriter) reserveIdentity(observationID string, fingerprint []byte) (bool, error) {
	envelope := writer.envelope
	result, err := writer.transaction.ExecContext(writer.context, reserveObservationSQL,
		envelope.ScraperID, observationID, fingerprint, writer.identity, envelope.RunID,
		envelope.Collector.Name, envelope.Collector.Version)
	if err != nil {
		return false, err
	}
	inserted, err := result.RowsAffected()
	return inserted != 0, err
}

func (writer payloadWriter) requireSameFingerprint(observationID string, fingerprint []byte) error {
	var existing []byte
	row := writer.transaction.QueryRowContext(writer.context, existingFingerprintSQL, writer.envelope.ScraperID, observationID)
	if err := row.Scan(&existing); err != nil {
		return err
	}
	if !bytes.Equal(existing, fingerprint) {
		return document.Permanent("observation %s: conflicting content for existing identity", observationID)
	}
	return nil
}

func (writer payloadWriter) persistNewObservation(observation document.Observation) error {
	assignment, err := writer.assignListing(observation)
	if err != nil {
		return err
	}
	scraperID := writer.envelope.ScraperID
	unchanged, err := history.SkipUnchanged(writer.context, writer.transaction, assignment.listingID, assignment.productID, scraperID, observation)
	if err != nil || unchanged {
		return err
	}
	offers := storedOffers(observation.Offers)
	for position := 0; position < len(offers) && err == nil; position++ {
		err = writer.persistOffer(observation, assignment, position, offers[position])
	}
	return err
}

func (writer payloadWriter) assignListing(observation document.Observation) (listingAssignment, error) {
	product := observation.Product
	productID, method, err := catalog.MatchProduct(writer.context, writer.transaction, writer.envelope.ShopID, product)
	if err != nil {
		return listingAssignment{}, err
	}
	if err := monitoring.Associate(writer.context, writer.transaction, writer.targets, productID, product.Name); err != nil {
		return listingAssignment{}, err
	}
	listingID, err := writer.upsertListing(observation, productID)
	return listingAssignment{listingID, productID, method}, err
}

func (writer payloadWriter) upsertListing(observation document.Observation, productID int64) (int64, error) {
	product := observation.Product
	row := writer.transaction.QueryRowContext(writer.context, upsertListingSQL, writer.envelope.ShopID,
		product.SourceID, product.Name, product.Brand, product.GTIN, observation.ProductURL, observation.ObservedAt, productID)
	var listingID int64
	err := row.Scan(&listingID)
	if errors.Is(err, sql.ErrNoRows) {
		return writer.existingListing(product.SourceID)
	}
	return listingID, err
}

func (writer payloadWriter) existingListing(sourceID string) (int64, error) {
	var listingID int64
	row := writer.transaction.QueryRowContext(writer.context, existingListingSQL, writer.envelope.ShopID, sourceID)
	err := row.Scan(&listingID)
	return listingID, err
}

func storedOffers(offers []document.Offer) []document.Offer {
	if len(offers) == 0 {
		return []document.Offer{{}}
	}
	return offers
}

func packageValues(size *catalog.PackageSize) (quantity, unit, count any) {
	if size == nil {
		return nil, nil, nil
	}
	return size.Quantity, size.Unit, size.Count
}

func (writer payloadWriter) observationArguments(observation document.Observation, assignment listingAssignment, index int) []any {
	quantity, unit, count := packageValues(observation.Product.Package)
	location := observation.Context
	return []any{writer.identity, assignment.listingID, writer.envelope.ScraperID, observation.ID,
		observation.ObservedAt, observation.SourceURL, observation.CollectionMethod, quantity, unit, count,
		location.SellerID, location.StoreID, location.PostalCode, location.City, location.State, location.Channel,
		observation.Availability, len(observation.Offers), index}
}

func offerArguments(offer document.Offer) []any {
	conditions := offer.Conditions
	return []any{offer.Amount, offer.Currency, offer.Basis, offer.ReferenceAmount, conditions.PaymentMethod,
		conditions.Installments, conditions.MembershipRequired, conditions.MembershipProgram,
		conditions.CouponRequired, conditions.MinimumQuantity, conditions.Description}
}

func (writer payloadWriter) persistOffer(observation document.Observation, assignment listingAssignment, index int, offer document.Offer) error {
	arguments := writer.observationArguments(observation, assignment, index)
	arguments = append(arguments, offerArguments(offer)...)
	arguments = append(arguments, assignment.productID, assignment.matchMethod)
	_, err := writer.transaction.ExecContext(writer.context, insertOfferSQL, arguments...)
	return err
}
