package catalog

import "math/big"

type PackageSize struct {
	Quantity string
	Unit     string
	Count    int
}

type Product struct {
	SourceID string `json:"source_product_id"`
	Name     string
	Brand    *string
	GTIN     *string
	Package  *PackageSize
}

type catalogProduct struct {
	ID int64
	Product
}

func compatibleProduct(source, candidate Product) bool {
	if source.GTIN != nil && candidate.GTIN != nil && normalizedGTIN(*source.GTIN) != normalizedGTIN(*candidate.GTIN) {
		return false
	}
	if source.Brand != nil && candidate.Brand != nil && normalizedText(*source.Brand) != normalizedText(*candidate.Brand) {
		return false
	}
	if source.Package != nil && candidate.Package != nil {
		return source.Package.equivalent(candidate.Package)
	}
	return true
}

func (size *PackageSize) equivalent(other *PackageSize) bool {
	quantity, unit := comparablePackage(size)
	otherQuantity, otherUnit := comparablePackage(other)
	return unit == otherUnit && quantity.Cmp(otherQuantity) == 0 && size.Count == other.Count
}

func comparablePackage(size *PackageSize) (*big.Rat, string) {
	quantity, _ := new(big.Rat).SetString(size.Quantity)
	if size.Unit == "kg" || size.Unit == "l" {
		quantity.Mul(quantity, big.NewRat(1000, 1))
	}
	units := map[string]string{"kg": "g", "l": "ml"}
	if unit, exists := units[size.Unit]; exists {
		return quantity, unit
	}
	return quantity, size.Unit
}

func (source Product) attributeMatchable() bool {
	return source.Brand != nil && normalizedText(*source.Brand) != "" && source.Package != nil && nameSignature(source.Name) != ""
}

func (source Product) matchesAttributes(candidate catalogProduct) bool {
	return candidate.Brand != nil && normalizedText(*source.Brand) == normalizedText(*candidate.Brand) &&
		nameSignature(source.Name) == nameSignature(candidate.Name) && compatibleProduct(source, candidate.Product)
}

func (source Product) matchesListing(candidate catalogProduct, previousName string) bool {
	if !compatibleProduct(source, candidate.Product) {
		return false
	}
	if source.GTIN != nil && candidate.GTIN != nil && normalizedGTIN(*source.GTIN) == normalizedGTIN(*candidate.GTIN) {
		return true
	}
	return nameSignature(source.Name) != "" && nameSignature(source.Name) == nameSignature(previousName)
}

func (source Product) packageValues() (quantity, unit, count any) {
	if source.Package == nil {
		return nil, nil, nil
	}
	return source.Package.Quantity, source.Package.Unit, source.Package.Count
}

func (source Product) gtinValue() any {
	if source.GTIN == nil {
		return nil
	}
	return normalizedGTIN(*source.GTIN)
}
