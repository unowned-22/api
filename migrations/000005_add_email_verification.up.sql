ALTER TABLE users
    ADD COLUMN email_verified_at TIMESTAMPTZ,
    ADD COLUMN verification_token TEXT,
    ADD COLUMN verification_token_expires_at TIMESTAMPTZ;
