-- Additive account-ban coverage after the trusted, sealed native45 cut.
-- Bot46-49 are not rewritten, installed, asserted or qualified by this file.
BEGIN;
LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.epoch WHERE singleton
  AND epoch='94db7e08-e958-44fa-94b2-988410ab0044'
  AND catalog_sha256=native_membership_guard.catalog()) THEN
  RAISE EXCEPTION 'account coverage requires an already trusted native45 seal' USING ERRCODE='55000';
 END IF;
 IF to_regprocedure('public.guard_join_legacy_selection_immutable()') IS NULL THEN
  RAISE EXCEPTION 'native45 source required' USING ERRCODE='55000';
 END IF;
END $$;

CREATE SCHEMA space_account_ban_guard;
REVOKE ALL ON SCHEMA space_account_ban_guard FROM PUBLIC;

CREATE TABLE space_account_ban_operations (
 actor_profile_id UUID NOT NULL CHECK(actor_profile_id<>'00000000-0000-0000-0000-000000000000'),
 client_operation_id UUID NOT NULL CHECK(client_operation_id<>'00000000-0000-0000-0000-000000000000'),
 actor_account_id UUID NOT NULL CHECK(actor_account_id<>'00000000-0000-0000-0000-000000000000'),
 operation_kind TEXT NOT NULL CHECK(operation_kind IN ('BAN','UNBAN')),
 space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 body_sha256 BYTEA NOT NULL CHECK(octet_length(body_sha256)=32),
 state TEXT NOT NULL CHECK(state IN ('CAPTURING','COMMITTED')),
 selected_capture_id UUID,
 selected_attempt_seq BIGINT NOT NULL DEFAULT 0 CHECK(selected_attempt_seq>=0),
 execution_id UUID CHECK(execution_id<>'00000000-0000-0000-0000-000000000000'),
 execution_expires_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 completed_at TIMESTAMPTZ,
 response_bytes BYTEA,
 matched_profiles UUID[],
 PRIMARY KEY(actor_profile_id,client_operation_id),
 UNIQUE(actor_profile_id,client_operation_id,space_id),
 CHECK((execution_id IS NULL)=(execution_expires_at IS NULL)),
 CHECK((selected_capture_id IS NULL)=(selected_attempt_seq=0)),
 CHECK(operation_kind='BAN' OR selected_capture_id IS NULL),
 CHECK((state='COMMITTED')=(completed_at IS NOT NULL)),
 CHECK((state='COMMITTED')=(response_bytes IS NOT NULL AND matched_profiles IS NOT NULL)),
 CHECK(state<>'CAPTURING' OR (response_bytes IS NULL AND matched_profiles IS NULL)),
 CHECK(state<>'COMMITTED' OR operation_kind='UNBAN' OR selected_capture_id IS NOT NULL)
);
-- Per-Space cleanup/P3 and the global completed-outcome hint scan have
-- independent access paths; CAPTURING is never an age-based cleanup target.
CREATE INDEX space_account_ban_operations_space_expiry
 ON space_account_ban_operations(space_id,completed_at,actor_profile_id,client_operation_id);
CREATE INDEX space_account_ban_operations_completed_expiry
 ON space_account_ban_operations(completed_at,space_id) WHERE state='COMMITTED';

-- Public intent is stable; each selected owner request is an immutable child.
-- Cohort changes append a new child, never rewrite a previously dispatched one.
CREATE TABLE space_account_ban_captures (
 capture_id UUID PRIMARY KEY CHECK(capture_id<>'00000000-0000-0000-0000-000000000000'),
 actor_profile_id UUID NOT NULL,
 client_operation_id UUID NOT NULL,
 space_id UUID NOT NULL,
 attempt_seq BIGINT NOT NULL CHECK(attempt_seq>0),
 cohort_bytes BYTEA NOT NULL,
 cohort_digest BYTEA NOT NULL CHECK(octet_length(cohort_digest)=32),
 source_revision BIGINT NOT NULL CHECK(source_revision>0),
 owner_profile_id UUID NOT NULL CHECK(owner_profile_id<>'00000000-0000-0000-0000-000000000000'),
 UNIQUE(actor_profile_id,client_operation_id,space_id,capture_id,attempt_seq),
 UNIQUE(capture_id,space_id),
 UNIQUE(actor_profile_id,client_operation_id,attempt_seq),
 FOREIGN KEY(actor_profile_id,client_operation_id,space_id)
 REFERENCES space_account_ban_operations(actor_profile_id,client_operation_id,space_id) ON DELETE CASCADE
);
ALTER TABLE space_account_ban_operations ADD CONSTRAINT account_ban_selected_child
 FOREIGN KEY(actor_profile_id,client_operation_id,space_id,selected_capture_id,selected_attempt_seq)
 REFERENCES space_account_ban_captures(actor_profile_id,client_operation_id,space_id,capture_id,attempt_seq)
 DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE space_account_ban_reconciliation (
 capture_id UUID NOT NULL,
 space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 execution_id UUID NOT NULL CHECK(execution_id<>'00000000-0000-0000-0000-000000000000'),
 source_bytes BYTEA NOT NULL,
 profile_id UUID NOT NULL CHECK(profile_id<>'00000000-0000-0000-0000-000000000000'),
 reason TEXT NOT NULL CHECK(reason IN ('INCOMPLETE','TERMINAL_PROFILE','CONTRADICTION','INVALID_RESPONSE')),
 result_bytes BYTEA,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(capture_id,source_bytes),
 FOREIGN KEY(capture_id,space_id) REFERENCES space_account_ban_captures(capture_id,space_id) ON DELETE CASCADE
);
CREATE INDEX space_account_ban_reconciliation_space ON space_account_ban_reconciliation(space_id);

CREATE TABLE space_account_qualifications (
 space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 source_bytes BYTEA NOT NULL,
 profile_id UUID NOT NULL CHECK(profile_id<>'00000000-0000-0000-0000-000000000000'),
 account_id UUID NOT NULL CHECK(account_id<>'00000000-0000-0000-0000-000000000000'),
 association_id UUID NOT NULL CHECK(association_id<>'00000000-0000-0000-0000-000000000000'),
 association_revision BIGINT NOT NULL CHECK(association_revision>0),
 owner_receipt_digest BYTEA NOT NULL CHECK(octet_length(owner_receipt_digest)=32),
 result_bytes BYTEA NOT NULL,
 operation_id UUID NOT NULL,
 PRIMARY KEY(space_id,source_bytes)
);

CREATE TABLE space_account_ban_suppressions (
 space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 profile_id UUID NOT NULL CHECK(profile_id<>'00000000-0000-0000-0000-000000000000'),
 account_id UUID NOT NULL CHECK(account_id<>'00000000-0000-0000-0000-000000000000'),
 owner_generation BIGINT NOT NULL CHECK(owner_generation>0),
 source_revision BIGINT NOT NULL CHECK(source_revision>=0),
 episode_id UUID CHECK(episode_id<>'00000000-0000-0000-0000-000000000000'),
 member_generation BIGINT NOT NULL CHECK(member_generation>=0),
 CHECK((episode_id IS NULL)=(member_generation=0)),
 PRIMARY KEY(space_id,profile_id)
);

CREATE TABLE space_account_roster_projections (
 operation_id UUID PRIMARY KEY REFERENCES community_roster_operations(operation_id) ON DELETE CASCADE,
 space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 effective_profile_ids UUID[] NOT NULL
);
CREATE INDEX space_account_roster_projections_space ON space_account_roster_projections(space_id);
-- At the exact inherited45 source every requested profile was projected; this
-- preserves historical operation effects, without adding an association fact.
INSERT INTO space_account_roster_projections(operation_id,space_id,effective_profile_ids)
 SELECT operation_id,space_id,profile_ids FROM community_roster_operations;

-- New helpers are private and independently pinned by the account-ban catalog.
-- They do not modify M44/45's functions or reseal their closed catalog.
CREATE FUNCTION space_account_ban_guard.links() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,space_account_ban_guard,pg_temp AS $$
DECLARE s uuid; lock_key bigint; wall timestamptz; root public.space_account_ban_operations%ROWTYPE;
BEGIN
 IF TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'account coverage cannot truncate' USING ERRCODE='55000'; END IF;
 s:=CASE WHEN TG_OP='DELETE' THEN OLD.space_id ELSE NEW.space_id END;
 IF TG_OP='DELETE' AND public.space_membership_terminal_cleanup_allowed(s) THEN RETURN OLD; END IF;
 IF EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=s)
 OR NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=s) THEN RAISE EXCEPTION 'terminal Space coverage' USING ERRCODE='55000'; END IF;
 lock_key:=('x'||substr(encode(sha256(convert_to('voice.space.mutation.v1','UTF8')||decode('00','hex')||uuid_send(s)),'hex'),1,16))::bit(64)::bigint;
 IF current_setting('transaction_isolation')<>'read committed' OR NOT EXISTS(SELECT 1 FROM pg_locks
  WHERE locktype='advisory' AND pid=pg_backend_pid() AND granted AND mode='ExclusiveLock' AND objsubid=1
  AND classid::bigint=((lock_key>>32)&4294967295) AND objid::bigint=(lock_key&4294967295)) THEN
  RAISE EXCEPTION 'account coverage requires shared Space fence' USING ERRCODE='55000';
 END IF;
 -- Sample wall time after the actual lock wait; transaction-start time is stale.
 wall:=clock_timestamp();
 IF TG_OP='DELETE' THEN
  IF TG_TABLE_NAME='space_account_ban_operations' THEN
   IF OLD.state='COMMITTED' AND wall>OLD.completed_at+interval '30 days'
    AND (OLD.execution_id IS NULL OR wall>=OLD.execution_expires_at) THEN RETURN OLD; END IF;
  ELSIF TG_TABLE_NAME='space_account_ban_captures' THEN
   -- Only the FK cascade of an already lawfully deleted root can erase a child.
   IF NOT EXISTS(SELECT 1 FROM public.space_account_ban_operations WHERE actor_profile_id=OLD.actor_profile_id AND client_operation_id=OLD.client_operation_id) THEN RETURN OLD; END IF;
  ELSIF TG_TABLE_NAME='space_account_ban_reconciliation' THEN
   IF NOT EXISTS(SELECT 1 FROM public.space_account_ban_captures WHERE capture_id=OLD.capture_id) THEN RETURN OLD; END IF;
  END IF;
  RAISE EXCEPTION 'account coverage deletion requires canonical P3 or expired intent cleanup' USING ERRCODE='55000';
 END IF;
 IF TG_TABLE_NAME='space_account_ban_operations' THEN
  IF TG_OP='INSERT' THEN
   IF NEW.state<>'CAPTURING' OR NEW.selected_capture_id IS NOT NULL OR NEW.selected_attempt_seq<>0 THEN RAISE EXCEPTION 'ban operation must begin capturing' USING ERRCODE='55000'; END IF;
   NEW.created_at:=wall;
   IF NEW.execution_id IS NOT NULL AND (NEW.execution_expires_at<=wall OR NEW.execution_expires_at>wall+interval '5 seconds') THEN RAISE EXCEPTION 'invalid execution deadline' USING ERRCODE='55000'; END IF;
   RETURN NEW;
  END IF;
  IF (NEW.actor_profile_id,NEW.client_operation_id,NEW.actor_account_id,NEW.operation_kind,NEW.space_id,NEW.body_sha256,NEW.created_at)
   IS DISTINCT FROM (OLD.actor_profile_id,OLD.client_operation_id,OLD.actor_account_id,OLD.operation_kind,OLD.space_id,OLD.body_sha256,OLD.created_at)
   OR (OLD.state='COMMITTED' AND NEW IS DISTINCT FROM OLD) THEN
   RAISE EXCEPTION 'ban operation binding and terminal outcome immutable' USING ERRCODE='55000';
  END IF;
  IF OLD.state='COMMITTED' THEN RETURN NEW; END IF;
  IF NEW.execution_id IS DISTINCT FROM OLD.execution_id THEN
   IF NEW.state<>'CAPTURING' OR NEW.selected_capture_id IS DISTINCT FROM OLD.selected_capture_id OR NEW.selected_attempt_seq<>OLD.selected_attempt_seq THEN RAISE EXCEPTION 'execution claim cannot change intent selection' USING ERRCODE='55000'; END IF;
   IF NEW.execution_id IS NOT NULL AND ((OLD.execution_id IS NOT NULL AND wall<OLD.execution_expires_at) OR NEW.execution_expires_at<=wall OR NEW.execution_expires_at>wall+interval '5 seconds') THEN RAISE EXCEPTION 'execution already active or invalid deadline' USING ERRCODE='55000'; END IF;
  ELSIF NEW.execution_expires_at IS DISTINCT FROM OLD.execution_expires_at THEN
   RAISE EXCEPTION 'execution cannot renew' USING ERRCODE='55000';
  END IF;
  IF NEW.selected_capture_id IS DISTINCT FROM OLD.selected_capture_id THEN
   IF NEW.operation_kind<>'BAN' OR NEW.state<>'CAPTURING' OR NEW.selected_capture_id IS NULL OR NEW.selected_attempt_seq<>OLD.selected_attempt_seq+1
    OR NEW.execution_id IS DISTINCT FROM OLD.execution_id OR NEW.execution_id IS NULL OR wall>=NEW.execution_expires_at THEN RAISE EXCEPTION 'invalid immutable capture selection' USING ERRCODE='55000'; END IF;
  ELSIF NEW.selected_attempt_seq<>OLD.selected_attempt_seq THEN RAISE EXCEPTION 'capture sequence cannot rebind' USING ERRCODE='55000'; END IF;
  IF NEW.state='CAPTURING' THEN RETURN NEW; END IF;
  IF NEW.execution_id IS NULL OR NEW.execution_id IS DISTINCT FROM OLD.execution_id OR wall>=NEW.execution_expires_at
   OR NEW.selected_capture_id IS DISTINCT FROM OLD.selected_capture_id OR NEW.selected_attempt_seq<>OLD.selected_attempt_seq THEN RAISE EXCEPTION 'stale execution cannot commit' USING ERRCODE='55000'; END IF;
  NEW.completed_at:=wall;
 ELSIF TG_TABLE_NAME='space_account_ban_captures' THEN
  IF TG_OP<>'INSERT' THEN RAISE EXCEPTION 'capture manifest immutable' USING ERRCODE='55000'; END IF;
  SELECT * INTO root FROM public.space_account_ban_operations WHERE actor_profile_id=NEW.actor_profile_id AND client_operation_id=NEW.client_operation_id FOR UPDATE;
  IF NOT FOUND OR root.space_id<>NEW.space_id OR root.operation_kind<>'BAN' OR root.state<>'CAPTURING'
   OR root.execution_id IS NULL OR clock_timestamp()>=root.execution_expires_at OR NEW.attempt_seq<>root.selected_attempt_seq+1 THEN RAISE EXCEPTION 'capture requires current same-root execution' USING ERRCODE='55000'; END IF;
  RETURN NEW;
 ELSIF TG_TABLE_NAME='space_account_ban_reconciliation' THEN
  IF TG_OP<>'INSERT' THEN RAISE EXCEPTION 'capture reconciliation immutable' USING ERRCODE='55000'; END IF;
  SELECT r.* INTO root FROM public.space_account_ban_operations r JOIN public.space_account_ban_captures c
   ON c.actor_profile_id=r.actor_profile_id AND c.client_operation_id=r.client_operation_id AND c.space_id=r.space_id
   WHERE c.capture_id=NEW.capture_id AND c.space_id=NEW.space_id AND r.selected_capture_id=c.capture_id FOR UPDATE OF r;
  IF NOT FOUND OR root.state<>'CAPTURING' OR root.execution_id IS DISTINCT FROM NEW.execution_id
   OR clock_timestamp()>=root.execution_expires_at THEN RAISE EXCEPTION 'reconciliation requires exact current capture execution' USING ERRCODE='55000'; END IF;
  NEW.created_at:=clock_timestamp();
  RETURN NEW;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=s AND xid=txid_current() AND pid=pg_backend_pid() AND mode='MUTATION') THEN
  RAISE EXCEPTION 'account coverage requires captured native mutation' USING ERRCODE='55000';
 END IF;
 IF TG_OP='UPDATE' THEN
  IF TG_TABLE_NAME IN ('space_account_qualifications','space_account_roster_projections') AND NEW IS DISTINCT FROM OLD THEN RAISE EXCEPTION 'episode association or roster projection immutable' USING ERRCODE='55000'; END IF;
  IF TG_TABLE_NAME='space_account_ban_suppressions' THEN
   IF (NEW.space_id<>OLD.space_id OR NEW.profile_id<>OLD.profile_id OR NEW.account_id<>OLD.account_id
    OR NEW.owner_generation<OLD.owner_generation OR (NEW.owner_generation=OLD.owner_generation AND NEW.source_revision<OLD.source_revision)
    OR NEW.member_generation<OLD.member_generation
    OR (NEW.member_generation=OLD.member_generation AND NEW.episode_id IS DISTINCT FROM OLD.episode_id)
    OR (NEW.member_generation>OLD.member_generation AND NEW.episode_id IS NOT DISTINCT FROM OLD.episode_id)) THEN RAISE EXCEPTION 'ban suppression floor cannot rewind or rebind' USING ERRCODE='55000'; END IF;
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION space_account_ban_guard.commit_guard() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,space_account_ban_guard,pg_temp AS $$
DECLARE root public.space_account_ban_operations%ROWTYPE;
BEGIN
 IF NEW.state<>'COMMITTED' OR OLD.state='COMMITTED' THEN RETURN NULL; END IF;
 SELECT * INTO root FROM public.space_account_ban_operations WHERE actor_profile_id=NEW.actor_profile_id AND client_operation_id=NEW.client_operation_id;
 IF NOT FOUND OR root IS DISTINCT FROM NEW OR root.execution_id IS NULL OR clock_timestamp()>=root.execution_expires_at
  OR EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=NEW.space_id)
  OR NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=NEW.space_id) THEN
  RAISE EXCEPTION 'moderation execution expired or changed before commit' USING ERRCODE='55000';
 END IF;
 RETURN NULL;
END $$;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA space_account_ban_guard FROM PUBLIC;

CREATE TRIGGER account_qualification_immutable BEFORE INSERT OR UPDATE OR DELETE ON space_account_qualifications
 FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_qualification_no_truncate BEFORE TRUNCATE ON space_account_qualifications
 FOR EACH STATEMENT EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_suppression_floor BEFORE INSERT OR UPDATE OR DELETE ON space_account_ban_suppressions
 FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_suppression_no_truncate BEFORE TRUNCATE ON space_account_ban_suppressions
 FOR EACH STATEMENT EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_ban_operation_immutable BEFORE INSERT OR UPDATE OR DELETE ON space_account_ban_operations
 FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_ban_operation_no_truncate BEFORE TRUNCATE ON space_account_ban_operations
 FOR EACH STATEMENT EXECUTE FUNCTION space_account_ban_guard.links();
CREATE CONSTRAINT TRIGGER account_ban_commit_guard AFTER UPDATE ON space_account_ban_operations
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.commit_guard();
CREATE TRIGGER account_ban_capture_immutable BEFORE INSERT OR UPDATE OR DELETE ON space_account_ban_captures
 FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_ban_capture_no_truncate BEFORE TRUNCATE ON space_account_ban_captures
 FOR EACH STATEMENT EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_ban_reconciliation_immutable BEFORE INSERT OR UPDATE OR DELETE ON space_account_ban_reconciliation
 FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_ban_reconciliation_no_truncate BEFORE TRUNCATE ON space_account_ban_reconciliation
 FOR EACH STATEMENT EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_roster_projection_immutable BEFORE INSERT OR UPDATE OR DELETE ON space_account_roster_projections
 FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.links();
CREATE TRIGGER account_roster_projection_no_truncate BEFORE TRUNCATE ON space_account_roster_projections
 FOR EACH STATEMENT EXECUTE FUNCTION space_account_ban_guard.links();

CREATE TRIGGER space_authority_revision_space_account_qualifications AFTER INSERT OR UPDATE OR DELETE ON space_account_qualifications
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_account_ban_suppressions AFTER INSERT OR UPDATE OR DELETE ON space_account_ban_suppressions
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');

-- Profile-local writes cannot change another profile's O witness. Every OLD C
-- witness must still be sampled: an unrelated write can observe a time-only
-- owner/member lease gap that a later same-witness renewal would otherwise hide.
-- All global/unknown paths retain the original full-cohort observation. This
-- preserves the existing function OID, ACL and every original frame operation.
CREATE OR REPLACE FUNCTION native_membership_guard.source_after() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE s uuid;r native_membership_guard.profiles;src jsonb;at_time timestamptz;
 affected uuid[]:=ARRAY[]::uuid[];identity uuid;local_relation boolean:=false;valid_identity boolean:=true;
BEGIN
 IF TG_TABLE_NAME='space_effective_member_heads' THEN RETURN NULL; END IF;
 IF TG_TABLE_NAME='spaces' AND TG_OP='UPDATE' AND to_jsonb(NEW)->>'owner_profile_id' IS NOT DISTINCT FROM to_jsonb(OLD)->>'owner_profile_id' THEN RETURN NULL; END IF;
 s:=((CASE WHEN TG_OP='DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END)->>TG_ARGV[0])::uuid;
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s) THEN RETURN NULL; END IF;
 at_time:=clock_timestamp();
 IF TG_OP IN ('INSERT','UPDATE','DELETE') AND TG_RELID=ANY(ARRAY[
  'public.space_members'::regclass,'public.space_ordinary_member_heads'::regclass,
  'public.community_roster_members'::regclass,'public.space_community_member_heads'::regclass]) THEN
  IF TG_OP IN ('UPDATE','DELETE') THEN
   identity:=(to_jsonb(OLD)->>'profile_id')::uuid;
   IF identity IS NULL OR identity='00000000-0000-0000-0000-000000000000'::uuid THEN valid_identity:=false;
   ELSE affected:=array_append(affected,identity); END IF;
  END IF;
  IF TG_OP IN ('INSERT','UPDATE') THEN
   identity:=(to_jsonb(NEW)->>'profile_id')::uuid;
   IF identity IS NULL OR identity='00000000-0000-0000-0000-000000000000'::uuid THEN valid_identity:=false;
   ELSE affected:=array_append(affected,identity); END IF;
  END IF;
  local_relation:=valid_identity AND cardinality(affected)>0;
 END IF;
 -- Capture intermediate semantic disappearance too: ban/unban ABA cannot hide
 -- a real loss merely because its final ordinary witness equals the old one.
 FOR r IN SELECT * FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=s
  AND (NOT local_relation OR profile_id=ANY(affected) OR old_sources->>'community' IS NOT NULL) LOOP
  src:=native_membership_guard.sources(s,r.profile_id,at_time);
  UPDATE native_membership_guard.profiles SET
   ordinary_cut=ordinary_cut OR (r.old_sources->>'ordinary' IS NOT NULL AND r.old_sources->>'ordinary' IS DISTINCT FROM src->>'ordinary'),
   community_cut=community_cut OR (r.old_sources->>'community' IS NOT NULL AND r.old_sources->>'community' IS DISTINCT FROM src->>'community')
   WHERE xid=r.xid AND space_id=s AND profile_id=r.profile_id
    AND ((NOT ordinary_cut AND (r.old_sources->>'ordinary' IS NOT NULL AND r.old_sources->>'ordinary' IS DISTINCT FROM src->>'ordinary'))
     OR (NOT community_cut AND (r.old_sources->>'community' IS NOT NULL AND r.old_sources->>'community' IS DISTINCT FROM src->>'community')));
 END LOOP;
 UPDATE native_membership_guard.frames SET last_source_at=clock_timestamp(),sequence=sequence+1,completed_sequence=-1,
  completed_digest=NULL,completed_at=NULL WHERE xid=txid_current() AND space_id=s;
 RETURN NULL;
END $$;
COMMIT;

-- Preserve every fresh per-invocation catalog projection; aggregate full typed rows.
CREATE OR REPLACE FUNCTION native_membership_guard.catalog() RETURNS bytea LANGUAGE sql STABLE
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
 SELECT sha256(convert_to((SELECT '{"tables": ' || COALESCE('[' || (SELECT string_agg((jsonb_build_array(c.oid,c.relname,c.relowner,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,
    CASE WHEN n.nspname='native_membership_guard' THEN c.relacl END))::text, ', ' ORDER BY c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR (n.nspname='native_membership_guard' AND c.relkind='r')) || ']', 'null') ||
 ', "columns": ' || COALESCE('[' || (SELECT string_agg((to_jsonb(a))::text, ', ' ORDER BY a.attrelid,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE a.attnum>0 AND NOT a.attisdropped AND ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')) || ']', 'null') ||
 ', "triggers": ' || COALESCE('[' || (SELECT string_agg((to_jsonb(t))::text, ', ' ORDER BY t.oid) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard') || ']', 'null') ||
 ', "functions": ' || COALESCE('[' || (SELECT string_agg((jsonb_build_array(p.oid,pg_get_functiondef(p.oid),p.proowner,p.proacl))::text, ', ' ORDER BY p.oid)
   FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
   WHERE p.prokind='f' AND n.nspname IN ('public','native_membership_guard')) || ']', 'null') ||
 ', "constraints": ' || COALESCE('[' || (SELECT string_agg((to_jsonb(k))::text, ', ' ORDER BY k.oid) FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard') || ']', 'null') || '}'),'UTF8'))
$$;
