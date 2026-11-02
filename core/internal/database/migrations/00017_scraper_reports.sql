-- +goose Up
CREATE TABLE scraper_runs (
 scraper_id text NOT NULL, shop_id text NOT NULL REFERENCES shops(id), run_id uuid NOT NULL,
 scraper_name text NOT NULL, started_at timestamptz NOT NULL, finished_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(), observations integer NOT NULL,
 errors integer NOT NULL, warnings integer NOT NULL, omitted integer NOT NULL,
 PRIMARY KEY(scraper_id,shop_id,run_id)
);
CREATE INDEX scraper_runs_latest ON scraper_runs(scraper_id,shop_id,started_at DESC,run_id DESC);
CREATE TABLE scraper_diagnostics (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 scraper_id text NOT NULL, shop_id text NOT NULL, run_id uuid NOT NULL,
 severity text NOT NULL CHECK(severity IN ('error','warning')),
 category text NOT NULL, message text NOT NULL,
 FOREIGN KEY(scraper_id,shop_id,run_id) REFERENCES scraper_runs(scraper_id,shop_id,run_id)
);
CREATE INDEX scraper_diagnostics_lookup ON scraper_diagnostics(scraper_id,shop_id,id DESC);
-- +goose Down
DROP TABLE scraper_diagnostics;
DROP TABLE scraper_runs;
