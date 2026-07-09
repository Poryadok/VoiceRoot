-- privacy/trust (docs/features/privacy.md): TOTP backup codes — aligned with src/backend/migrations/auth_db/000003_backup_codes.up.sql
CREATE TABLE backup_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    code_hash CHAR(64) NOT NULL,
    used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX backup_codes_account_active_idx ON backup_codes (account_id)
    WHERE used_at IS NULL;
