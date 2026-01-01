-- Add email verification support to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT FALSE;

-- Remove old unused columns (if they exist)
ALTER TABLE users DROP COLUMN IF EXISTS confirmation_code;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;

-- Create verification_codes table
CREATE TABLE IF NOT EXISTS verification_codes (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_verification_code_email ON verification_codes(email, code);
CREATE INDEX IF NOT EXISTS idx_verification_code_expires ON verification_codes(expires_at);

-- Update existing users to be verified (backward compatibility)
UPDATE users SET is_verified = TRUE WHERE is_verified IS NULL OR is_verified = FALSE;

