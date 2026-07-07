-- app stack5: client-encrypted key backup (opaque blob; docs/features/encryption.md).
CREATE TABLE IF NOT EXISTS e2e_key_backups (
    account_id UUID PRIMARY KEY,
    encrypted_blob TEXT NOT NULL,
    password_hint TEXT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
