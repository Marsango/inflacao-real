package equivalence

import (
	"testing"

	"inflacao-real/core/internal/catalog"
)

func looseProduct(name string) Product {
	return Product{Name: name, Loose: true, Bases: []string{"kg"}}
}
func stringPointer(value string) *string { return &value }

func TestCommodityAliasesAndDepartments(test *testing.T) {
	first := looseProduct("Banana Maçã Kg")
	first.Brand = stringPointer("Hortifruti")
	second := looseProduct("Banana Maca KG")
	if CommodityKey(first) == "" || CommodityKey(first) != CommodityKey(second) {
		test.Fatal("department or accents prevented equivalent bananas")
	}
	nanica := looseProduct("Banana Nanica Kg")
	for _, name := range []string{"Banana Caturra Kg", "Banana Caturra/Nanica Kg"} {
		assertCommodityAlias(test, nanica, looseProduct(name))
	}
	meat := looseProduct("Carne Bovina Bife De Patinho Kg")
	meat.Brand = stringPointer("Açougue")
	if CommodityKey(meat) == "" {
		test.Fatal("department meat preparation not recognized")
	}
}

func TestCommodityUnknownAndBrandedRecordsStaySeparate(test *testing.T) {
	for _, name := range []string{"Banana Kg", "Banana Maçã Premium Kg", "Banana Maçã MarcaX Kg", "Alface Kg", "Patinho Temperado Kg", "Patinho Fresco Congelado Kg", "Banana Maçã Bandeja Kg", "Banana Maçã 1 Kg"} {
		assertNotCommodity(test, looseProduct(name))
	}
	product := looseProduct("Patinho Kg")
	product.Brand = stringPointer("Muffato Foods")
	if CommodityKey(product) != "" {
		test.Fatal("real brand treated as department")
	}
	product.Brand = stringPointer("Hortifruti")
	if CommodityKey(product) != "" {
		test.Fatal("department allowlist crossed categories")
	}
}

func TestCommoditySeparatesAttributes(test *testing.T) {
	names := []string{"Patinho Kg", "Patinho Inteiro Kg", "Patinho Bife Kg", "Patinho Tiras Kg", "Patinho Cubos Kg", "Patinho Moído Kg", "Patinho Orgânico Kg", "Patinho Congelado Kg", "Patinho Fresco Kg"}
	keys := map[string]bool{}
	for _, name := range names {
		recordDistinctCommodity(test, keys, name)
	}
	assertSeparateVarieties(test)
}

func assertSeparateVarieties(test *testing.T) {
	test.Helper()
	keys := map[string]bool{}
	for _, name := range []string{"Banana Maçã Kg", "Banana Prata Kg", "Banana Terra Kg", "Banana Nanica Kg"} {
		recordDistinctCommodity(test, keys, name)
	}
}

func TestCommodityRequiresLooseKilogramAndConsistentListings(test *testing.T) {
	product := looseProduct("Banana Maçã Kg")
	product.Bases = []string{"unit"}
	if CommodityKey(product) != "" {
		test.Fatal("unit sale matched")
	}
	product.Bases = []string{"kg"}
	product.Package = &catalog.PackageSize{Quantity: "1", Unit: "kg", Count: 1}
	if CommodityKey(product) != "" {
		test.Fatal("fixed package matched")
	}
	product.Package = nil
	product.Listings = []Listing{{Name: "Banana Prata Kg"}}
	if CommodityKey(product) != "" {
		test.Fatal("conflicting listing matched")
	}
}

func TestReviewFixturesAreNeverAutomaticallyGrouped(test *testing.T) {
	for _, name := range []string{"Alface Crespa Kg", "Alface Verde Kg", "Nescafé Solúvel Original Kg"} {
		assertNotCommodity(test, looseProduct(name))
	}
	product := looseProduct("Banana Maçã Kg")
	product.Brand = stringPointer("Nescafé")
	assertNotCommodity(test, product)
}
func assertNotCommodity(test *testing.T, product Product) {
	test.Helper()
	if key := CommodityKey(product); key != "" {
		test.Fatal("review fixture auto recognized", key)
	}
}

func TestManualLooseReviewStillPreservesPreparation(test *testing.T) {
	first := looseProduct("Patinho Kg")
	for _, name := range []string{"Patinho Temperado Kg", "Patinho Moído Kg", "Patinho Congelado Kg", "Patinho Orgânico Kg"} {
		assertManualSeparation(test, first, looseProduct(name))
	}
}
func assertManualSeparation(test *testing.T, first, second Product) {
	test.Helper()
	if manualLooseComparable(first, second, true) {
		test.Fatal("manual review overrode preparation", second.Name)
	}
}

func assertCommodityAlias(test *testing.T, first, second Product) {
	test.Helper()
	if CommodityKey(first) != CommodityKey(second) {
		test.Fatal("explicit alias lost", second.Name)
	}
}
func recordDistinctCommodity(test *testing.T, keys map[string]bool, name string) {
	test.Helper()
	key := CommodityKey(looseProduct(name))
	if key == "" || keys[key] {
		test.Fatal("attributes combined", name, key)
	}
	keys[key] = true
}
