-- Create Roles and Permissions Tables and Seed Initial Data
-- Run this after server starts (GORM will create tables automatically)

-- Insert Roles
INSERT INTO roles (id, name, display_name, description, created_at, updated_at) 
VALUES 
  (1, 'admin', 'Administrator', 'Full system access with all permissions', NOW(), NOW()),
  (2, 'member', 'Member', 'Standard user with limited access', NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- Insert Permissions
INSERT INTO permissions (name, display_name, description, resource, action, created_at, updated_at) 
VALUES 
  -- Users permissions
  ('users.read', 'View Users', 'Can view user list and details', 'users', 'read', NOW(), NOW()),
  ('users.create', 'Create Users', 'Can create new users', 'users', 'create', NOW(), NOW()),
  ('users.update', 'Update Users', 'Can update user information', 'users', 'update', NOW(), NOW()),
  ('users.delete', 'Delete Users', 'Can delete users', 'users', 'delete', NOW(), NOW()),
  
  -- Logins permissions
  ('logins.read', 'View Logins', 'Can view login credentials', 'logins', 'read', NOW(), NOW()),
  ('logins.create', 'Create Logins', 'Can create new login credentials', 'logins', 'create', NOW(), NOW()),
  ('logins.update', 'Update Logins', 'Can update login credentials', 'logins', 'update', NOW(), NOW()),
  ('logins.delete', 'Delete Logins', 'Can delete login credentials', 'logins', 'delete', NOW(), NOW()),
  
  -- Credit Cards permissions
  ('credit_cards.read', 'View Credit Cards', 'Can view credit cards', 'credit_cards', 'read', NOW(), NOW()),
  ('credit_cards.create', 'Create Credit Cards', 'Can create credit cards', 'credit_cards', 'create', NOW(), NOW()),
  ('credit_cards.update', 'Update Credit Cards', 'Can update credit cards', 'credit_cards', 'update', NOW(), NOW()),
  ('credit_cards.delete', 'Delete Credit Cards', 'Can delete credit cards', 'credit_cards', 'delete', NOW(), NOW()),
  
  -- Bank Accounts permissions
  ('bank_accounts.read', 'View Bank Accounts', 'Can view bank accounts', 'bank_accounts', 'read', NOW(), NOW()),
  ('bank_accounts.create', 'Create Bank Accounts', 'Can create bank accounts', 'bank_accounts', 'create', NOW(), NOW()),
  ('bank_accounts.update', 'Update Bank Accounts', 'Can update bank accounts', 'bank_accounts', 'update', NOW(), NOW()),
  ('bank_accounts.delete', 'Delete Bank Accounts', 'Can delete bank accounts', 'bank_accounts', 'delete', NOW(), NOW()),
  
  -- Notes permissions
  ('notes.read', 'View Notes', 'Can view notes', 'notes', 'read', NOW(), NOW()),
  ('notes.create', 'Create Notes', 'Can create notes', 'notes', 'create', NOW(), NOW()),
  ('notes.update', 'Update Notes', 'Can update notes', 'notes', 'update', NOW(), NOW()),
  ('notes.delete', 'Delete Notes', 'Can delete notes', 'notes', 'delete', NOW(), NOW()),
  
  -- Emails permissions
  ('emails.read', 'View Emails', 'Can view emails', 'emails', 'read', NOW(), NOW()),
  ('emails.create', 'Create Emails', 'Can create emails', 'emails', 'create', NOW(), NOW()),
  ('emails.update', 'Update Emails', 'Can update emails', 'emails', 'update', NOW(), NOW()),
  ('emails.delete', 'Delete Emails', 'Can delete emails', 'emails', 'delete', NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- Assign ALL permissions to Admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT 1, id FROM permissions
ON CONFLICT DO NOTHING;

-- Assign limited permissions to Member role (own data only, no user management)
INSERT INTO role_permissions (role_id, permission_id)
SELECT 2, id FROM permissions 
WHERE resource IN ('logins', 'credit_cards', 'bank_accounts', 'notes', 'emails')
ON CONFLICT DO NOTHING;

-- Update existing users to use role_id
UPDATE users SET role_id = 2 WHERE role_id IS NULL OR role_id = 0;

-- Set erhan@passwall.io as admin
UPDATE users SET role_id = 1 WHERE email = 'erhan@passwall.io';

-- Verify
SELECT u.id, u.email, u.name, u.role_id, r.name as role_name, r.display_name
FROM users u
LEFT JOIN roles r ON u.role_id = r.id
WHERE u.email = 'erhan@passwall.io';

