-- +goose NO TRANSACTION
-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '15min';

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 0;

-- Expand step: new server versions no longer write the legacy schema column.
-- The physical column is retained until the post-deploy contract migration.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'schema'
    ) THEN
        ALTER TABLE users ALTER COLUMN schema DROP NOT NULL;
    END IF;
END
$$;
-- +goose StatementEnd

CREATE SEQUENCE IF NOT EXISTS organization_item_support_id_seq AS bigint;
ALTER TABLE organization_items
    ALTER COLUMN support_id SET DEFAULT nextval('organization_item_support_id_seq');
ALTER SEQUENCE organization_item_support_id_seq
    OWNED BY organization_items.support_id;

WITH ranked AS (
    SELECT
        source.id,
        (SELECT COALESCE(max(support_id), 0) FROM organization_items WHERE support_id <> 0)
            + row_number() OVER (ORDER BY source.created_at, source.id) AS support_id
    FROM organization_items AS source
    WHERE source.support_id = 0
)
UPDATE organization_items AS item
SET support_id = ranked.support_id
FROM ranked
WHERE item.id = ranked.id;

SELECT setval(
    'organization_item_support_id_seq',
    GREATEST(COALESCE((SELECT max(support_id) FROM organization_items), 0), 1),
    EXISTS (SELECT 1 FROM organization_items)
);

WITH ranked AS (
    SELECT
        source.id,
        (
            SELECT COALESCE(max(existing.revision), 0)
            FROM organization_items AS existing
            WHERE existing.organization_id = source.organization_id
              AND existing.revision <> 0
        ) + row_number() OVER (
            PARTITION BY source.organization_id
            ORDER BY source.created_at, source.id
        ) AS revision
    FROM organization_items AS source
    WHERE source.revision = 0
)
UPDATE organization_items AS item
SET revision = COALESCE(ranked.revision, 0)
FROM ranked
WHERE item.id = ranked.id;

UPDATE organizations AS organization
SET revision = revisions.max_revision
FROM (
    SELECT organization_id, max(revision) AS max_revision
    FROM organization_items
    GROUP BY organization_id
) AS revisions
WHERE organization.id = revisions.organization_id
  AND organization.revision < revisions.max_revision;

INSERT INTO collections (
    uuid,
    created_at,
    updated_at,
    organization_id,
    name,
    description,
    is_default,
    is_private
)
SELECT
    md5(random()::text || clock_timestamp()::text || organization.id::text)::uuid,
    now(),
    now(),
    organization.id,
    'General',
    'System default collection',
    true,
    false
FROM organizations AS organization
WHERE organization.deleted_at IS NULL
  AND organization.is_active = true
  AND NOT EXISTS (
      SELECT 1
      FROM collections
      WHERE collections.organization_id = organization.id
        AND collections.is_default = true
        AND collections.deleted_at IS NULL
  );

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM organization_users
        GROUP BY organization_id, user_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate organization membership prevents invariant migration';
    END IF;
    IF EXISTS (
        SELECT 1 FROM organization_items GROUP BY uuid HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate organization item UUID prevents invariant migration';
    END IF;
    IF EXISTS (
        SELECT 1 FROM organization_items GROUP BY support_id HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate organization item support_id prevents invariant migration';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM collections
        WHERE is_default = true AND deleted_at IS NULL
        GROUP BY organization_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'multiple default collections prevent invariant migration';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM subscriptions
        WHERE state IN ('active', 'trialing', 'past_due')
        GROUP BY organization_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'multiple active subscriptions prevent invariant migration';
    END IF;
END
$$;
-- +goose StatementEnd

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_organization_users_org_user
    ON organization_users (organization_id, user_id);
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_organization_items_uuid
    ON organization_items (uuid);
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_organization_items_support_id
    ON organization_items (support_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_organization_items_org_revision_id
    ON organization_items (organization_id, revision, id);
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_collections_one_default_per_org
    ON collections (organization_id)
    WHERE is_default = true AND deleted_at IS NULL;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_subscriptions_one_active_per_org
    ON subscriptions (organization_id)
    WHERE state IN ('active', 'trialing', 'past_due');
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_organizations_personal_owner
    ON organizations (personal_owner_user_id)
    WHERE is_personal = true AND deleted_at IS NULL;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_collections_id_organization
    ON collections (id, organization_id);
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ux_organization_folders_id_organization
    ON organization_folders (id, organization_id);

ALTER TABLE organization_items
    ALTER COLUMN created_by_user_id DROP NOT NULL;
ALTER TABLE organization_items
    DROP CONSTRAINT IF EXISTS fk_organization_items_created_by;
ALTER TABLE organization_items
    ADD CONSTRAINT fk_organization_items_created_by
    FOREIGN KEY (created_by_user_id)
    REFERENCES users (id)
    ON DELETE SET NULL
    NOT VALID;
ALTER TABLE organization_items
    VALIDATE CONSTRAINT fk_organization_items_created_by;

ALTER TABLE organization_items
    DROP CONSTRAINT IF EXISTS fk_organization_items_collection;
ALTER TABLE organization_items
    ADD CONSTRAINT fk_organization_items_collection_organization
    FOREIGN KEY (collection_id, organization_id)
    REFERENCES collections (id, organization_id)
    ON DELETE RESTRICT
    NOT VALID;
ALTER TABLE organization_items
    VALIDATE CONSTRAINT fk_organization_items_collection_organization;

ALTER TABLE organization_items
    DROP CONSTRAINT IF EXISTS fk_organization_items_folder;
ALTER TABLE organization_items
    ADD CONSTRAINT fk_organization_items_folder_organization
    FOREIGN KEY (folder_id, organization_id)
    REFERENCES organization_folders (id, organization_id)
    ON DELETE RESTRICT
    NOT VALID;
ALTER TABLE organization_items
    VALIDATE CONSTRAINT fk_organization_items_folder_organization;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS fk_users_personal_organization;
ALTER TABLE users
    ADD CONSTRAINT fk_users_personal_organization
    FOREIGN KEY (personal_organization_id)
    REFERENCES organizations (id)
    ON DELETE RESTRICT
    NOT VALID;
ALTER TABLE users
    VALIDATE CONSTRAINT fk_users_personal_organization;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS fk_users_default_organization;
ALTER TABLE users
    ADD CONSTRAINT fk_users_default_organization
    FOREIGN KEY (default_organization_id)
    REFERENCES organizations (id)
    ON DELETE RESTRICT
    NOT VALID;
ALTER TABLE users
    VALIDATE CONSTRAINT fk_users_default_organization;

-- +goose Down
SELECT 1;
