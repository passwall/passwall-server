-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Domain ownership proof (DNS TXT). Existing connections start unverified
-- and inactive: an owner verifies the domain, then activates again. Setting
-- them inactive also keeps a rollback to an older server from reopening them.
ALTER TABLE sso_connections
    ADD COLUMN IF NOT EXISTS domain_verification_token varchar(64);
ALTER TABLE sso_connections
    ADD COLUMN IF NOT EXISTS domain_verified_at timestamptz;
UPDATE sso_connections
SET domain_verification_token = md5(random()::text || clock_timestamp()::text || id::text)
WHERE domain_verification_token IS NULL OR domain_verification_token = '';
UPDATE sso_connections SET status = 'inactive'
WHERE status = 'active' AND domain_verified_at IS NULL;

-- A domain is unique only among verified connections, so an unverified claim
-- cannot block the organization that really owns it.
DROP INDEX IF EXISTS idx_sso_connections_domain;
CREATE INDEX IF NOT EXISTS idx_sso_connections_domain ON sso_connections (domain);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sso_connections_verified_domain
    ON sso_connections (domain) WHERE domain_verified_at IS NOT NULL;

-- GORM default:true turned every "off" into "on". Provisioning is opt-in:
-- reset it so admins re-enable it deliberately after verifying the domain.
ALTER TABLE sso_connections ALTER COLUMN auto_provision SET DEFAULT false;
ALTER TABLE sso_connections ALTER COLUMN jit_provisioning SET DEFAULT false;
UPDATE sso_connections SET auto_provision = false, jit_provisioning = false;

-- Only the member role may be granted automatically.
UPDATE sso_connections SET default_role = 'member' WHERE default_role IS DISTINCT FROM 'member';

-- Key escrow is removed from the code. Its column and (empty) tables stay
-- until a later migration so a rollback to the previous image still works.

-- Browser binding for the login code (PKCE-style).
ALTER TABLE sso_states
    ADD COLUMN IF NOT EXISTS client_code_challenge varchar(128);
-- SAML: the AuthnRequest ID the assertion must answer (InResponseTo).
ALTER TABLE sso_states
    ADD COLUMN IF NOT EXISTS saml_request_id varchar(128);
DELETE FROM sso_states WHERE expires_at < NOW();

CREATE TABLE IF NOT EXISTS sso_login_codes (
    id bigserial PRIMARY KEY,
    created_at timestamptz,
    code_hash varchar(64) NOT NULL,
    user_id bigint NOT NULL,
    connection_id bigint NOT NULL,
    organization_id bigint NOT NULL,
    client_code_challenge varchar(128) NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sso_login_codes_code_hash ON sso_login_codes (code_hash);
CREATE INDEX IF NOT EXISTS idx_sso_login_codes_connection_id ON sso_login_codes (connection_id);
CREATE INDEX IF NOT EXISTS idx_sso_login_codes_expires_at ON sso_login_codes (expires_at);
-- Same FK whether the table came from here or from a fresh-database bootstrap.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_sso_login_codes_connection') THEN
        ALTER TABLE sso_login_codes
            ADD CONSTRAINT fk_sso_login_codes_connection
            FOREIGN KEY (connection_id) REFERENCES sso_connections (id) ON DELETE CASCADE;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS sso_login_codes;
ALTER TABLE sso_states DROP COLUMN IF EXISTS saml_request_id;
ALTER TABLE sso_states DROP COLUMN IF EXISTS client_code_challenge;
DROP INDEX IF EXISTS idx_sso_connections_verified_domain;
-- Not unique again: unverified rows may share a domain now.
DROP INDEX IF EXISTS idx_sso_connections_domain;
CREATE INDEX IF NOT EXISTS idx_sso_connections_domain ON sso_connections (domain);
ALTER TABLE sso_connections DROP COLUMN IF EXISTS domain_verified_at;
ALTER TABLE sso_connections DROP COLUMN IF EXISTS domain_verification_token;
