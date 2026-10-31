package equivalence

import (
	"errors"
	"sync"
	"testing"
)

func TestGenericBackfillAndAudit(test *testing.T) {
	fixture := newFixture(test)
	fixture.seedLooseEvidence()
	preview := fixture.snapshot()
	if preview.GenericEnabled || len(preview.Generic) != 3 {
		test.Fatalf("bad preview %+v", preview)
	}
	fixture.require(fixture.apply("activate", 0, 0))
	grouped := fixture.snapshot()
	if !grouped.GenericEnabled || len(grouped.Generic) != 0 {
		test.Fatal("backfill did not group")
	}
	for _, product := range grouped.Products {
		assertRepresentative(test, product, 1)
	}
	assertAuditAndCodes(fixture)
}

func assertAuditAndCodes(fixture fixture) {
	test := fixture.test
	var audited int
	fixture.require(fixture.database.QueryRowContext(fixture.requestContext, `SELECT count(*) FROM equivalence_audit WHERE rule_version=$1 AND actor='automatic'`, RuleVersion).Scan(&audited))
	if audited != 2 {
		test.Fatal("automatic decisions missing rule audit", audited)
	}
	var codes int
	fixture.require(fixture.database.QueryRowContext(fixture.requestContext, `SELECT count(DISTINCT gtin) FROM products`).Scan(&codes))
	if codes != 3 {
		test.Fatal("codes rewritten")
	}
}

func assertRepresentative(test *testing.T, product Product, expected int64) {
	test.Helper()
	if product.Representative != expected {
		test.Fatalf("product %d representative %d, expected %d", product.ID, product.Representative, expected)
	}
}

func TestRemovalDissolutionAndManualOverrides(test *testing.T) {
	fixture := newFixture(test)
	fixture.seedLooseEvidence()
	fixture.require(fixture.apply("activate", 0, 0))
	fixture.require(fixture.apply("remove", 1, 0))
	products := fixture.snapshot().Products
	assertRepresentative(test, products[0], 1)
	assertRepresentative(test, products[1], 2)
	assertRepresentative(test, products[2], 2)
	fixture.require(fixture.apply("activate", 0, 0))
	if len(fixture.snapshot().Generic) != 0 {
		test.Fatal("removed pair resurfaced")
	}
	verifyDissolution(fixture)
}

func verifyDissolution(fixture fixture) {
	test := fixture.test
	fixture.require(fixture.apply("dissolve", 2, 0))
	fixture.require(fixture.apply("activate", 0, 0))
	products := fixture.snapshot().Products
	for _, product := range products {
		assertRepresentative(test, product, product.ID)
	}
	if err := fixture.apply("approve", 1, 2); !errors.Is(err, ErrInvalid) {
		test.Fatal("manual exclusion bypassed", err)
	}
}

func TestRejectionOverridesTransitiveAutomaticGrouping(test *testing.T) {
	fixture := newFixture(test)
	fixture.seedLooseEvidence()
	fixture.require(fixture.apply("reject", 1, 3))
	fixture.require(fixture.apply("activate", 0, 0))
	products := fixture.snapshot().Products
	if products[0].Representative == products[2].Representative {
		test.Fatal("rejected pair combined transitively")
	}
	if len(fixture.snapshot().Generic) != 0 {
		test.Fatal("blocked group reappeared")
	}
}

func TestConcurrentReviewsAndRepresentativeSelection(test *testing.T) {
	fixture := newFixture(test)
	fixture.seedLooseEvidence()
	revision := fixture.snapshot().Revision
	results := make(chan error, 2)
	var group sync.WaitGroup
	group.Add(2)
	for range 2 {
		go concurrentDecision(fixture, revision, results, &group)
	}
	group.Wait()
	close(results)
	successes, stale := countResults(results)
	if successes != 1 || stale != 1 {
		test.Fatal("reviews not serialized", successes, stale)
	}
	decision := Decision{Revision: fixture.snapshot().Revision, Action: "representative", First: 1, Representative: 2, Reason: "Nome mais claro"}
	fixture.require(Apply(fixture.requestContext, fixture.database, "owner", decision))
	assertRepresentative(test, fixture.snapshot().Products[0], 2)
}

func concurrentDecision(fixture fixture, revision int64, results chan error, group *sync.WaitGroup) {
	defer group.Done()
	results <- Apply(fixture.requestContext, fixture.database, "owner", Decision{Revision: revision, Action: "approve", First: 1, Second: 2, Reason: "Verified"})
}
func countResults(results chan error) (int, int) {
	successes, stale := 0, 0
	for err := range results {
		successes, stale = countResult(successes, stale, err)
	}
	return successes, stale
}

func TestOmittedPackagingCannotBridgeConflictingGroups(test *testing.T) {
	fixture := newFixture(test)
	fixture.execute(`UPDATE products SET brand='Pilão',package_quantity=500,package_unit='g',package_count=1,
  name=CASE id WHEN 1 THEN 'Café Pilão Extra Forte Vácuo 500g' WHEN 2 THEN 'Café Pilão Extra Forte 500g' ELSE 'Café Pilão Extra Forte Almofada 500g' END`)
	fixture.require(fixture.apply("approve", 1, 2))
	if err := fixture.apply("approve", 2, 3); !errors.Is(err, ErrInvalid) {
		test.Fatal("conflicting packaging merged transitively", err)
	}
	products := fixture.snapshot().Products
	assertRepresentative(test, products[0], 1)
	assertRepresentative(test, products[2], 3)
}

func countResult(successes, stale int, err error) (int, int) {
	if err == nil {
		return successes + 1, stale
	}
	if errors.Is(err, ErrStale) {
		return successes, stale + 1
	}
	return successes, stale
}

func TestMissingPackagingRequiresVerificationAcrossAllMembers(test *testing.T) {
	fixture := newFixture(test)
	fixture.execute(`UPDATE products SET brand='Pilão',package_quantity=500,package_unit='g',package_count=1,
  name=CASE id WHEN 1 THEN 'Café Pilão Extra Forte Vácuo 500g' ELSE 'Café Pilão Extra Forte 500g' END`)
	fixture.require(fixture.apply("approve", 1, 2))
	decision := Decision{Revision: fixture.snapshot().Revision, Action: "approve", First: 2, Second: 3, Reason: "Same normalized title"}
	if err := Apply(fixture.requestContext, fixture.database, "owner", decision); !errors.Is(err, ErrInvalid) {
		test.Fatal("group packaging omission not verified", err)
	}
	decision.SourceVerified = true
	fixture.require(Apply(fixture.requestContext, fixture.database, "owner", decision))
}

func TestRemovingAnotherMemberPreservesOwnerRepresentative(test *testing.T) {
	fixture := newFixture(test)
	fixture.seedLooseEvidence()
	fixture.require(fixture.apply("activate", 0, 0))
	decision := Decision{Revision: fixture.snapshot().Revision, Action: "representative", First: 1, Representative: 3, Reason: "Nome escolhido"}
	fixture.require(Apply(fixture.requestContext, fixture.database, "owner", decision))
	fixture.require(fixture.apply("remove", 1, 0))
	assertRepresentative(test, fixture.snapshot().Products[1], 3)
}
