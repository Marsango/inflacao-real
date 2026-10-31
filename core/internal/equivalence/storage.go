package equivalence

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
)

const productSQL = `SELECT jsonb_build_object('id',p.id,'name',p.name,'brand',p.brand,'barcode',p.gtin,
 'package',CASE WHEN p.package_quantity IS NOT NULL THEN jsonb_build_object('Quantity',p.package_quantity::text,'Unit',p.package_unit,'Count',p.package_count) END,
 'representative',equivalent_product_id(p.id),
 'bases',COALESCE((SELECT jsonb_agg(basis ORDER BY basis) FROM (SELECT DISTINCT price_basis AS basis FROM price_observations WHERE product_id=p.id AND price_basis IS NOT NULL) b),'[]'),
 'loose',p.package_quantity IS NULL AND NOT EXISTS(SELECT 1 FROM price_observations WHERE product_id=p.id AND package_quantity IS NOT NULL),
 'firstObserved',(SELECT to_char(min(observed_at) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM price_observations WHERE product_id=p.id),
 'lastObserved',(SELECT to_char(max(observed_at) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM price_observations WHERE product_id=p.id),
 'observations',(SELECT count(*) FROM price_observations WHERE product_id=p.id),
 'listings',COALESCE((SELECT jsonb_agg(jsonb_build_object('name',s.name,'brand',s.brand,'barcode',s.gtin,'url',s.product_url,'shop',s.shop_id) ORDER BY s.id) FROM shop_products s WHERE s.product_id=p.id),'[]'))
 FROM products p ORDER BY p.id`

func Load(requestContext context.Context, database queryer) ([]Product, error) {
	rows, err := database.QueryContext(requestContext, productSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := []Product{}
	for err == nil && rows.Next() {
		products, err = appendProductRow(rows, products)
	}
	if err != nil {
		return nil, err
	}
	return products, rows.Err()
}

func readProduct(rows *sql.Rows) (Product, error) {
	var data []byte
	var product Product
	if err := rows.Scan(&data); err != nil {
		return product, err
	}
	err := json.Unmarshal(data, &product)
	return product, err
}

func Read(requestContext context.Context, database *sql.DB) (Snapshot, error) {
	transaction, err := database.BeginTx(requestContext, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Snapshot{}, err
	}
	defer transaction.Rollback()
	result, err := snapshot(requestContext, transaction)
	if err != nil {
		return result, err
	}
	return result, transaction.Commit()
}

func snapshot(requestContext context.Context, database queryer) (Snapshot, error) {
	result := Snapshot{Candidates: []Candidate{}, Generic: []Candidate{}}
	err := database.QueryRowContext(requestContext, `SELECT revision,generic_enabled FROM catalog_decisions`).Scan(&result.Revision, &result.GenericEnabled)
	if err != nil {
		return result, err
	}
	result.Products, err = Load(requestContext, database)
	if err != nil {
		return result, err
	}
	exclusions, err := readExclusions(requestContext, database)
	if err != nil {
		return result, err
	}
	result.Candidates, result.Generic = discover(result.Products, exclusions)
	err = database.QueryRowContext(requestContext, `SELECT COALESCE(jsonb_agg(a ORDER BY id DESC),'[]') FROM (SELECT * FROM equivalence_audit ORDER BY id DESC LIMIT 100) a`).Scan(&result.Audit)
	return result, err
}

func readExclusions(requestContext context.Context, database queryer) (map[[2]int64]bool, error) {
	rows, err := database.QueryContext(requestContext, `SELECT equivalent_product_id(first_id),equivalent_product_id(second_id) FROM product_exclusions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[[2]int64]bool{}
	for err == nil && rows.Next() {
		err = readExclusionRow(rows, result)
	}
	if err != nil {
		return nil, err
	}
	return result, rows.Err()
}

func discover(products []Product, exclusions map[[2]int64]bool) ([]Candidate, []Candidate) {
	branded, generic := []Candidate{}, []Candidate{}
	for position, first := range products {
		candidates, automatic := discoverFor(first, products[position+1:], exclusions)
		branded, generic = append(branded, candidates...), append(generic, automatic...)
	}
	sort.SliceStable(branded, func(i, j int) bool {
		if branded[i].Kind != branded[j].Kind {
			return branded[i].Kind == "exact"
		}
		return branded[i].Score > branded[j].Score
	})
	return branded, generic
}

func discoverFor(first Product, others []Product, exclusions map[[2]int64]bool) ([]Candidate, []Candidate) {
	branded, generic := []Candidate{}, []Candidate{}
	for _, second := range others {
		candidate, kind := classifyPair(first, second, exclusions)
		generic = appendKind(generic, candidate, kind, "generic")
		branded = appendKind(branded, candidate, kind, "branded")
	}
	return branded, generic
}

func classifyPair(first, second Product, exclusions map[[2]int64]bool) (Candidate, string) {
	if first.Representative == second.Representative || excludedGroups(first, second, exclusions) {
		return Candidate{}, ""
	}
	key := CommodityKey(first)
	if key != "" && key == CommodityKey(second) {
		return Candidate{First: first.ID, Second: second.ID, Kind: "generic", Reason: key, Differences: []string{}}, "generic"
	}
	candidate, ok := Suggest(first, second)
	if ok {
		return candidate, "branded"
	}
	return candidate, ""
}

func excludedGroups(first, second Product, exclusions map[[2]int64]bool) bool {
	return exclusions[orderedPair(first.ID, second.ID)] || exclusions[orderedPair(first.Representative, second.Representative)]
}

func orderedPair(first, second int64) [2]int64 {
	return [2]int64{min(first, second), max(first, second)}
}

func appendProductRow(rows *sql.Rows, products []Product) ([]Product, error) {
	product, err := readProduct(rows)
	return append(products, product), err
}
func readExclusionRow(rows *sql.Rows, exclusions map[[2]int64]bool) error {
	var first, second int64
	if err := rows.Scan(&first, &second); err != nil {
		return err
	}
	exclusions[orderedPair(first, second)] = true
	return nil
}
func appendKind(candidates []Candidate, candidate Candidate, actual, expected string) []Candidate {
	if actual == expected {
		return append(candidates, candidate)
	}
	return candidates
}

func genericCandidates(products []Product, exclusions map[[2]int64]bool) []Candidate {
	buckets := map[string][]Product{}
	candidates := []Candidate{}
	for _, product := range products {
		candidates = append(candidates, bucketCandidates(buckets, product, exclusions)...)
	}
	return candidates
}
func bucketCandidates(buckets map[string][]Product, product Product, exclusions map[[2]int64]bool) []Candidate {
	key := CommodityKey(product)
	if key == "" {
		return nil
	}
	_, automatic := discoverFor(product, buckets[key], exclusions)
	buckets[key] = append(buckets[key], product)
	return automatic
}
