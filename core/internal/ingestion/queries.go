package ingestion

const queueMetricsSQL = `SELECT
    count(*) FILTER (WHERE status='pending'),
    COALESCE(EXTRACT(EPOCH FROM now()-min(received_at) FILTER (WHERE status='pending')),0)::float8,
    count(*) FILTER (WHERE status='failed'),
    COALESCE(sum(GREATEST(attempts-1,0)),0)::bigint FROM ingestion_payloads`
const nextPayloadSQL = `SELECT id,attempts,payload,scraper_id,submission_id,shop_id,schema_version
    FROM ingestion_payloads WHERE status='pending' AND next_attempt_at<=now()
    ORDER BY received_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`
const processedPayloadSQL = `UPDATE ingestion_payloads SET status='processed',
    processed_at=clock_timestamp(),attempts=attempts+1,last_error=NULL WHERE id=$1`
const failedPayloadSQL = `UPDATE ingestion_payloads SET
    attempts=attempts+1,last_error=$3,
    next_attempt_at=clock_timestamp()+$4::bigint*interval '1 microsecond',
    status=CASE WHEN $5 THEN 'failed' ELSE 'pending' END,
    failed_at=CASE WHEN $5 THEN clock_timestamp() ELSE NULL END
    WHERE id=$1 AND status='pending' AND attempts=$2`

const registeredShopSQL = `SELECT EXISTS (SELECT 1 FROM shops WHERE id=$1)`

const catalogLockSQL = `SELECT pg_advisory_xact_lock(694120031)`

const reserveObservationSQL = `INSERT INTO observation_identities
			(scraper_id, observation_id, fingerprint, payload_id, run_id, collector_name, collector_version)
			VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`

const existingFingerprintSQL = `SELECT fingerprint FROM observation_identities
				WHERE scraper_id=$1 AND observation_id=$2`

const upsertListingSQL = `INSERT INTO shop_products
			(shop_id,source_product_id,name,brand,gtin,product_url,metadata_observed_at,product_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7::timestamptz,$8)
			ON CONFLICT (shop_id,source_product_id) DO UPDATE SET
			name=EXCLUDED.name,brand=EXCLUDED.brand,gtin=EXCLUDED.gtin,
			product_url=EXCLUDED.product_url,metadata_observed_at=EXCLUDED.metadata_observed_at,
			product_id=EXCLUDED.product_id
			WHERE shop_products.metadata_observed_at IS NULL
				OR shop_products.metadata_observed_at<=EXCLUDED.metadata_observed_at
			RETURNING id`

const existingListingSQL = `SELECT id FROM shop_products WHERE shop_id=$1 AND source_product_id=$2`

const insertOfferSQL = `INSERT INTO price_observations (
			payload_id,shop_product_id,scraper_id,observation_id,observed_at,source_url,
			collection_method,package_quantity,package_unit,package_count,seller_id,store_id,
			postal_code,city,state,channel,availability,offer_count,offer_index,
			price_amount,price_currency,price_basis,reference_amount,payment_method,installments,
			membership_required,membership_program,coupon_required,minimum_quantity,condition_description,
			product_id,product_match_method
		) VALUES ($1,$2,$3,$4,$5::timestamptz,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,
			$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32)`
