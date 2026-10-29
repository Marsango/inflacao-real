package ingestion

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type catalogProduct struct {
	ID int64
	sourceProduct
}

// Product decisions run under persistEnvelope's transaction-scoped catalog lock.
// A missing or ambiguous cross-shop identity creates a separate product rather
// than discarding the observation or guessing which variant it describes.
func matchProduct(ctx context.Context, tx *sql.Tx, shopID string, source sourceProduct) (int64, string, error) {
	var selected *catalogProduct
	method := "created_v1"
	if source.GTIN != nil {
		candidates, err := catalogCandidates(ctx, tx, `lpad(gtin,14,'0')=$1`, normalizedGTIN(*source.GTIN))
		if err != nil {
			return 0, "", err
		}
		for _, candidate := range candidates {
			if compatibleProduct(source, candidate.sourceProduct) {
				if selected != nil {
					selected = nil // Ambiguous GTIN data must not pick an arbitrary product.
					break
				}
				copy := candidate
				selected = &copy
			}
		}
		if selected != nil {
			method = "gtin_v1"
		}
	}
	if selected == nil {
		// Shop/SKU continuity is useful even when the retailer omits a barcode.
		// A changed variant must not silently keep the previous product link.
		var currentID int64
		var previousName string
		err := tx.QueryRowContext(ctx, `SELECT product_id,name FROM shop_products
			WHERE shop_id=$1 AND source_product_id=$2 AND product_id IS NOT NULL`, shopID, source.SourceID).Scan(&currentID, &previousName)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, "", err
		}
		if err == nil {
			candidates, err := catalogCandidates(ctx, tx, `id=$1`, currentID)
			if err != nil {
				return 0, "", err
			}
			if len(candidates) == 1 && compatibleProduct(source, candidates[0].sourceProduct) {
				candidate := candidates[0]
				sameGTIN := source.GTIN != nil && candidate.GTIN != nil && normalizedGTIN(*source.GTIN) == normalizedGTIN(*candidate.GTIN)
				if sameGTIN || (nameSignature(source.Name) != "" && nameSignature(source.Name) == nameSignature(previousName)) {
					selected, method = &candidate, "listing_v1"
				}
			}
		}
	}
	if selected == nil && source.Brand != nil && normalizedText(*source.Brand) != "" && source.Package != nil && nameSignature(source.Name) != "" {
		quantity, unit := comparablePackage(source.Package)
		candidates, err := catalogCandidates(ctx, tx, `package_count=$1
			AND CASE package_unit WHEN 'kg' THEN 'g' WHEN 'l' THEN 'ml' ELSE package_unit END=$2
			AND package_quantity*CASE WHEN package_unit IN ('kg','l') THEN 1000 ELSE 1 END=$3::numeric`,
			source.Package.Count, unit, quantity.FloatString(6))
		if err != nil {
			return 0, "", err
		}
		for _, candidate := range candidates {
			if candidate.Brand != nil && normalizedText(*source.Brand) == normalizedText(*candidate.Brand) &&
				nameSignature(source.Name) == nameSignature(candidate.Name) && compatibleProduct(source, candidate.sourceProduct) {
				if selected != nil {
					selected = nil
					break
				}
				copy := candidate
				selected = &copy
			}
		}
		if selected != nil {
			method = "attributes_v1"
		}
	}
	var quantity, unit, count, gtin any
	if source.Package != nil {
		quantity, unit, count = source.Package.Quantity, source.Package.Unit, source.Package.Count
	}
	if source.GTIN != nil {
		gtin = normalizedGTIN(*source.GTIN)
	}
	if selected != nil {
		// Fill unknown attributes when later observations provide them. Never
		// replace a known canonical value with an incompatible measurement.
		_, err := tx.ExecContext(ctx, `UPDATE products SET brand=COALESCE(brand,$2),gtin=COALESCE(gtin,$3),
			package_quantity=COALESCE(package_quantity,$4),package_unit=COALESCE(package_unit,$5),
			package_count=COALESCE(package_count,$6) WHERE id=$1`, selected.ID, source.Brand, gtin, quantity, unit, count)
		return selected.ID, method, err
	}
	var id int64
	err := tx.QueryRowContext(ctx, `INSERT INTO products (name,brand,gtin,package_quantity,package_unit,package_count)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, source.Name, source.Brand, gtin, quantity, unit, count).Scan(&id)
	return id, method, err
}

func catalogCandidates(ctx context.Context, tx *sql.Tx, condition string, args ...any) ([]catalogProduct, error) {
	// condition is supplied only by the fixed queries above; source values are parameters.
	rows, err := tx.QueryContext(ctx, `SELECT id,name,brand,gtin,package_quantity::text,package_unit,package_count
		FROM products WHERE `+condition+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []catalogProduct
	for rows.Next() {
		var p catalogProduct
		var quantity, unit *string
		var count *int
		if err := rows.Scan(&p.ID, &p.Name, &p.Brand, &p.GTIN, &quantity, &unit, &count); err != nil {
			return nil, err
		}
		if quantity != nil {
			p.Package = &packageSize{Quantity: *quantity, Unit: *unit, Count: *count}
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func normalizedGTIN(value string) string { return strings.Repeat("0", 14-len(value)) + value }

func compatibleProduct(a, b sourceProduct) bool {
	if a.GTIN != nil && b.GTIN != nil && normalizedGTIN(*a.GTIN) != normalizedGTIN(*b.GTIN) {
		return false
	}
	if a.Brand != nil && b.Brand != nil && normalizedText(*a.Brand) != normalizedText(*b.Brand) {
		return false
	}
	if a.Package != nil && b.Package != nil {
		aq, au := comparablePackage(a.Package)
		bq, bu := comparablePackage(b.Package)
		return au == bu && aq.Cmp(bq) == 0 && a.Package.Count == b.Package.Count
	}
	return true
}

func comparablePackage(p *packageSize) (*big.Rat, string) {
	quantity, _ := new(big.Rat).SetString(p.Quantity)
	unit := p.Unit
	if unit == "kg" || unit == "l" {
		quantity.Mul(quantity, big.NewRat(1000, 1))
		if unit == "kg" {
			unit = "g"
		} else {
			unit = "ml"
		}
	}
	return quantity, unit
}

func normalizedText(value string) string {
	var out strings.Builder
	var previous rune
	for _, r := range norm.NFD.String(strings.ToLower(value)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			out.WriteByte(' ')
			previous = 0
			continue
		}
		if previous != 0 && unicode.IsDigit(r) != unicode.IsDigit(previous) {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
		previous = r
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

var namedQuantity = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]{1,6})?)\s*(kg|ml|g|l)\b`)

func nameSignature(value string) string {
	// Canonicalize explicit measures without dropping size or variant tokens.
	value = namedQuantity.ReplaceAllStringFunc(value, func(measure string) string {
		parts := namedQuantity.FindStringSubmatch(measure)
		p := packageSize{Quantity: strings.ReplaceAll(parts[1], ",", "."), Unit: strings.ToLower(parts[2])}
		quantity, unit := comparablePackage(&p)
		return quantity.FloatString(6) + " " + unit
	})
	words := strings.Fields(normalizedText(value))
	sort.Strings(words)
	return strings.Join(words, " ")
}
