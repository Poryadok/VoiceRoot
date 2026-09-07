-- Existing regular accounts predate email verification enforcement and retain their authority.
-- Only newly inserted email registrations set this marker and begin as guest until OTP verification.
ALTER TABLE accounts
    ADD COLUMN regular_email_verification_pending BOOLEAN NOT NULL DEFAULT false;
