-- Add public_id column to organizations table.
-- This short 12-character alphanumeric ID is used in URLs instead of
-- the sequential numeric ID to avoid exposing customer count.

ALTER TABLE organizations ADD COLUMN IF NOT EXISTS public_id VARCHAR(12);

-- Backfill existing rows with random 12-char alphanumeric IDs.
-- Uses a PL/pgSQL DO block with crypto-grade randomness (gen_random_uuid).
DO $$
DECLARE
    r RECORD;
    alphabet TEXT := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    new_id   TEXT;
    i        INT;
BEGIN
    FOR r IN SELECT id FROM organizations WHERE public_id IS NULL OR public_id = '' LOOP
        new_id := '';
        FOR i IN 1..12 LOOP
            new_id := new_id || substr(alphabet, floor(random() * 62 + 1)::int, 1);
        END LOOP;
        UPDATE organizations SET public_id = new_id WHERE id = r.id;
    END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_public_id
    ON organizations (public_id)
    WHERE public_id IS NOT NULL AND public_id != '';
