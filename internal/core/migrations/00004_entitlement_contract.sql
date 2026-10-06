-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS max_devices integer,
    ADD COLUMN IF NOT EXISTS expiry_behavior varchar(30) NOT NULL DEFAULT 'freeze',
    ADD COLUMN IF NOT EXISTS grace_days integer NOT NULL DEFAULT 14;

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS chk_plans_expiry_behavior;
ALTER TABLE plans
    ADD CONSTRAINT chk_plans_expiry_behavior
    CHECK (expiry_behavior IN ('downgrade_to_free', 'freeze'));

CREATE TABLE IF NOT EXISTS organization_entitlement_overrides (
    id bigserial PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    organization_id bigint NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key varchar(100) NOT NULL,
    value jsonb NOT NULL,
    reason varchar(255) NOT NULL,
    expires_at timestamptz,
    created_by bigint NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_entitlement_overrides_org_key
    ON organization_entitlement_overrides (organization_id, key);
CREATE INDEX IF NOT EXISTS idx_entitlement_overrides_expires_at
    ON organization_entitlement_overrides (expires_at);

UPDATE plans
SET max_users = 1,
    max_collections = 10,
    max_items = 100,
    max_devices = NULL,
    expiry_behavior = 'downgrade_to_free',
    grace_days = 0,
    features = features || '{
      "sharing": false,
      "shared_items": false,
      "secure_send": false,
      "passkeys": false,
      "emergency_access": false,
      "teams": false,
      "audit": false,
      "sso": false,
      "api_access": false,
      "priority_support": false,
      "policies": false,
      "business_policies": false,
      "enterprise_policies": false,
      "security_insights": false,
      "breach_monitoring": false
    }'::jsonb
WHERE code = 'free-monthly';

UPDATE plans
SET max_users = 1,
    max_collections = NULL,
    max_items = NULL,
    max_devices = NULL,
    expiry_behavior = 'downgrade_to_free',
    grace_days = 14,
    features = features || '{
      "sharing": true,
      "shared_items": true,
      "secure_send": true,
      "passkeys": true,
      "emergency_access": true,
      "teams": false,
      "audit": false,
      "sso": false,
      "api_access": false,
      "priority_support": false,
      "policies": false,
      "business_policies": false,
      "enterprise_policies": false,
      "security_insights": true,
      "breach_monitoring": true
    }'::jsonb
WHERE code IN ('pro-monthly', 'pro-yearly');

UPDATE plans
SET max_users = 6,
    max_collections = NULL,
    max_items = NULL,
    max_devices = NULL,
    expiry_behavior = 'freeze',
    grace_days = 14,
    features = features || '{
      "sharing": true,
      "shared_items": true,
      "secure_send": true,
      "passkeys": true,
      "emergency_access": true,
      "teams": false,
      "audit": false,
      "sso": false,
      "api_access": false,
      "priority_support": false,
      "policies": false,
      "business_policies": false,
      "enterprise_policies": false,
      "security_insights": true,
      "breach_monitoring": true
    }'::jsonb
WHERE code IN ('family-monthly', 'family-yearly');

UPDATE plans
SET max_users = 10,
    max_collections = NULL,
    max_items = NULL,
    max_devices = NULL,
    expiry_behavior = 'freeze',
    grace_days = 14,
    features = features || '{
      "sharing": true,
      "shared_items": true,
      "secure_send": true,
      "passkeys": true,
      "emergency_access": true,
      "teams": true,
      "audit": false,
      "sso": false,
      "api_access": false,
      "priority_support": false,
      "policies": true,
      "business_policies": false,
      "enterprise_policies": false,
      "security_insights": true,
      "breach_monitoring": true
    }'::jsonb
WHERE code IN ('team-monthly', 'team-yearly');

UPDATE plans
SET max_users = NULL,
    max_collections = NULL,
    max_items = NULL,
    max_devices = NULL,
    expiry_behavior = 'freeze',
    grace_days = 14,
    features = features || '{
      "sharing": true,
      "shared_items": true,
      "secure_send": true,
      "passkeys": true,
      "emergency_access": true,
      "teams": true,
      "audit": true,
      "sso": true,
      "api_access": false,
      "priority_support": false,
      "policies": true,
      "business_policies": true,
      "enterprise_policies": false,
      "security_insights": true,
      "breach_monitoring": true
    }'::jsonb
WHERE code IN ('business-monthly', 'business-yearly');

UPDATE plans
SET max_users = NULL,
    max_collections = NULL,
    max_items = NULL,
    max_devices = NULL,
    expiry_behavior = 'freeze',
    grace_days = 14,
    features = features || '{
      "sharing": true,
      "shared_items": true,
      "secure_send": true,
      "passkeys": true,
      "emergency_access": true,
      "teams": true,
      "audit": true,
      "sso": true,
      "api_access": false,
      "priority_support": false,
      "policies": true,
      "business_policies": true,
      "enterprise_policies": true,
      "security_insights": true,
      "breach_monitoring": true
    }'::jsonb
WHERE code IN ('enterprise-monthly', 'enterprise-yearly');

UPDATE subscriptions AS subscription
SET grace_period_ends_at =
    subscription.updated_at +
    (plan.grace_days * interval '1 day')
FROM plans AS plan
WHERE subscription.plan_id = plan.id
  AND subscription.state = 'past_due'
  AND subscription.grace_period_ends_at IS NULL;

UPDATE subscriptions
SET state = 'expired',
    ended_at = COALESCE(ended_at, now()),
    updated_at = now()
WHERE (
        state = 'past_due'
        AND grace_period_ends_at <= now()
    )
    OR (
        state = 'canceled'
        AND (renew_at IS NULL OR renew_at <= now())
    )
    OR (
        state = 'trialing'
        AND stripe_subscription_id IS NULL
        AND trial_ends_at IS NOT NULL
        AND trial_ends_at <= now()
    );

INSERT INTO subscriptions (
    uuid,
    created_at,
    updated_at,
    organization_id,
    plan_id,
    state,
    started_at
)
SELECT
    (md5(random()::text || clock_timestamp()::text || o.id::text))::uuid,
    now(),
    now(),
    o.id,
    free_plan.id,
    'active',
    now()
FROM organizations o
JOIN plans free_plan ON free_plan.code = 'free-monthly'
WHERE o.is_personal = true
  AND EXISTS (
      SELECT 1
      FROM subscriptions expired_subscription
      JOIN plans expired_plan ON expired_plan.id = expired_subscription.plan_id
      WHERE expired_subscription.organization_id = o.id
        AND expired_subscription.state = 'expired'
        AND expired_plan.code IN ('pro-monthly', 'pro-yearly')
  )
  AND NOT EXISTS (
      SELECT 1
      FROM subscriptions effective_subscription
      WHERE effective_subscription.organization_id = o.id
        AND (
            effective_subscription.state IN ('active', 'trialing')
            OR (
                effective_subscription.state = 'past_due'
                AND effective_subscription.grace_period_ends_at > now()
            )
            OR (
                effective_subscription.state = 'canceled'
                AND effective_subscription.renew_at > now()
            )
        )
  );

-- +goose Down
DROP TABLE IF EXISTS organization_entitlement_overrides;
ALTER TABLE plans DROP CONSTRAINT IF EXISTS chk_plans_expiry_behavior;
ALTER TABLE plans
    DROP COLUMN IF EXISTS max_devices,
    DROP COLUMN IF EXISTS expiry_behavior,
    DROP COLUMN IF EXISTS grace_days;
