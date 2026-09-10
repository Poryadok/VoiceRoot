-- Hash-only step-up proofs and durable one-operation consume receipts.
ALTER TABLE accounts ADD COLUMN security_revision BIGINT NOT NULL DEFAULT 1
    CHECK (security_revision > 0);

-- A monotonic revision prevents a credential change-and-revert from reviving a proof.
CREATE FUNCTION auth_advance_security_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.password_hash, NEW.totp_secret, NEW.totp_enabled, NEW.session_epoch,
           NEW.status, NEW.deleted_at, NEW.type, NEW.email, NEW.phone,
           NEW.regular_email_verification_pending)
       IS DISTINCT FROM
       ROW(OLD.password_hash, OLD.totp_secret, OLD.totp_enabled, OLD.session_epoch,
           OLD.status, OLD.deleted_at, OLD.type, OLD.email, OLD.phone,
           OLD.regular_email_verification_pending) THEN
        NEW.security_revision := GREATEST(OLD.security_revision + 1, NEW.security_revision);
    ELSE
        NEW.security_revision := GREATEST(OLD.security_revision, NEW.security_revision);
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER accounts_security_revision BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION auth_advance_security_revision();

-- Backup factors are account security state too. Account updates share the same lock
-- as proof issuance/consume, and factor consumption rolls back with failed issuance.
CREATE FUNCTION auth_backup_security_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE accounts SET security_revision = security_revision + 1 WHERE id = OLD.account_id;
        RETURN OLD;
    END IF;
    IF TG_OP = 'INSERT' THEN
        UPDATE accounts SET security_revision = security_revision + 1 WHERE id = NEW.account_id;
    ELSIF ROW(NEW.account_id, NEW.code_hash, NEW.used_at)
          IS DISTINCT FROM ROW(OLD.account_id, OLD.code_hash, OLD.used_at) THEN
        UPDATE accounts SET security_revision = security_revision + 1
            WHERE id = NEW.account_id OR id = OLD.account_id;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER backup_codes_security_revision AFTER INSERT OR UPDATE OR DELETE ON backup_codes
    FOR EACH ROW EXECUTE FUNCTION auth_backup_security_revision();

CREATE TABLE ownership_transfer_proofs (
    operation_id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    space_id UUID NOT NULL,
    new_owner_profile_id UUID NOT NULL,
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    proof_hash CHAR(64) NOT NULL UNIQUE CHECK (proof_hash ~ '^[0-9a-f]{64}$'),
    security_revision BIGINT NOT NULL CHECK (security_revision > 0),
    verified_factors TEXT NOT NULL CHECK (verified_factors IN ('password', 'password,totp', 'password,backup_code')),
    expires_at TIMESTAMPTZ NOT NULL,
    receipt_id UUID NOT NULL UNIQUE,
    consumed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ownership_transfer_proofs_account_idx ON ownership_transfer_proofs(account_id);
