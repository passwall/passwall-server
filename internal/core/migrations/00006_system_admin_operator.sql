-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '1min';

-- Manual subscription grant/extend/end requires role=admin AND is_system_user,
-- verified against the database. Mark the Passwall operator account before the
-- new middleware serves traffic so it is not locked out. Only an existing admin
-- is touched; on other installations this is a no-op.
UPDATE users
SET is_system_user = true,
    updated_at = now()
WHERE LOWER(email) = 'erhan@passwall.io'
  AND role_id = 1
  AND is_system_user = false;

-- +goose Down
-- Intentionally a no-op: the flag may have been set before this migration ran,
-- and clearing it would also re-enable deletion of the account.
SELECT 1;
