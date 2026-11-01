-- +goose Up
-- Existing products and listings are assigned to monitoring targets on demand
-- through the planned category assignment operation. Keep this migration fast
-- so deploying the schema does not scan the full catalog during startup.

-- +goose Down
-- No data changes are made by this migration.
SELECT 1;
