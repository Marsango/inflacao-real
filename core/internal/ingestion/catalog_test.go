package ingestion

import "testing"

func TestProductNameSignatures(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		same bool
	}{
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
	} {
		t.Run(tc.a+" / "+tc.b, func(t *testing.T) {
			a, b := nameSignature(tc.a), nameSignature(tc.b)
			if (a == b) != tc.same {
				t.Fatalf("signatures %q and %q: want same=%t", a, b, tc.same)
			}
		})
	}
}

func TestProductCompatibility(t *testing.T) {
	text := func(value string) *string { return &value }
	original := sourceProduct{Brand: text("Café Exemplo"), GTIN: text("4006381333931"),
		Package: &packageSize{Quantity: "0.5", Unit: "kg", Count: 1}}
	for _, tc := range []struct {
		name       string
		change     func(*sourceProduct)
		compatible bool
	}{
		{"same", func(*sourceProduct) {}, true},
		{"GTIN padding", func(p *sourceProduct) { p.GTIN = text("04006381333931") }, true},
		{"brand spelling", func(p *sourceProduct) { p.Brand = text("CAFE-EXEMPLO") }, true},
		{"equivalent units", func(p *sourceProduct) { p.Package = &packageSize{"500", "g", 1} }, true},
		{"exact small quantity", func(p *sourceProduct) { p.Package = &packageSize{"499.999999", "g", 1} }, false},
		{"different brand", func(p *sourceProduct) { p.Brand = text("Outro Café") }, false},
		{"different barcode", func(p *sourceProduct) { p.GTIN = text("036000291452") }, false},
		{"different count", func(p *sourceProduct) { p.Package = &packageSize{"0.5", "kg", 2} }, false},
		{"different measure", func(p *sourceProduct) { p.Package = &packageSize{"0.5", "l", 1} }, false},
		// Missing values are not contradictions. Cross-shop attribute matching
		// separately requires brand and complete package information.
		{"unknown barcode", func(p *sourceProduct) { p.GTIN = nil }, true},
		{"unknown package", func(p *sourceProduct) { p.Package = nil }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := original
			tc.change(&other)
			if got := compatibleProduct(original, other); got != tc.compatible {
				t.Fatalf("compatible=%t, want %t", got, tc.compatible)
			}
		})
	}
	if normalizedGTIN("96385074") != "00000096385074" || normalizedGTIN("036000291452") != "00036000291452" {
		t.Fatal("short GTINs must compare in their zero-padded form")
	}
}
