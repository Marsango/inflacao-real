package equivalence

import (
	"context"
	"database/sql"
)

func (mutation change) group() (int64, error) {
	var representative int64
	err := mutation.transaction.QueryRowContext(mutation.requestContext, `SELECT equivalent_product_id(id) FROM products WHERE id=$1`, mutation.decision.First).Scan(&representative)
	if err == sql.ErrNoRows {
		return 0, ErrInvalid
	}
	return representative, err
}

func (mutation change) remove() error {
	representative, err := mutation.group()
	if err != nil {
		return err
	}
	if err := mutation.excludeRemoved(representative); err != nil {
		return err
	}
	if _, err := mutation.transaction.ExecContext(mutation.requestContext, `DELETE FROM product_equivalence WHERE product_id=$1`, mutation.decision.First); err != nil {
		return err
	}
	return mutation.repairRepresentative(representative)
}

func (mutation change) excludeRemoved(representative int64) error {
	_, err := mutation.transaction.ExecContext(mutation.requestContext, `INSERT INTO product_exclusions(first_id,second_id)
  SELECT LEAST(id,$1),GREATEST(id,$1) FROM products WHERE equivalent_product_id(id)=$2 AND id<>$1
  ON CONFLICT DO NOTHING`, mutation.decision.First, representative)
	return err
}

func (mutation change) repairRepresentative(previous int64) error {
	if previous != mutation.decision.First {
		return nil
	}
	_, err := mutation.transaction.ExecContext(mutation.requestContext, `UPDATE product_equivalence SET representative_id=
  (SELECT min(product_id) FROM product_equivalence WHERE representative_id=$1) WHERE representative_id=$1`, previous)
	return err
}

func (mutation change) dissolve() error {
	representative, err := mutation.group()
	if err != nil {
		return err
	}
	_, err = mutation.transaction.ExecContext(mutation.requestContext, `INSERT INTO product_exclusions(first_id,second_id)
  SELECT a.product_id,b.product_id FROM product_equivalence a CROSS JOIN product_equivalence b
  WHERE a.representative_id=$1 AND b.representative_id=$1 AND a.product_id<b.product_id ON CONFLICT DO NOTHING`, representative)
	if err != nil {
		return err
	}
	_, err = mutation.transaction.ExecContext(mutation.requestContext, `DELETE FROM product_equivalence WHERE representative_id=$1`, representative)
	return err
}

func (mutation change) representative() error {
	current, err := mutation.group()
	if err != nil {
		return err
	}
	var valid bool
	err = mutation.transaction.QueryRowContext(mutation.requestContext, `SELECT EXISTS(SELECT 1 FROM product_equivalence WHERE product_id=$1 AND representative_id=$2)`, mutation.decision.Representative, current).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalid
	}
	_, err = mutation.transaction.ExecContext(mutation.requestContext, `UPDATE product_equivalence SET representative_id=$2 WHERE representative_id=$1`, current, mutation.decision.Representative)
	return err
}

// Reconcile requires the same transaction-scoped catalog lock used by ingestion.
func Reconcile(requestContext context.Context, transaction *sql.Tx) error {
	var enabled bool
	if err := transaction.QueryRowContext(requestContext, `SELECT generic_enabled FROM catalog_decisions`).Scan(&enabled); err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	return reconcileEnabled(requestContext, transaction)
}

func reconcileEnabled(requestContext context.Context, transaction *sql.Tx) error {
	products, err := Load(requestContext, transaction)
	if err != nil {
		return err
	}
	exclusions, err := readExclusions(requestContext, transaction)
	if err != nil {
		return err
	}
	candidates := genericCandidates(products, exclusions)
	for position := 0; position < len(candidates) && err == nil; position++ {
		err = automaticPair(requestContext, transaction, candidates[position])
	}
	return err
}

func automaticPair(requestContext context.Context, transaction *sql.Tx, candidate Candidate) error {
	version := RuleVersion
	mutation := change{requestContext: requestContext, transaction: transaction, actor: "automatic", rule: &version,
		decision: Decision{Action: "approve", First: candidate.First, Second: candidate.Second, Reason: candidate.Reason}}
	first, second, err := mutation.pair()
	if err != nil {
		return err
	}
	if first.Representative == second.Representative {
		return nil
	}
	// A manual exclusion across any members always wins, even after earlier merges.
	err = mutation.record()
	if err == ErrInvalid {
		return nil
	}
	return err
}

func (mutation change) validateGroups(first, second Product) error {
	products, err := Load(mutation.requestContext, mutation.transaction)
	if err != nil {
		return err
	}
	for position := 0; position < len(products) && err == nil; position++ {
		err = mutation.validateMember(products[position], first, second, products)
	}
	return err
}
func (mutation change) validateMember(product, first, second Product, products []Product) error {
	if product.Representative != first.Representative {
		return nil
	}
	valid := true
	for _, other := range products {
		valid = valid && mutation.compatibleMember(product, other, second.Representative)
	}
	if !valid {
		return ErrInvalid
	}
	return nil
}
func (mutation change) compatibleMember(first, second Product, representative int64) bool {
	if second.Representative != representative {
		return true
	}
	if !mergeComparable(first, second) && !manualLooseComparable(first, second, mutation.decision.SourceVerified) {
		return false
	}
	candidate, _ := Suggest(first, second)
	return !candidate.VerificationRequired || mutation.decision.SourceVerified
}
