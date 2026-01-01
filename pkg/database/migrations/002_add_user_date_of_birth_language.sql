-- Add date_of_birth and language columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS date_of_birth TIMESTAMP NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS language VARCHAR(10) NOT NULL DEFAULT 'en';

-- Create index on language for better query performance
CREATE INDEX IF NOT EXISTS idx_users_language ON users(language);

-- Update existing users to have default language 'en' if NULL
UPDATE users SET language = 'en' WHERE language IS NULL OR language = '';

