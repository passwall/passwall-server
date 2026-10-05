-- +goose Up
-- Existing installations are baselined at their current GORM-managed schema.
SELECT 1;

-- +goose Down
-- Production migrations are forward-only. This no-op exists for goose syntax.
SELECT 1;
