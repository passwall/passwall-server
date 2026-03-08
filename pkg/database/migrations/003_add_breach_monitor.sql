-- Breach Monitoring: monitored emails and breach records
-- This migration adds tables for dark web / breach monitoring (HIBP integration).

CREATE TABLE IF NOT EXISTS monitored_emails (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT       NOT NULL,
    email           VARCHAR(320) NOT NULL,
    last_checked_at TIMESTAMPTZ,
    breach_count    INT          NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_monitored_emails_org_id ON monitored_emails (organization_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_monitored_emails_org_email ON monitored_emails (organization_id, email);

CREATE TABLE IF NOT EXISTS breach_records (
    id                  BIGSERIAL PRIMARY KEY,
    monitored_email_id  BIGINT       NOT NULL REFERENCES monitored_emails(id) ON DELETE CASCADE,
    breach_name         VARCHAR(255) NOT NULL,
    breach_domain       VARCHAR(255) NOT NULL DEFAULT '',
    breach_date         VARCHAR(10)  NOT NULL DEFAULT '',
    added_date          VARCHAR(30)  NOT NULL DEFAULT '',
    data_classes        JSONB        NOT NULL DEFAULT '[]',
    description         TEXT         NOT NULL DEFAULT '',
    logo_path           VARCHAR(512) NOT NULL DEFAULT '',
    pwn_count           INT          NOT NULL DEFAULT 0,
    is_verified         BOOLEAN      NOT NULL DEFAULT FALSE,
    is_sensitive        BOOLEAN      NOT NULL DEFAULT FALSE,
    is_dismissed        BOOLEAN      NOT NULL DEFAULT FALSE,
    discovered_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_breach_records_email_id ON breach_records (monitored_email_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_breach_records_email_breach ON breach_records (monitored_email_id, breach_name);
