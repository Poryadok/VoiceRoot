BEGIN;
-- Auth-owned raw-lifetime facts, not Subscription entitlement/CURRENT grants.
-- Capture the actual coordinator operation and conditional restore transaction.
-- Purge/replay-HMAC and protected dispatch remain required before qualification.
SET LOCAL lock_timeout='2s';
SET LOCAL statement_timeout='5s';
LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE;
CREATE TABLE public.account_subscription_lifecycle_heads (
    account_id UUID PRIMARY KEY REFERENCES public.accounts(id) ON DELETE CASCADE,
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    event_id UUID NOT NULL UNIQUE,
    deletion_cycle_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('DELETE_SCHEDULED','RESTORE_REQUESTED')),
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    scheduled_at TIMESTAMPTZ NOT NULL,
    purge_at TIMESTAMPTZ NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    CHECK (purge_at = scheduled_at + INTERVAL '720 hours')
);

CREATE TABLE public.account_subscription_lifecycle_outbox (
    event_id UUID PRIMARY KEY,
    protocol_version INTEGER NOT NULL DEFAULT 1 CHECK (protocol_version=1),
    account_id UUID NOT NULL REFERENCES public.accounts(id) ON DELETE CASCADE,
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    deletion_cycle_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('DELETE_SCHEDULED','RESTORE_REQUESTED')),
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    scheduled_at TIMESTAMPTZ NOT NULL,
    purge_at TIMESTAMPTZ NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    UNIQUE (account_id,source_revision),
    UNIQUE (account_id,deletion_cycle_id,kind),
    CHECK (purge_at = scheduled_at + INTERVAL '720 hours'),
    CHECK ((lease_token IS NULL) = (lease_until IS NULL))
);
CREATE INDEX account_subscription_lifecycle_pending ON public.account_subscription_lifecycle_outbox(available_at,captured_at)
    WHERE delivered_at IS NULL;

CREATE FUNCTION public.auth_capture_subscription_deletion() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE
    target public.accounts%ROWTYPE;
    fact public.account_subscription_lifecycle_heads%ROWTYPE;
    captured TIMESTAMPTZ := clock_timestamp();
BEGIN
    SELECT * INTO STRICT target FROM public.accounts WHERE id=NEW.account_id FOR UPDATE;
    IF target.status <> 'deleted' OR target.deleted_at IS NULL OR target.session_epoch < NEW.session_epoch THEN
        RAISE EXCEPTION 'account deletion fact requires matching deleted owner state' USING ERRCODE='23514';
    END IF;
    INSERT INTO public.account_subscription_lifecycle_heads
        (account_id,source_revision,event_id,deletion_cycle_id,kind,session_epoch,scheduled_at,purge_at,effective_at,captured_at)
    VALUES
        (target.id,1,NEW.operation_id,NEW.operation_id,'DELETE_SCHEDULED',NEW.session_epoch,
         target.deleted_at,target.deleted_at+INTERVAL '720 hours',target.deleted_at,captured)
    ON CONFLICT (account_id) DO UPDATE SET
        source_revision=public.account_subscription_lifecycle_heads.source_revision+1,
        event_id=EXCLUDED.event_id,deletion_cycle_id=EXCLUDED.deletion_cycle_id,kind=EXCLUDED.kind,
        session_epoch=EXCLUDED.session_epoch,scheduled_at=EXCLUDED.scheduled_at,purge_at=EXCLUDED.purge_at,
        effective_at=EXCLUDED.effective_at,captured_at=EXCLUDED.captured_at
    RETURNING * INTO fact;
    INSERT INTO public.account_subscription_lifecycle_outbox
        (event_id,account_id,source_revision,deletion_cycle_id,kind,session_epoch,scheduled_at,purge_at,effective_at,captured_at)
    VALUES
        (fact.event_id,fact.account_id,fact.source_revision,fact.deletion_cycle_id,fact.kind,fact.session_epoch,
         fact.scheduled_at,fact.purge_at,fact.effective_at,fact.captured_at);
    RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION public.auth_capture_subscription_deletion() FROM PUBLIC;
CREATE TRIGGER auth_subscription_deletion_capture AFTER INSERT ON public.account_deletion_operations
    FOR EACH ROW EXECUTE FUNCTION public.auth_capture_subscription_deletion();

CREATE FUNCTION public.auth_capture_subscription_restore() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE
    fact public.account_subscription_lifecycle_heads%ROWTYPE;
    captured TIMESTAMPTZ := clock_timestamp();
BEGIN
    -- Legacy restore remains a functioning free path. A missing proven cycle
    -- never fabricates a new paid-authority fact or guessed history.
    SELECT * INTO fact FROM public.account_subscription_lifecycle_heads
      WHERE account_id=NEW.id AND kind='DELETE_SCHEDULED' AND scheduled_at=OLD.deleted_at FOR UPDATE;
    IF NOT FOUND THEN RETURN NEW; END IF;
    UPDATE public.account_subscription_lifecycle_heads SET
        source_revision=source_revision+1,event_id=gen_random_uuid(),kind='RESTORE_REQUESTED',
        session_epoch=NEW.session_epoch,effective_at=captured,captured_at=captured
      WHERE account_id=NEW.id RETURNING * INTO fact;
    INSERT INTO public.account_subscription_lifecycle_outbox
        (event_id,account_id,source_revision,deletion_cycle_id,kind,session_epoch,scheduled_at,purge_at,effective_at,captured_at)
    VALUES
        (fact.event_id,fact.account_id,fact.source_revision,fact.deletion_cycle_id,fact.kind,fact.session_epoch,
         fact.scheduled_at,fact.purge_at,fact.effective_at,fact.captured_at);
    RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION public.auth_capture_subscription_restore() FROM PUBLIC;
CREATE TRIGGER auth_subscription_restore_capture AFTER UPDATE OF status,deleted_at ON public.accounts
    FOR EACH ROW WHEN (OLD.status='deleted' AND OLD.deleted_at IS NOT NULL AND NEW.status='active' AND NEW.deleted_at IS NULL)
    EXECUTE FUNCTION public.auth_capture_subscription_restore();

CREATE FUNCTION public.auth_subscription_fact_immutable() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
BEGIN
    IF (to_jsonb(NEW)-ARRAY['available_at','attempts','last_error','lease_token','lease_until','delivered_at'])
       IS DISTINCT FROM
       (to_jsonb(OLD)-ARRAY['available_at','attempts','last_error','lease_token','lease_until','delivered_at']) THEN
        RAISE EXCEPTION 'subscription lifecycle fact is immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION public.auth_subscription_fact_immutable() FROM PUBLIC;
CREATE TRIGGER auth_subscription_fact_immutable BEFORE UPDATE ON public.account_subscription_lifecycle_outbox
    FOR EACH ROW EXECUTE FUNCTION public.auth_subscription_fact_immutable();
COMMIT;
