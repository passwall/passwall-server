-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Collection management settings (Bitwarden style). Defaults keep the
-- current behavior: owners/admins manage everything, members may create
-- collections, members with "manage" may delete them.
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS admins_manage_all_collections boolean NOT NULL DEFAULT true;
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS collection_creation_limited boolean NOT NULL DEFAULT false;
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS manage_can_delete_collections boolean NOT NULL DEFAULT true;

-- The manager role is replaced by the per-collection "manage" permission.
-- Managers had no collection access through their role, so they become
-- members; anything they could reach came from explicit grants, which stay.
UPDATE organization_users SET role = 'member' WHERE role = 'manager';
UPDATE organization_invitations SET role = 'member' WHERE role = 'manager';
UPDATE sso_connections SET default_role = 'member' WHERE default_role = 'manager';

-- +goose Down
-- Former managers are not restored; they stay members.
ALTER TABLE organizations DROP COLUMN IF EXISTS manage_can_delete_collections;
ALTER TABLE organizations DROP COLUMN IF EXISTS collection_creation_limited;
ALTER TABLE organizations DROP COLUMN IF EXISTS admins_manage_all_collections;
