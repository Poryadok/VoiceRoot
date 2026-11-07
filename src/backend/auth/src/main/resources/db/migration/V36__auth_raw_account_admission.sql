-- Raw credential ownership is immutable. Serialize new rows with account
-- erasure, including writers that observed an active account before deletion.
SET LOCAL lock_timeout='2s';
SET LOCAL statement_timeout='5s';
LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE;

CREATE FUNCTION public.auth_raw_account_admission()
RETURNS trigger LANGUAGE plpgsql VOLATILE SECURITY INVOKER
SET search_path=pg_catalog,public AS $$
DECLARE
    owner public.accounts%ROWTYPE;
    cut TIMESTAMPTZ;
BEGIN
    IF TG_OP='UPDATE' THEN
        IF NEW.account_id IS DISTINCT FROM OLD.account_id THEN
            RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='raw_account_owner_immutable';
        END IF;
        -- Existing row updates must not introduce a row -> account lock edge.
        RETURN NEW;
    END IF;
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='raw_account_requires_read_committed';
    END IF;
    SELECT * INTO owner FROM public.accounts WHERE id=NEW.account_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='raw_account_unavailable';
    END IF;
    cut := clock_timestamp();
    -- Every lifecycle writer takes this account lock before its lifetime lock.
    -- Reading the lifetime after the wait therefore needs no additional lock.
    IF (owner.deleted_at IS NOT NULL AND cut>=owner.deleted_at+INTERVAL '30 days')
        OR NOT EXISTS (SELECT 1 FROM public.role_identity_lifetimes f
            WHERE f.owner_handle=owner.role_identity_handle AND f.state<>'TERMINAL'
              AND (f.erase_at IS NULL OR cut<f.erase_at)) THEN
        RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='raw_account_unavailable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.otp_codes
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.backup_codes
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.linked_identities
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.e2e_key_backups
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.guest_conversion_operations
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.verification_source_sync_targets
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.linked_oauth_attempts
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
CREATE TRIGGER auth_raw_account_admission BEFORE INSERT OR UPDATE ON public.account_deletion_operations
    FOR EACH ROW EXECUTE FUNCTION public.auth_raw_account_admission();
