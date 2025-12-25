-- Update erhan@passwall.io to admin role
-- Run this SQL script in your PostgreSQL database

UPDATE users 
SET role = 'admin' 
WHERE email = 'erhan@passwall.io';

-- Verify the update
SELECT id, email, role, name 
FROM users 
WHERE email = 'erhan@passwall.io';

