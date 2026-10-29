package catalog

import "testing"

type signatureCase struct {
	first, second string
	same          bool
}

func signatureCases() []signatureCase {
	return []signatureCase{
		{"CAFÉ Torrado — Marca A 500g", "Marca A cafe torrado 0,5 kg", true},
		{"Arroz Branco 5KG", "BRANCO ARROZ 5000 g", true},
		{"Leite Integral 1 L", "integral leite 1000ml", true},
		{"Leite Integral 1L", "Leite Desnatado 1L", false},
		{"Café 500g", "Café 250g", false},
		{"Suco zero açúcar 1L", "Suco açúcar 1L", false},
		{"Cerveja 6 x 350ml", "Cerveja 1 x 2100ml", false},
		{"Leite A2 1L", "Leite A1 1L", false},
		{"Chocolate 70% cacau 100g", "Chocolate 50% cacau 100g", false},
		{"Arroz tipo 1 5kg", "Arroz tipo 2 5kg", false},
	}
}

func TestProductNameSignatures(test *testing.T) {
	for _, scenario := range signatureCases() {
		test.Run(scenario.first+" / "+scenario.second, func(test *testing.T) { checkSignature(test, scenario) })
	}
}

func checkSignature(test *testing.T, scenario signatureCase) {
	first, second := nameSignature(scenario.first), nameSignature(scenario.second)
	if (first == second) != scenario.same {
		test.Fatalf("signatures %q and %q: want same=%t", first, second, scenario.same)
	}
}

func text(value string) *string { return &value }

type compatibilityCase struct {
	name       string
	change     func(*Product)
	compatible bool
}

func compatibilityCases() []compatibilityCase {
	return []compatibilityCase{
		{"same", func(*Product) {}, true},
		{"GTIN padding", func(p *Product) { p.GTIN = text("04006381333931") }, true},
		{"brand spelling", func(p *Product) { p.Brand = text("CAFE-EXEMPLO") }, true},
		{"equivalent units", func(p *Product) { p.Package = &PackageSize{"500", "g", 1} }, true},
		{"exact small quantity", func(p *Product) { p.Package = &PackageSize{"499.999999", "g", 1} }, false},
		{"different brand", func(p *Product) { p.Brand = text("Outro Café") }, false},
		{"different barcode", func(p *Product) { p.GTIN = text("036000291452") }, false},
		{"different count", func(p *Product) { p.Package = &PackageSize{"0.5", "kg", 2} }, false},
		{"different measure", func(p *Product) { p.Package = &PackageSize{"0.5", "l", 1} }, false},
		// Missing values are not contradictions. Cross-shop attribute matching
		// separately requires brand and complete package information.
		{"unknown barcode", func(p *Product) { p.GTIN = nil }, true},
		{"unknown package", func(p *Product) { p.Package = nil }, true},
	}
}

func TestProductCompatibility(test *testing.T) {
	for _, scenario := range compatibilityCases() {
		test.Run(scenario.name, func(test *testing.T) { checkCompatibility(test, scenario) })
	}
	if normalizedGTIN("96385074") != "00000096385074" || normalizedGTIN("036000291452") != "00036000291452" {
		test.Fatal("short GTINs must compare in their zero-padded form")
	}
}

func checkCompatibility(test *testing.T, scenario compatibilityCase) {
	original := Product{Brand: text("Café Exemplo"), GTIN: text("4006381333931"), Package: &PackageSize{Quantity: "0.5", Unit: "kg", Count: 1}}
	other := original
	scenario.change(&other)
	if result := compatibleProduct(original, other); result != scenario.compatible {
		test.Fatalf("compatible=%t, want %t", result, scenario.compatible)
	}
}
