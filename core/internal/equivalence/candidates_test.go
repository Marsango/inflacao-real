package equivalence

import (
	"testing"

	"inflacao-real/core/internal/catalog"
)

func brandedProduct(identity int64, name string) Product {
	return Product{ID: identity, Representative: identity, Name: name, Brand: stringPointer("Pilão"),
		Package: &catalog.PackageSize{Quantity: "500", Unit: "g", Count: 1}, Bases: []string{"package"}}
}

func TestExactSuggestionsNormalizeUnitsAndWordOrder(test *testing.T) {
	first := brandedProduct(1, "Café Pilão Extra Forte 500g")
	first.Barcode = stringPointer("12345670")
	second := brandedProduct(2, "500G FORTE Extra PILAO Cafe")
	second.Barcode = stringPointer("12345671")
	candidate, ok := Suggest(first, second)
	if !ok || candidate.Kind != "exact" || candidate.Score != 1 {
		test.Fatalf("missing exact suggestion: %+v", candidate)
	}
	second.Name = "Café Pilão Extra Forte 0,5kg"
	second.Package = &catalog.PackageSize{Quantity: "0.5", Unit: "kg", Count: 1}
	if candidate, ok = Suggest(first, second); !ok || candidate.Kind != "exact" {
		test.Fatal("exact unit normalization lost", candidate)
	}
}

func TestFuzzySuggestionsUseTokensAndTolerateTypos(test *testing.T) {
	first := brandedProduct(1, "Café Pilão Torrado Moído Tradicional 500g")
	for _, name := range []string{"Café Pilão Torrdo Moído Tradicional 500g", "Café Pilão Torrado Torrado Moído Tradicional 500g", "Café Pilão Torrado Moído Tradicional Selecionado 500g"} {
		assertFuzzySuggestion(test, first, brandedProduct(2, name))
	}
}

func TestConflictingVariantsAreNotSuggested(test *testing.T) {
	for _, pair := range [][2]string{{"Forte", "Suave"}, {"Chocolate", "Morango"}, {"Integral", "Desnatado"}, {"Vácuo", "Almofada"}, {"Zero Açúcar", "Tradicional"}, {"Sem Lactose", "Tradicional"}, {"10%", "20%"}} {
		first := brandedProduct(1, "Café Pilão Torrado Moído "+pair[0]+" 500g")
		second := brandedProduct(2, "Café Pilão Torrado Moído "+pair[1]+" 500g")
		assertNotSuggested(test, first, second)
	}
	first := brandedProduct(1, "Café Pilão Extra Forte 500g")
	second := brandedProduct(2, first.Name)
	second.Package.Count = 2
	if _, ok := Suggest(first, second); ok {
		test.Fatal("component counts ignored")
	}
	second.Package.Count = 1
	second.Package.Quantity = "250"
	if _, ok := Suggest(first, second); ok {
		test.Fatal("size ignored")
	}
}

func TestPilaoOmittedPackagingRequiresVerification(test *testing.T) {
	first := brandedProduct(133, "Café Pilão Torrado E Moído Extra Forte Vácuo 500g")
	second := brandedProduct(3103, "Café Torrado e Moído PILÃO Extra Forte 500g")
	candidate, ok := Suggest(first, second)
	if !ok {
		return
	} // Discovery is optional; confirmed equivalence is never implied.
	if candidate.Kind == "exact" || !candidate.VerificationRequired || !contains(candidate.Differences, "vacuo") {
		test.Fatal("omitted packaging hidden", candidate)
	}
}

func assertFuzzySuggestion(test *testing.T, first, second Product) {
	test.Helper()
	candidate, ok := Suggest(first, second)
	if !ok || candidate.Kind != "fuzzy" {
		test.Fatal("controlled fuzzy fixture missing", second.Name, candidate)
	}
}
func assertNotSuggested(test *testing.T, first, second Product) {
	test.Helper()
	if candidate, ok := Suggest(first, second); ok {
		test.Fatal("conflicting variants suggested", candidate)
	}
}
