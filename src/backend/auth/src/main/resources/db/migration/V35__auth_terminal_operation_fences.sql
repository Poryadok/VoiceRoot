-- Permanent identity-free terminal outcomes. The cleaner inserts fences and
-- deletes raw receipts in one transaction; no fence references an erasable row.
SET LOCAL lock_timeout='2s';
SET LOCAL statement_timeout='2s';
LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE;

CREATE TABLE public.auth_terminal_operation_fences (
    family TEXT NOT NULL CHECK (family IN ('sdk_authorization', 'sdk_binding',
        'sdk_conversion', 'sdk_execution', 'sdk_registration', 'ownership_transfer')),
    operation_id UUID NOT NULL,
    owner_handle UUID NOT NULL,
    deletion_cycle_id UUID NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (family, operation_id)
);

-- The hash is only an ephemeral lock key over an opaque operation identity; it
-- is never stored as a personal-data digest. Collisions only serialize writers.
-- Acquire owning SDK identity/account locks first, then operation keys in sorted
-- family/UUID order. After fencing, erase by those keys, including a raw writer
-- that committed before the cleaner obtained the operation lock.
CREATE FUNCTION public.auth_terminal_operation_lock(operation_family TEXT, operation_key UUID)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY INVOKER
SET search_path=pg_catalog,public AS $$
BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='terminal_operation_requires_read_committed';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(
        'auth-terminal-operation:' || operation_family || ':' || operation_key::text, 0));
END;
$$;

CREATE FUNCTION public.auth_terminal_fence_insert_guard()
RETURNS trigger LANGUAGE plpgsql VOLATILE SECURITY INVOKER
SET search_path=pg_catalog,public AS $$
BEGIN
    PERFORM public.auth_terminal_operation_lock(NEW.family, NEW.operation_id);
    RETURN NEW;
END;
$$;
CREATE TRIGGER auth_terminal_fence_insert BEFORE INSERT ON public.auth_terminal_operation_fences
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_fence_insert_guard();

CREATE FUNCTION public.auth_terminal_fence_immutable()
RETURNS trigger LANGUAGE plpgsql VOLATILE SECURITY INVOKER
SET search_path=pg_catalog,public AS $$
BEGIN
    RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='terminal_operation_fence_immutable';
END;
$$;
CREATE TRIGGER auth_terminal_fence_immutable BEFORE UPDATE OR DELETE OR TRUNCATE
    ON public.auth_terminal_operation_fences
    FOR EACH STATEMENT EXECUTE FUNCTION public.auth_terminal_fence_immutable();

-- Fixed trigger arguments bind every persisted operation alias to its family.
-- Guard both old and new keys: UPDATE must not move a fenced receipt elsewhere.
-- At READ COMMITTED the post-lock query sees a fence committed during the wait.
CREATE FUNCTION public.auth_terminal_raw_operation_guard()
RETURNS trigger LANGUAGE plpgsql VOLATILE SECURITY INVOKER
SET search_path=pg_catalog,public AS $$
DECLARE
    previous_row JSONB;
    operation RECORD;
BEGIN
    IF TG_OP='UPDATE' THEN previous_row := to_jsonb(OLD); END IF;
    FOR operation IN
        SELECT DISTINCT TG_ARGV[indexes.n] AS family,
            (state.body ->> TG_ARGV[indexes.n+1])::uuid AS operation_id
        FROM generate_series(0,TG_NARGS-2,2) AS indexes(n)
        CROSS JOIN (VALUES (to_jsonb(NEW)), (previous_row)) AS state(body)
        WHERE state.body ->> TG_ARGV[indexes.n+1] IS NOT NULL
        ORDER BY family,operation_id
    LOOP
        PERFORM public.auth_terminal_operation_lock(operation.family,operation.operation_id);
        IF EXISTS (SELECT 1 FROM public.auth_terminal_operation_fences f
                   WHERE f.family=operation.family AND f.operation_id=operation.operation_id) THEN
            RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='terminal_operation_erased';
        END IF;
    END LOOP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.ownership_transfer_proofs
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('ownership_transfer','operation_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_authorizations
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_authorization','request_id','sdk_binding','game_binding_operation_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_linked_sessions
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_authorization','request_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_game_binding_handoff_issuances
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_authorization','authorization_request_id','sdk_binding','operation_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_game_binding_handoff_claims
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_authorization','authorization_request_id','sdk_binding','operation_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_game_message_grants
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_authorization','authorization_request_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_conversion_operations
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_conversion','operation_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_conversion_owner_receipts
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_conversion','operation_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_registration_intents
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_conversion','operation_id','sdk_registration','intent_id');
CREATE TRIGGER auth_terminal_operation_guard BEFORE INSERT OR UPDATE ON public.sdk_game_message_execution_permits
    FOR EACH ROW EXECUTE FUNCTION public.auth_terminal_raw_operation_guard('sdk_execution','operation_id');
