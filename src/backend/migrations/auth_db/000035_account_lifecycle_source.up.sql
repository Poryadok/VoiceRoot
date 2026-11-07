BEGIN;
-- Auth-owned opaque lifecycle history. No raw account/profile/provider IDs.
-- Terminal restore fencing is distinct from a physical-erasure receipt.
SET LOCAL lock_timeout='2s';
SET LOCAL statement_timeout='5s';
LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE;

CREATE TABLE public.auth_account_lifecycle_schedules (
  owner_handle UUID NOT NULL REFERENCES public.role_identity_lifetimes(owner_handle),
  deletion_cycle_id UUID NOT NULL UNIQUE,
  protocol_version SMALLINT NOT NULL DEFAULT 1 CHECK (protocol_version=1),
  schedule_revision BIGINT NOT NULL CHECK (schedule_revision>0),
  scheduled_at TIMESTAMPTZ NOT NULL,
  purge_at TIMESTAMPTZ NOT NULL,
  intent_id UUID NOT NULL UNIQUE,
  PRIMARY KEY(owner_handle,deletion_cycle_id),
  UNIQUE(owner_handle,deletion_cycle_id,schedule_revision),
  UNIQUE(owner_handle,deletion_cycle_id,schedule_revision,scheduled_at,purge_at),
  CHECK(isfinite(scheduled_at) AND isfinite(purge_at)),
  CHECK(purge_at=scheduled_at+INTERVAL '720 hours')
);

CREATE TABLE public.auth_account_lifecycle_decisions (
  owner_handle UUID NOT NULL,
  deletion_cycle_id UUID NOT NULL,
  schedule_revision BIGINT NOT NULL,
  scheduled_at TIMESTAMPTZ NOT NULL,
  purge_at TIMESTAMPTZ NOT NULL,
  decision TEXT NOT NULL CHECK(decision IN ('RESTORED','LIFETIME_TERMINAL')),
  decision_revision BIGINT NOT NULL CHECK(decision_revision>schedule_revision),
  decision_at TIMESTAMPTZ NOT NULL,
  intent_id UUID NOT NULL UNIQUE,
  PRIMARY KEY(owner_handle,deletion_cycle_id),
  FOREIGN KEY(owner_handle,deletion_cycle_id,schedule_revision,scheduled_at,purge_at)
    REFERENCES public.auth_account_lifecycle_schedules(owner_handle,deletion_cycle_id,schedule_revision,scheduled_at,purge_at),
  CHECK(isfinite(decision_at) AND decision_at>=scheduled_at),
  CHECK((decision='RESTORED' AND decision_at<purge_at) OR
        (decision='LIFETIME_TERMINAL' AND decision_at>=purge_at))
);

CREATE TABLE public.auth_account_lifecycle_intents (
  event_id UUID PRIMARY KEY,
  protocol_version SMALLINT NOT NULL DEFAULT 1 CHECK(protocol_version=1),
  kind TEXT NOT NULL CHECK(kind IN ('DELETE_SCHEDULED','RESTORED','LIFETIME_TERMINAL')),
  owner_handle UUID NOT NULL,
  deletion_cycle_id UUID NOT NULL,
  lifecycle_revision BIGINT NOT NULL CHECK(lifecycle_revision>0),
  event_bytes BYTEA NOT NULL CHECK(octet_length(event_bytes)>0 AND octet_length(event_bytes)<=16384),
  event_sha256 BYTEA NOT NULL CHECK(octet_length(event_sha256)=32 AND event_sha256=sha256(event_bytes)),
  created_at TIMESTAMPTZ NOT NULL,
  available_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts>=0),
  last_error_code TEXT CHECK(last_error_code IS NULL OR last_error_code ~ '^[A-Z0-9_]{1,64}$'),
  lease_token UUID,
  lease_until TIMESTAMPTZ,
  pubacked_at TIMESTAMPTZ,
  UNIQUE(owner_handle,deletion_cycle_id,kind),
  FOREIGN KEY(owner_handle,deletion_cycle_id)
    REFERENCES public.auth_account_lifecycle_schedules(owner_handle,deletion_cycle_id),
  CHECK((lease_token IS NULL)=(lease_until IS NULL))
);
ALTER TABLE public.auth_account_lifecycle_schedules ADD CONSTRAINT auth_lifecycle_schedule_intent
  FOREIGN KEY(intent_id) REFERENCES public.auth_account_lifecycle_intents(event_id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE public.auth_account_lifecycle_decisions ADD CONSTRAINT auth_lifecycle_decision_intent
  FOREIGN KEY(intent_id) REFERENCES public.auth_account_lifecycle_intents(event_id) DEFERRABLE INITIALLY DEFERRED;
CREATE INDEX auth_account_lifecycle_pending ON public.auth_account_lifecycle_intents(available_at,created_at)
  WHERE pubacked_at IS NULL;

CREATE FUNCTION public.auth_account_lifecycle_fact_immutable() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
BEGIN
  RAISE EXCEPTION 'Auth cycle facts are immutable' USING ERRCODE='55000';
END $$;
CREATE TRIGGER auth_lifecycle_schedule_immutable BEFORE UPDATE OR DELETE ON public.auth_account_lifecycle_schedules
  FOR EACH ROW EXECUTE FUNCTION public.auth_account_lifecycle_fact_immutable();
CREATE TRIGGER auth_lifecycle_decision_immutable BEFORE UPDATE OR DELETE ON public.auth_account_lifecycle_decisions
  FOR EACH ROW EXECUTE FUNCTION public.auth_account_lifecycle_fact_immutable();
CREATE TRIGGER auth_lifecycle_schedule_no_truncate BEFORE TRUNCATE ON public.auth_account_lifecycle_schedules
  FOR EACH STATEMENT EXECUTE FUNCTION public.auth_account_lifecycle_fact_immutable();
CREATE TRIGGER auth_lifecycle_decision_no_truncate BEFORE TRUNCATE ON public.auth_account_lifecycle_decisions
  FOR EACH STATEMENT EXECUTE FUNCTION public.auth_account_lifecycle_fact_immutable();

CREATE FUNCTION public.auth_account_lifecycle_intent_immutable() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
BEGIN
  IF TG_OP IN ('DELETE','TRUNCATE') OR
    (to_jsonb(NEW)-ARRAY['available_at','attempts','last_error_code','lease_token','lease_until','pubacked_at'])
    IS DISTINCT FROM
    (to_jsonb(OLD)-ARRAY['available_at','attempts','last_error_code','lease_token','lease_until','pubacked_at']) THEN
    RAISE EXCEPTION 'Auth cycle intent identity and bytes are immutable' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER auth_lifecycle_intent_immutable BEFORE UPDATE OR DELETE ON public.auth_account_lifecycle_intents
  FOR EACH ROW EXECUTE FUNCTION public.auth_account_lifecycle_intent_immutable();
CREATE TRIGGER auth_lifecycle_intent_no_truncate BEFORE TRUNCATE ON public.auth_account_lifecycle_intents
  FOR EACH STATEMENT EXECUTE FUNCTION public.auth_account_lifecycle_intent_immutable();

-- Owning transition capture. These are private source intents, not a published
-- transport schema or proof of consumer receipt/currentness/physical erasure.
CREATE FUNCTION public.auth_account_lifecycle_emit(p_event uuid,p_kind text,p_owner uuid,
  p_cycle uuid,p_revision bigint,p_scheduled timestamptz,p_purge timestamptz,p_decided timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE payload bytea;
BEGIN
  payload := convert_to(jsonb_build_object('version',1,'event_id',p_event::text,
    'kind',p_kind,'owner_handle',p_owner::text,'deletion_cycle_id',p_cycle::text,
    'lifecycle_revision',p_revision::text,
    'scheduled_at',to_char(p_scheduled AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'purge_at',to_char(p_purge AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'decision_at',CASE WHEN p_decided IS NOT NULL THEN
      to_char(p_decided AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') END)::text,'UTF8');
  INSERT INTO public.auth_account_lifecycle_intents
    (event_id,kind,owner_handle,deletion_cycle_id,lifecycle_revision,event_bytes,event_sha256,created_at)
  VALUES(p_event,p_kind,p_owner,p_cycle,p_revision,payload,sha256(payload),COALESCE(p_decided,p_scheduled));
END $$;

CREATE FUNCTION public.auth_account_lifecycle_schedule(p_owner uuid,p_scheduled timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE f public.role_identity_lifetimes%ROWTYPE; event uuid := gen_random_uuid();
BEGIN
  SELECT * INTO STRICT f FROM public.role_identity_lifetimes WHERE owner_handle=p_owner;
  INSERT INTO public.auth_account_lifecycle_schedules
    (owner_handle,deletion_cycle_id,schedule_revision,scheduled_at,purge_at,intent_id)
  VALUES(p_owner,f.deletion_cycle,f.lifecycle_revision,p_scheduled,f.erase_at,event);
  PERFORM public.auth_account_lifecycle_emit(event,'DELETE_SCHEDULED',p_owner,f.deletion_cycle,
    f.lifecycle_revision,p_scheduled,f.erase_at,NULL);
END $$;

CREATE FUNCTION public.auth_account_lifecycle_decide(p_owner uuid,p_cycle uuid,p_kind text,
  p_revision bigint,p_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE s public.auth_account_lifecycle_schedules%ROWTYPE; event uuid := gen_random_uuid();
BEGIN
  SELECT * INTO s FROM public.auth_account_lifecycle_schedules
    WHERE owner_handle=p_owner AND deletion_cycle_id=p_cycle;
  -- No fabricated original schedule for a pre-capture lifetime/cycle.
  IF NOT FOUND THEN RETURN; END IF;
  INSERT INTO public.auth_account_lifecycle_decisions
    (owner_handle,deletion_cycle_id,schedule_revision,scheduled_at,purge_at,decision,decision_revision,decision_at,intent_id)
  VALUES(p_owner,p_cycle,s.schedule_revision,s.scheduled_at,s.purge_at,p_kind,p_revision,p_at,event);
  PERFORM public.auth_account_lifecycle_emit(event,p_kind,p_owner,p_cycle,p_revision,s.scheduled_at,s.purge_at,p_at);
END $$;

CREATE OR REPLACE FUNCTION role_identity_materialize_handle(p_handle uuid) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE f role_identity_lifetimes%ROWTYPE; cut timestamptz;
BEGIN
  -- The same account -> lifetime -> journal order is used by all domain writers.
  PERFORM 1 FROM accounts WHERE role_identity_handle=p_handle FOR UPDATE;
  SELECT * INTO STRICT f FROM role_identity_lifetimes WHERE owner_handle=p_handle FOR UPDATE;
  cut := clock_timestamp();
  IF f.state='RECOVERABLE' AND cut>=f.erase_at THEN
    UPDATE role_identity_lifetimes SET state='TERMINAL',eligible=false,
      lifecycle_revision=lifecycle_revision+1 WHERE owner_handle=p_handle;
    PERFORM role_identity_append('TERMINAL',p_handle);
    PERFORM public.auth_account_lifecycle_decide(p_handle,f.deletion_cycle,'LIFETIME_TERMINAL',f.lifecycle_revision+1,cut);
    RETURN f.lifecycle_revision+1;
  END IF;
  RETURN f.lifecycle_revision;
END $$;

CREATE OR REPLACE FUNCTION auth_role_identity_account_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE f role_identity_lifetimes%ROWTYPE; s role_identity_owner_state%ROWTYPE; changed boolean := false; cut timestamptz;
BEGIN
  -- UPDATE/DELETE already hold the exact account row lock. A transaction-start
  -- clock must never authorize an expired restore after waiting on that lock.
  SELECT * INTO STRICT f FROM role_identity_lifetimes WHERE owner_handle=OLD.role_identity_handle FOR UPDATE;
  cut := clock_timestamp();
  IF TG_OP='DELETE' THEN
    IF f.state<>'TERMINAL' AND (OLD.deleted_at IS NULL OR cut<OLD.deleted_at+interval '30 days')
      THEN RAISE EXCEPTION 'account erasure boundary not reached'; END IF;
    PERFORM role_identity_materialize_handle(OLD.role_identity_handle);
    IF f.state<>'TERMINAL' AND f.erase_at IS NULL THEN RAISE EXCEPTION 'account erasure boundary unknown'; END IF;
    PERFORM role_identity_append('RAW_ERASURE',OLD.role_identity_handle);
    RETURN OLD;
  END IF;
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.role_identity_handle IS DISTINCT FROM OLD.role_identity_handle
    THEN RAISE EXCEPTION 'identity lifetime is immutable'; END IF;
  IF f.state='TERMINAL' THEN
    IF NEW.deleted_at IS NULL OR NEW.status<>'deleted' THEN RAISE EXCEPTION 'terminal identity cannot be restored'; END IF;
    IF NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN RAISE EXCEPTION 'identity deletion cycle is immutable'; END IF;
  ELSIF OLD.deleted_at IS NOT NULL THEN
    IF cut>=OLD.deleted_at+interval '30 days' THEN
      IF NEW.deleted_at IS NULL OR NEW.status<>'deleted' THEN RAISE EXCEPTION 'terminal identity cannot be restored'; END IF;
    END IF;
    IF NEW.deleted_at IS NOT NULL AND NEW.deleted_at IS DISTINCT FROM OLD.deleted_at
      THEN RAISE EXCEPTION 'identity deletion cycle is immutable'; END IF;
  END IF;
  IF NEW.deleted_at IS NOT NULL THEN NEW.status := 'deleted';
  ELSIF NEW.status='deleted' THEN RAISE EXCEPTION 'identity deletion boundary missing'; END IF;
  IF NEW.role_identity_selector_version IS DISTINCT FROM OLD.role_identity_selector_version
      OR NEW.role_identity_account_selector IS DISTINCT FROM OLD.role_identity_account_selector
      OR NEW.role_identity_keyset_revision IS DISTINCT FROM OLD.role_identity_keyset_revision THEN
    SELECT * INTO STRICT s FROM role_identity_owner_state FOR UPDATE;
    IF NEW.role_identity_selector_version IS NULL OR NEW.role_identity_account_selector IS NULL
        OR NEW.role_identity_keyset_revision IS NULL OR NEW.role_identity_selector_version IS DISTINCT FROM s.active_version
        OR NEW.role_identity_keyset_revision<>s.keyset_revision THEN RAISE EXCEPTION 'identity selector keyset unavailable'; END IF;
    INSERT INTO role_identity_account_selectors VALUES(NEW.role_identity_selector_version,
      NEW.role_identity_account_selector,NEW.role_identity_handle) ON CONFLICT DO NOTHING;
    IF NOT EXISTS(SELECT 1 FROM role_identity_account_selectors WHERE key_version=NEW.role_identity_selector_version
        AND selector=NEW.role_identity_account_selector AND owner_handle=NEW.role_identity_handle)
      THEN RAISE EXCEPTION 'identity selector binding conflict'; END IF;
    UPDATE role_identity_lifetimes SET selector_complete=true WHERE owner_handle=f.owner_handle;
    changed := true;
  END IF;
  IF NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
    UPDATE role_identity_lifetimes SET lifecycle_revision=lifecycle_revision+1,
      state=CASE WHEN NEW.deleted_at IS NULL THEN 'ACTIVE'
                 ELSE 'RECOVERABLE' END,
      eligible=NEW.status='active' AND NEW.deleted_at IS NULL,
      deletion_cycle=CASE WHEN NEW.deleted_at IS NOT NULL THEN gen_random_uuid() END,
      erase_at=NEW.deleted_at+interval '30 days' WHERE owner_handle=f.owner_handle;
    IF NEW.deleted_at IS NOT NULL THEN
      PERFORM public.auth_account_lifecycle_schedule(f.owner_handle,NEW.deleted_at);
    ELSE
      PERFORM public.auth_account_lifecycle_decide(f.owner_handle,f.deletion_cycle,'RESTORED',f.lifecycle_revision+1,cut);
    END IF;
    changed := true;
  ELSIF NEW.status IS DISTINCT FROM OLD.status OR NEW.type IS DISTINCT FROM OLD.type THEN
    UPDATE role_identity_lifetimes SET lifecycle_revision=lifecycle_revision+1,
      eligible=NEW.status='active' AND NEW.deleted_at IS NULL WHERE owner_handle=f.owner_handle;
    changed := true;
  END IF;
  IF changed THEN PERFORM role_identity_append('MUTATION',NEW.role_identity_handle); END IF;
  PERFORM role_identity_materialize_handle(NEW.role_identity_handle);
  RETURN NEW;
END $$;
COMMIT;
