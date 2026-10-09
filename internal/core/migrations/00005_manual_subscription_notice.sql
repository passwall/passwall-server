-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS manual_end_notice_sent_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_subscriptions_manual_end_notice
    ON subscriptions (renew_at)
    WHERE (stripe_subscription_id IS NULL OR BTRIM(stripe_subscription_id) = '')
      AND manual_end_notice_sent_at IS NULL
      AND state = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_subscriptions_manual_end_notice;

ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS manual_end_notice_sent_at;
