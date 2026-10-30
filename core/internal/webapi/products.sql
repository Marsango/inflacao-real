SELECT id::text, name || coalesce(' · ' || brand, '')
FROM products
WHERE id > $1
  AND EXISTS (SELECT 1 FROM price_observations WHERE product_id = products.id)
  AND search_document @@ to_tsquery('simple',
      nullif(array_to_string(tsvector_to_array(
          to_tsvector('simple', product_search_text($3))), ':* & '), '') || ':*')
ORDER BY id
LIMIT $2
