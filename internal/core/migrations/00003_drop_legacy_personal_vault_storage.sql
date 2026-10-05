-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '15min';

CREATE TEMP TABLE legacy_personal_schemas (
    name text PRIMARY KEY
) ON COMMIT DROP;

-- Existing installations identify their old per-user schemas through users.schema.
-- Fresh installations do not have that column, so this intentionally remains empty.
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
        EXECUTE '
            INSERT INTO legacy_personal_schemas (name)
            SELECT DISTINCT schema
            FROM public.users
            WHERE schema IS NOT NULL
        ';
    END IF;
END
$$;
-- +goose StatementEnd

-- Refuse to cascade-drop anything that is not an explicitly recognized legacy
-- personal-vault schema and object shape.
-- +goose StatementBegin
DO $$
DECLARE
    schema_name text;
BEGIN
    IF EXISTS (
        SELECT 1
        FROM legacy_personal_schemas
        WHERE name !~ '^user_[0-9a-f]{8}$'
    ) THEN
        RAISE EXCEPTION 'unsafe legacy schema name detected';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM legacy_personal_schemas AS legacy
        LEFT JOIN pg_namespace AS namespace ON namespace.nspname = legacy.name
        WHERE namespace.oid IS NULL
    ) THEN
        RAISE EXCEPTION 'users.schema references a missing legacy schema';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_namespace AS namespace
        WHERE namespace.nspname ~ '^user_[0-9a-f]{8}$'
          AND NOT EXISTS (
              SELECT 1
              FROM legacy_personal_schemas AS legacy
              WHERE legacy.name = namespace.nspname
          )
    ) THEN
        RAISE EXCEPTION 'untracked legacy user schema detected';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_class AS object
        JOIN pg_namespace AS namespace ON namespace.oid = object.relnamespace
        JOIN legacy_personal_schemas AS legacy ON legacy.name = namespace.nspname
        WHERE NOT (
            (object.relkind = 'r' AND object.relname = 'items')
            OR (
                object.relkind = 'S'
                AND object.relname IN (
                    'items_id_seq',
                    'items_revision_seq',
                    'items_support_id_seq'
                )
            )
            OR (
                object.relkind = 'i'
                AND object.relname IN (
                    'items_pkey',
                    'idx_items_autofill',
                    'idx_items_deleted_at',
                    'idx_items_favorite',
                    'idx_items_metadata_gin',
                    'idx_items_revision',
                    'idx_items_support_id',
                    'idx_items_type',
                    'idx_items_uuid'
                )
            )
        )
    ) THEN
        RAISE EXCEPTION 'unexpected relation detected in a legacy user schema';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_proc AS routine
        JOIN pg_namespace AS namespace ON namespace.oid = routine.pronamespace
        JOIN legacy_personal_schemas AS legacy ON legacy.name = namespace.nspname
        WHERE routine.prokind <> 'f'
           OR routine.proname <> 'update_item_metadata'
           OR pg_get_function_identity_arguments(routine.oid) <> ''
    ) THEN
        RAISE EXCEPTION 'unexpected function detected in a legacy user schema';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_trigger AS trigger
        JOIN pg_class AS target_table ON target_table.oid = trigger.tgrelid
        JOIN pg_namespace AS target_namespace ON target_namespace.oid = target_table.relnamespace
        JOIN legacy_personal_schemas AS legacy ON legacy.name = target_namespace.nspname
        JOIN pg_proc AS routine ON routine.oid = trigger.tgfoid
        JOIN pg_namespace AS function_namespace ON function_namespace.oid = routine.pronamespace
        WHERE NOT trigger.tgisinternal
          AND (
              target_table.relname <> 'items'
              OR trigger.tgname <> 'item_metadata_trigger'
              OR function_namespace.nspname <> target_namespace.nspname
              OR routine.proname <> 'update_item_metadata'
          )
    ) THEN
        RAISE EXCEPTION 'unexpected trigger detected in a legacy user schema';
    END IF;

    IF to_regclass('public.items') IS NOT NULL AND EXISTS (
        SELECT 1
        FROM pg_depend
        WHERE refclassid = 'pg_class'::regclass
          AND refobjid = to_regclass('public.items')
          AND deptype NOT IN ('a', 'i')
    ) THEN
        RAISE EXCEPTION 'public.items has an external dependency';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_attribute AS attribute
        JOIN pg_depend AS dependency
          ON dependency.refclassid = 'pg_class'::regclass
         AND dependency.refobjid = attribute.attrelid
         AND dependency.refobjsubid = attribute.attnum
        WHERE attribute.attrelid = 'public.users'::regclass
          AND attribute.attname = 'schema'
          AND NOT attribute.attisdropped
          AND dependency.deptype NOT IN ('a', 'i')
    ) THEN
        RAISE EXCEPTION 'users.schema has an external dependency';
    END IF;

    FOR schema_name IN
        SELECT name
        FROM legacy_personal_schemas
        ORDER BY name
    LOOP
        EXECUTE format('DROP SCHEMA %I CASCADE', schema_name);
    END LOOP;
END
$$;
-- +goose StatementEnd

DROP TABLE IF EXISTS public.items CASCADE;
ALTER TABLE public.users DROP COLUMN IF EXISTS schema;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_namespace
        WHERE nspname ~ '^user_[0-9a-f]{8}$'
    ) THEN
        RAISE EXCEPTION 'legacy user schemas remain after cleanup';
    END IF;
    IF to_regclass('public.items') IS NOT NULL THEN
        RAISE EXCEPTION 'public.items remains after cleanup';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'schema'
    ) THEN
        RAISE EXCEPTION 'users.schema remains after cleanup';
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Production migrations are forward-only.
SELECT 1;
