-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Invitation v2: organization invitations get their own table and become the
-- single source of truth. The legacy `invitations` table keeps only referral
-- ("invite a friend") rows, and organization_users no longer mirrors pending
-- invitations with status 'invited'.

CREATE TABLE IF NOT EXISTS organization_invitations (
    id bigserial PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    organization_id bigint NOT NULL,
    email varchar(255) NOT NULL,
    role varchar(20) NOT NULL,
    access_all boolean NOT NULL DEFAULT false,
    status varchar(16) NOT NULL DEFAULT 'pending',
    encrypted_org_key text,
    invited_by_user_id bigint NOT NULL,
    expires_at timestamptz NOT NULL,
    last_sent_at timestamptz,
    send_count integer NOT NULL DEFAULT 0,
    responded_at timestamptz,
    accepted_user_id bigint,
    revoked_by_user_id bigint
);

ALTER TABLE organization_invitations
    DROP CONSTRAINT IF EXISTS fk_organization_invitations_organization;
ALTER TABLE organization_invitations
    ADD CONSTRAINT fk_organization_invitations_organization
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE organization_invitations
    DROP CONSTRAINT IF EXISTS chk_organization_invitations_status;
ALTER TABLE organization_invitations
    ADD CONSTRAINT chk_organization_invitations_status
    CHECK (status IN ('pending', 'accepted', 'declined', 'revoked', 'expired'));

CREATE INDEX IF NOT EXISTS idx_organization_invitations_organization_id
    ON organization_invitations (organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_invitations_pending_email
    ON organization_invitations (lower(email))
    WHERE status = 'pending';

-- Migrate legacy organization invitations. When several pending rows exist for
-- the same organization and email, only the newest stays pending.
WITH legacy AS (
    SELECT
        i.*,
        CASE
            WHEN i.used_at IS NOT NULL THEN 'accepted'
            WHEN i.expires_at <= now() THEN 'expired'
            ELSE 'pending'
        END AS new_status,
        row_number() OVER (
            PARTITION BY i.organization_id, lower(i.email),
                (i.used_at IS NULL AND i.expires_at > now())
            ORDER BY i.created_at DESC, i.id DESC
        ) AS rank_in_scope
    FROM invitations i
    WHERE i.organization_id IS NOT NULL
      AND EXISTS (SELECT 1 FROM organizations o WHERE o.id = i.organization_id)
)
INSERT INTO organization_invitations (
    created_at, updated_at, organization_id, email, role, access_all, status,
    encrypted_org_key, invited_by_user_id, expires_at, last_sent_at, send_count,
    responded_at
)
SELECT
    created_at,
    now(),
    organization_id,
    lower(trim(email)),
    COALESCE(NULLIF(org_role, ''), 'member'),
    COALESCE(access_all, false),
    CASE WHEN new_status = 'pending' AND rank_in_scope > 1 THEN 'revoked' ELSE new_status END,
    NULLIF(encrypted_org_key, ''),
    created_by,
    expires_at,
    created_at,
    1,
    used_at
FROM legacy;

CREATE UNIQUE INDEX IF NOT EXISTS ux_organization_invitations_pending
    ON organization_invitations (organization_id, lower(email))
    WHERE status = 'pending';

-- Remove the pending-membership mirror. A still-pending invitation now lives
-- in organization_invitations; rows without one are stale and only held a seat.
CREATE TEMP TABLE invited_org_users ON COMMIT DROP AS
SELECT id FROM organization_users WHERE status = 'invited';

DELETE FROM team_users WHERE organization_user_id IN (SELECT id FROM invited_org_users);
DELETE FROM collection_users WHERE organization_user_id IN (SELECT id FROM invited_org_users);
DELETE FROM organization_users WHERE id IN (SELECT id FROM invited_org_users);

DELETE FROM invitations WHERE organization_id IS NOT NULL;

-- +goose Down
-- Organization invitations are not copied back; this only removes the new table.
DROP TABLE IF EXISTS organization_invitations;
