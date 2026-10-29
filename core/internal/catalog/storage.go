package catalog

import "database/sql"

const currentListingSQL = `SELECT product_id,name FROM shop_products
    WHERE shop_id=$1 AND source_product_id=$2 AND product_id IS NOT NULL`
const attributeCandidatesSQL = `package_count=$1
    AND CASE package_unit WHEN 'kg' THEN 'g' WHEN 'l' THEN 'ml' ELSE package_unit END=$2
    AND package_quantity*CASE WHEN package_unit IN ('kg','l') THEN 1000 ELSE 1 END=$3::numeric`
const productCandidatesSQL = `SELECT id,name,brand,gtin,package_quantity::text,package_unit,package_count
    FROM products WHERE `
const enrichProductSQL = `UPDATE products SET brand=COALESCE(brand,$2),gtin=COALESCE(gtin,$3),
    package_quantity=COALESCE(package_quantity,$4),package_unit=COALESCE(package_unit,$5),
    package_count=COALESCE(package_count,$6) WHERE id=$1`
const createProductSQL = `INSERT INTO products (name,brand,gtin,package_quantity,package_unit,package_count)
    VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`

func (matcher productMatcher) candidates(condition string, arguments ...any) ([]catalogProduct, error) {
	rows, err := matcher.transaction.QueryContext(matcher.context, productCandidatesSQL+condition+` ORDER BY id`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readCandidates(rows)
}

func readCandidates(rows *sql.Rows) ([]catalogProduct, error) {
	var products []catalogProduct
	var err error
	for err == nil && rows.Next() {
		products, err = appendCandidate(rows, products)
	}
	if err != nil {
		return nil, err
	}
	return products, rows.Err()
}

func appendCandidate(rows *sql.Rows, products []catalogProduct) ([]catalogProduct, error) {
	product, err := scanCandidate(rows)
	if err != nil {
		return nil, err
	}
	return append(products, product), nil
}

func scanCandidate(rows *sql.Rows) (catalogProduct, error) {
	var product catalogProduct
	var quantity, unit *string
	var count *int
	err := rows.Scan(&product.ID, &product.Name, &product.Brand, &product.GTIN, &quantity, &unit, &count)
	if err != nil {
		return product, err
	}
	if quantity != nil {
		product.Package = &PackageSize{Quantity: *quantity, Unit: *unit, Count: *count}
	}
	return product, nil
}

func (matcher productMatcher) enrich(selected catalogProduct, method string) (int64, string, error) {
	quantity, unit, count := matcher.source.packageValues()
	_, err := matcher.transaction.ExecContext(matcher.context, enrichProductSQL,
		selected.ID, matcher.source.Brand, matcher.source.gtinValue(), quantity, unit, count)
	return selected.ID, method, err
}

func (matcher productMatcher) create() (int64, string, error) {
	quantity, unit, count := matcher.source.packageValues()
	var identity int64
	row := matcher.transaction.QueryRowContext(matcher.context, createProductSQL,
		matcher.source.Name, matcher.source.Brand, matcher.source.gtinValue(), quantity, unit, count)
	err := row.Scan(&identity)
	return identity, "created_v1", err
}
