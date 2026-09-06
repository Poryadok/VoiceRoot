-- Existing accounts retain their current authority. Only fresh email registrations use this marker.
ALTER TABLE accounts
    ADD COLUMN regular_email_verification_pending BOOLEAN NOT NULL DEFAULT false;
