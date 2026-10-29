package catalog

import (
	"context"
	"database/sql"
	"errors"
)

type productMatcher struct {
	context     context.Context
	transaction *sql.Tx
	shopID      string
	source      Product
}

type matchingRule struct {
	method string
	find   func() (*catalogProduct, error)
}

// MatchProduct runs under the caller's transaction-scoped catalog lock.
func MatchProduct(requestContext context.Context, transaction *sql.Tx, shopID string, source Product) (int64, string, error) {
	matcher := productMatcher{requestContext, transaction, shopID, source}
	return matcher.match()
}

func (matcher productMatcher) match() (int64, string, error) {
	rules := []matchingRule{{"gtin_v1", matcher.byGTIN}, {"listing_v1", matcher.byListing}, {"attributes_v1", matcher.byAttributes}}
	var selected *catalogProduct
	var err error
	method := "created_v1"
	for position := 0; position < len(rules) && selected == nil && err == nil; position++ {
		method = rules[position].method
		selected, err = rules[position].find()
	}
	if err != nil {
		return 0, "", err
	}
	if selected != nil {
		return matcher.enrich(*selected, method)
	}
	return matcher.create()
}

func (matcher productMatcher) byGTIN() (*catalogProduct, error) {
	if matcher.source.GTIN == nil {
		return nil, nil
	}
	candidates, err := matcher.candidates(`lpad(gtin,14,'0')=$1`, normalizedGTIN(*matcher.source.GTIN))
	if err != nil {
		return nil, err
	}
	return uniqueCandidate(candidates, matcher.compatible), nil
}

func (matcher productMatcher) compatible(candidate catalogProduct) bool {
	return compatibleProduct(matcher.source, candidate.Product)
}

func (matcher productMatcher) byListing() (*catalogProduct, error) {
	var identity int64
	var previousName string
	row := matcher.transaction.QueryRowContext(matcher.context, currentListingSQL, matcher.shopID, matcher.source.SourceID)
	err := row.Scan(&identity, &previousName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return matcher.listingCandidate(identity, previousName)
}

func (matcher productMatcher) listingCandidate(identity int64, previousName string) (*catalogProduct, error) {
	candidates, err := matcher.candidates(`id=$1`, identity)
	if err != nil {
		return nil, err
	}
	if len(candidates) != 1 {
		return nil, nil
	}
	if matcher.source.matchesListing(candidates[0], previousName) {
		return &candidates[0], nil
	}
	return nil, nil
}

func (matcher productMatcher) byAttributes() (*catalogProduct, error) {
	if !matcher.source.attributeMatchable() {
		return nil, nil
	}
	quantity, unit := comparablePackage(matcher.source.Package)
	candidates, err := matcher.candidates(attributeCandidatesSQL, matcher.source.Package.Count, unit, quantity.FloatString(6))
	if err != nil {
		return nil, err
	}
	return uniqueCandidate(candidates, matcher.source.matchesAttributes), nil
}

func uniqueCandidate(candidates []catalogProduct, matches func(catalogProduct) bool) *catalogProduct {
	var selected *catalogProduct
	ambiguous := false
	for position := 0; position < len(candidates) && !ambiguous; position++ {
		selected, ambiguous = considerCandidate(selected, candidates[position], matches)
	}
	return selected
}

func considerCandidate(selected *catalogProduct, candidate catalogProduct, matches func(catalogProduct) bool) (*catalogProduct, bool) {
	if !matches(candidate) {
		return selected, false
	}
	if selected != nil {
		return nil, true
	}
	return &candidate, false
}
