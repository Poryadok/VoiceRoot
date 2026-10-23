-- The canonical core is extended here before publication; the identity guard
-- itself performs no owner call, selector computation or additional lock.
CREATE FUNCTION reject_user_profile_identity_rebind() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.account_id IS DISTINCT FROM OLD.account_id THEN
    RAISE EXCEPTION 'User profile identity cannot be rebound'
      USING ERRCODE='23514', CONSTRAINT='user_profile_identity_immutable';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER user_profile_identity_immutable
BEFORE UPDATE ON profiles FOR EACH ROW
EXECUTE FUNCTION reject_user_profile_identity_rebind();

-- I is the outer mutation mutex. H is independent of User19's statement clock A.
CREATE TABLE identity_meta (
  singleton boolean PRIMARY KEY CHECK(singleton),
  history_generation uuid NOT NULL DEFAULT gen_random_uuid(),
  user_source_generation uuid NOT NULL DEFAULT gen_random_uuid(),
  history_head bigint NOT NULL DEFAULT 0 CHECK(history_head>=0),
  retained_floor bigint NOT NULL DEFAULT 0 CHECK(retained_floor>=0 AND retained_floor<=history_head),
  head_digest bytea NOT NULL DEFAULT decode('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855','hex') CHECK(octet_length(head_digest)=32),
  keyset_revision bigint NOT NULL DEFAULT 0 CHECK(keyset_revision>=0),
  active_key_version bigint CHECK(active_key_version>0 AND active_key_version<=4294967295),
  guards_active boolean NOT NULL DEFAULT false,
  baseline_cut bigint NOT NULL DEFAULT 0 CHECK(baseline_cut>=0),
  unresolved_erase_issued bigint NOT NULL DEFAULT 0 CHECK(unresolved_erase_issued>=0 AND unresolved_erase_issued<=history_head),
  unresolved_erase_covered bigint NOT NULL DEFAULT 0 CHECK(unresolved_erase_covered>=0 AND unresolved_erase_covered<=unresolved_erase_issued)
);
INSERT INTO identity_meta(singleton) VALUES(true);

CREATE TABLE identity_key_versions (
  version bigint PRIMARY KEY CHECK(version>0 AND version<=4294967295),
  fingerprint bytea NOT NULL CHECK(octet_length(fingerprint)=32)
);

-- Permanent used-ID evidence has no account, association, parent or raw UUID.
CREATE TABLE identity_profile_reservations (
  key_version bigint NOT NULL REFERENCES identity_key_versions(version),
  profile_selector bytea NOT NULL CHECK(octet_length(profile_selector)=32),
  PRIMARY KEY(key_version,profile_selector)
);

CREATE TABLE identity_associations (
  association_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  association_revision bigint NOT NULL CHECK(association_revision>0),
  capture_revision bigint NOT NULL CHECK(capture_revision>0),
  capture_origin text NOT NULL CHECK(capture_origin IN ('BIRTH','RETAINED_NOW','VERIFIED_HISTORY')),
  captured_at timestamptz NOT NULL,
  receipt_id uuid NOT NULL UNIQUE,
  receipt_digest bytea NOT NULL CHECK(octet_length(receipt_digest)=32),
  parent_handle text,
  parent_lifecycle_revision bigint CHECK(parent_lifecycle_revision>0),
  parent_state text CHECK(parent_state IN ('ACTIVE','RECOVERABLE','TERMINAL')),
  parent_cut jsonb,
  deleted boolean NOT NULL DEFAULT false,
  frozen boolean NOT NULL DEFAULT false,
  account_inactive boolean NOT NULL DEFAULT false,
  profile_revision bigint NOT NULL CHECK(profile_revision>0),
  terminal_fact jsonb,
  CHECK((parent_handle IS NULL)=(parent_lifecycle_revision IS NULL)),
  CHECK((parent_handle IS NULL)=(parent_state IS NULL)),
  CHECK((parent_handle IS NULL)=(parent_cut IS NULL)),
  CHECK(capture_origin<>'BIRTH' OR parent_handle IS NOT NULL)
);

CREATE TABLE association_selector_aliases (
  association_id uuid NOT NULL REFERENCES identity_associations(association_id),
  key_version bigint NOT NULL REFERENCES identity_key_versions(version),
  kind text NOT NULL CHECK(kind IN ('ACCOUNT','PROFILE')),
  selector bytea NOT NULL CHECK(octet_length(selector)=32),
  PRIMARY KEY(association_id,key_version,kind)
);
CREATE UNIQUE INDEX association_unique_profile_alias ON association_selector_aliases(key_version,selector) WHERE kind='PROFILE';
CREATE INDEX association_account_alias ON association_selector_aliases(key_version,selector) WHERE kind='ACCOUNT';

-- Local Auth evidence is serialized by I. These aliases contain only User
-- account-purpose HMACs and an opaque parent, never a raw account identifier.
CREATE TABLE identity_parent_fences (
  owner_handle text PRIMARY KEY CHECK(length(owner_handle)>0),
  lifecycle_revision bigint NOT NULL CHECK(lifecycle_revision>0),
  state text NOT NULL CHECK(state IN ('ACTIVE','RECOVERABLE','TERMINAL')),
  owner_cut jsonb NOT NULL,
  owner_fact jsonb NOT NULL
);
CREATE TABLE identity_account_parent_aliases (
  key_version bigint NOT NULL REFERENCES identity_key_versions(version),
  account_selector bytea NOT NULL CHECK(octet_length(account_selector)=32),
  owner_handle text NOT NULL REFERENCES identity_parent_fences(owner_handle),
  PRIMARY KEY(key_version,account_selector)
);

CREATE TABLE identity_journal (
  revision bigint PRIMARY KEY CHECK(revision>0),
  association_id uuid,
  association_revision bigint,
  kind text NOT NULL CHECK(kind IN ('KEYSET','GUARDS_ACTIVATED','CAPTURE','BIRTH','PARENT_ATTACHED','LIFECYCLE','TERMINAL','ROSTER_SEALED','RETIRED','UNRESOLVED_RAW_ERASURE')),
  deleted boolean,
  frozen boolean,
  account_inactive boolean,
  previous_digest bytea NOT NULL CHECK(octet_length(previous_digest)=32),
  fact_digest bytea NOT NULL CHECK(octet_length(fact_digest)=32),
  digest bytea NOT NULL CHECK(octet_length(digest)=32),
  committed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK((association_id IS NULL)=(association_revision IS NULL)),
  CHECK(kind<>'UNRESOLVED_RAW_ERASURE' OR (association_id IS NULL AND association_revision IS NULL AND deleted IS NULL AND frozen IS NULL AND account_inactive IS NULL))
);

ALTER TABLE profiles ADD COLUMN identity_association_id uuid UNIQUE REFERENCES identity_associations(association_id);
ALTER TABLE profiles ADD COLUMN identity_baseline_cut bigint NOT NULL DEFAULT 0 CHECK(identity_baseline_cut>=0);

-- Defensive gates never wait with a previously acquired subordinate lock.
-- Supported mixed operations explicitly acquire I before their first lock.
CREATE FUNCTION identity_statement_gate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM 1 FROM public.identity_meta WHERE singleton FOR UPDATE NOWAIT;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'User identity mutex missing' USING ERRCODE='55000';
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER identity_profiles_gate BEFORE INSERT OR UPDATE OR DELETE ON profiles
FOR EACH STATEMENT EXECUTE FUNCTION identity_statement_gate();
CREATE TRIGGER identity_lifecycle_gate BEFORE INSERT OR UPDATE OR DELETE ON user_account_lifecycle
FOR EACH STATEMENT EXECUTE FUNCTION identity_statement_gate();
CREATE TRIGGER identity_sdk_gate BEFORE INSERT OR UPDATE OR DELETE ON sdk_author_tombstones
FOR EACH STATEMENT EXECUTE FUNCTION identity_statement_gate();
CREATE TRIGGER identity_authority_clock_gate BEFORE UPDATE ON user_authority_revision
FOR EACH STATEMENT EXECUTE FUNCTION identity_statement_gate();

DO $$ DECLARE relation text; BEGIN
  FOREACH relation IN ARRAY ARRAY['identity_key_versions','identity_profile_reservations','identity_associations','association_selector_aliases','identity_journal','identity_parent_fences','identity_account_parent_aliases'] LOOP
    EXECUTE format('CREATE TRIGGER identity_mutation_gate BEFORE INSERT OR UPDATE OR DELETE ON %I FOR EACH STATEMENT EXECUTE FUNCTION identity_statement_gate()',relation);
  END LOOP;
END $$;

-- No sequence/MAX value is a committed history cut. I serializes this allocator
-- and the row, domain state and journal entry commit or roll back together.
CREATE FUNCTION identity_append_event(a uuid, ar bigint, event_kind text, d boolean, f boolean, inactive boolean, fact bytea DEFAULT NULL)
RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE old_head bigint; next_head bigint; previous bytea; next_digest bytea;
BEGIN
  IF event_kind='UNRESOLVED_RAW_ERASURE' AND (a IS NOT NULL OR ar IS NOT NULL OR d IS NOT NULL OR f IS NOT NULL OR inactive IS NOT NULL OR fact IS NOT NULL) THEN
    RAISE EXCEPTION 'User unresolved gap cannot carry identity commitments' USING ERRCODE='23514';
  END IF;
  SELECT history_head,head_digest INTO old_head,previous FROM public.identity_meta WHERE singleton FOR UPDATE NOWAIT;
  IF NOT FOUND OR old_head=9223372036854775807 THEN
    RAISE EXCEPTION 'User identity head unavailable' USING ERRCODE='55000';
  END IF;
  next_head:=old_head+1;
  IF fact IS NULL THEN
    IF a IS NOT NULL THEN
      SELECT sha256(convert_to(jsonb_build_object('association',to_jsonb(item),'aliases',COALESCE((
        SELECT jsonb_agg(jsonb_build_array(key_version,kind,encode(selector,'hex')) ORDER BY key_version,kind)
        FROM public.association_selector_aliases WHERE association_id=a),'[]'::jsonb))::text,'UTF8'))
      INTO fact FROM public.identity_associations item WHERE item.association_id=a;
    ELSIF event_kind='KEYSET' THEN
      SELECT sha256(convert_to(jsonb_build_object('revision',m.keyset_revision,'active',m.active_key_version,'versions',(
        SELECT jsonb_agg(jsonb_build_array(version,encode(fingerprint,'hex')) ORDER BY version) FROM public.identity_key_versions))::text,'UTF8'))
      INTO fact FROM public.identity_meta m WHERE singleton;
    ELSIF event_kind='GUARDS_ACTIVATED' THEN
      SELECT sha256(convert_to(jsonb_build_object('baseline_cut',baseline_cut,'guards_active',guards_active,'keyset_revision',keyset_revision)::text,'UTF8'))
      INTO fact FROM public.identity_meta WHERE singleton;
    ELSIF event_kind='UNRESOLVED_RAW_ERASURE' THEN
      SELECT sha256(convert_to('voice.user.identity-association.unresolved-raw-erasure.v1','UTF8') || uuid_send(history_generation) || int8send(next_head) || convert_to('MISSING_REQUIRED_SELECTOR_COVERAGE','UTF8') || int8send(1))
      INTO fact FROM public.identity_meta WHERE singleton;
    END IF;
  END IF;
  IF fact IS NULL OR octet_length(fact)<>32 THEN RAISE EXCEPTION 'User identity event requires owner fact commitment' USING ERRCODE='23514'; END IF;
  next_digest:=sha256(previous || int8send(next_head) ||
    COALESCE(uuid_send(a),decode(repeat('00',16),'hex')) || int8send(COALESCE(ar,0)) ||
    convert_to(event_kind||':'||COALESCE(d::text,'?')||':'||COALESCE(f::text,'?')||':'||COALESCE(inactive::text,'?'),'UTF8') || fact);
  INSERT INTO public.identity_journal(revision,association_id,association_revision,kind,deleted,frozen,account_inactive,previous_digest,fact_digest,digest)
  VALUES(next_head,a,ar,event_kind,d,f,inactive,previous,fact,next_digest);
  UPDATE public.identity_meta SET history_head=next_head,head_digest=next_digest WHERE singleton;
  RETURN next_head;
END;
$$;

CREATE FUNCTION identity_meta_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP IN ('DELETE','TRUNCATE') THEN
    RAISE EXCEPTION 'User identity source cannot be reset' USING ERRCODE='55000';
  END IF;
  IF NEW.singleton IS DISTINCT FROM OLD.singleton OR NEW.history_generation IS DISTINCT FROM OLD.history_generation
    OR NEW.user_source_generation IS DISTINCT FROM OLD.user_source_generation OR NEW.history_head<OLD.history_head
    OR NEW.retained_floor<OLD.retained_floor OR NEW.keyset_revision<OLD.keyset_revision
    OR (OLD.guards_active AND NOT NEW.guards_active) OR NEW.baseline_cut<OLD.baseline_cut THEN
    RAISE EXCEPTION 'User identity source cannot rewind' USING ERRCODE='55000';
  END IF;
  IF NEW.unresolved_erase_issued<OLD.unresolved_erase_issued OR NEW.unresolved_erase_covered IS DISTINCT FROM OLD.unresolved_erase_covered THEN
    RAISE EXCEPTION 'User unresolved coverage requires independent verified closure' USING ERRCODE='55000';
  END IF;
  IF NEW.unresolved_erase_issued>OLD.unresolved_erase_issued AND (NEW.unresolved_erase_issued<>NEW.history_head OR NOT EXISTS(
    SELECT 1 FROM public.identity_journal WHERE revision=NEW.unresolved_erase_issued AND kind='UNRESOLVED_RAW_ERASURE' AND association_id IS NULL AND association_revision IS NULL)) THEN
    RAISE EXCEPTION 'User unresolved coverage requires committed gap event' USING ERRCODE='55000';
  END IF;
  IF NEW.history_head=OLD.history_head THEN
    IF NEW.head_digest IS DISTINCT FROM OLD.head_digest THEN
      RAISE EXCEPTION 'User identity digest cannot relabel a cut' USING ERRCODE='55000';
    END IF;
  ELSIF NEW.history_head::numeric<>OLD.history_head::numeric+1 OR NOT EXISTS(
    SELECT 1 FROM public.identity_journal j WHERE j.revision=NEW.history_head AND j.digest=NEW.head_digest AND j.previous_digest=OLD.head_digest) THEN
    RAISE EXCEPTION 'User identity head requires contiguous journal' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER identity_meta_no_reset BEFORE UPDATE OR DELETE ON identity_meta
FOR EACH ROW EXECUTE FUNCTION identity_meta_guard();
CREATE TRIGGER identity_meta_no_truncate BEFORE TRUNCATE ON identity_meta
FOR EACH STATEMENT EXECUTE FUNCTION identity_meta_guard();

CREATE FUNCTION identity_reservation_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'User used-profile reservations are permanent' USING ERRCODE='55000';
END;
$$;
CREATE TRIGGER identity_reservation_immutable BEFORE UPDATE OR DELETE ON identity_profile_reservations
FOR EACH ROW EXECUTE FUNCTION identity_reservation_guard();
CREATE TRIGGER identity_reservation_no_truncate BEFORE TRUNCATE ON identity_profile_reservations
FOR EACH STATEMENT EXECUTE FUNCTION identity_reservation_guard();

-- Trusted User registrar preparation is transaction-local. A GUC or a
-- shape-valid hash alone is not preparation. Registrar/DB-owner SQL forgery is
-- outside this application boundary; no secret is stored or computed in SQL.
CREATE FUNCTION identity_has_preparation(p uuid, account uuid, association uuid, purpose text)
RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE allowed boolean:=false;
BEGIN
  IF to_regclass('pg_temp.user_identity_preparation') IS NULL THEN RETURN false; END IF;
  EXECUTE 'SELECT EXISTS(SELECT 1 FROM pg_temp.user_identity_preparation WHERE transaction_id=txid_current() AND profile_id IS NOT DISTINCT FROM $1 AND account_id IS NOT DISTINCT FROM $2 AND association_id IS NOT DISTINCT FROM $3 AND kind=$4)'
  INTO allowed USING p,account,association,purpose;
  RETURN allowed;
END;
$$;

CREATE FUNCTION identity_profile_registrar_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE active boolean; prepared uuid; due record; consumed bigint; gap_revision bigint;
BEGIN
  SELECT guards_active INTO active FROM public.identity_meta WHERE singleton;
  IF NOT FOUND THEN RAISE EXCEPTION 'User identity source missing' USING ERRCODE='55000'; END IF;
  IF TG_OP='DELETE' THEN
    IF to_regclass('pg_temp.user_identity_due_erasure') IS NOT NULL THEN
      EXECUTE 'DELETE FROM pg_temp.user_identity_due_erasure WHERE transaction_id=txid_current() AND profile_id=$1 AND account_id=$2 AND association_id IS NOT DISTINCT FROM $3 RETURNING *'
        INTO due USING OLD.id,OLD.account_id,OLD.identity_association_id;
      GET DIAGNOSTICS consumed=ROW_COUNT;
      IF consumed=1 THEN
        IF due.valid_until<=clock_timestamp() OR due.observed_at>clock_timestamp() OR due.erase_at>due.observed_at OR due.lifecycle_revision<=0 OR due.cycle_id='00000000-0000-0000-0000-000000000000'::uuid THEN
          RAISE EXCEPTION 'User due erasure authority expired or invalid' USING ERRCODE='23514';
        END IF;
        IF OLD.identity_association_id IS NOT NULL AND EXISTS(SELECT 1 FROM public.identity_associations a WHERE a.association_id=OLD.identity_association_id AND a.parent_handle IS NOT NULL AND
          (a.parent_handle<>due.owner_handle OR a.parent_lifecycle_revision>due.lifecycle_revision OR a.parent_cut->>'Owner' IS DISTINCT FROM due.parent_cut->>'Owner'
          OR a.parent_cut->>'Stream' IS DISTINCT FROM due.parent_cut->>'Stream' OR a.parent_cut->>'HistoryGeneration' IS DISTINCT FROM due.parent_cut->>'HistoryGeneration'
          OR (a.parent_cut->>'ThroughRevision')::numeric>(due.parent_cut->>'ThroughRevision')::numeric OR (a.parent_cut->>'KeysetRevision')::numeric>(due.parent_cut->>'KeysetRevision')::numeric
          OR ((a.parent_cut->>'ThroughRevision')::numeric=(due.parent_cut->>'ThroughRevision')::numeric AND a.parent_cut->>'Digest' IS DISTINCT FROM due.parent_cut->>'Digest'))) THEN
          RAISE EXCEPTION 'User due erasure parent binding conflicts' USING ERRCODE='23514';
        END IF;
        -- Conservative fallback: no association-id-only reservation shortcut.
        -- H itself is the gap identity; this event commits no erased identity.
        gap_revision:=identity_append_event(NULL,NULL,'UNRESOLVED_RAW_ERASURE',NULL,NULL,NULL);
        UPDATE public.identity_meta SET unresolved_erase_issued=gap_revision WHERE singleton;
        RETURN OLD;
      END IF;
    END IF;
    IF active AND (OLD.identity_association_id IS NULL OR NOT identity_has_preparation(OLD.id,OLD.account_id,OLD.identity_association_id,'TERMINAL_DELETE')) THEN
      RAISE EXCEPTION 'User profile deletion requires preserved owner fact' USING ERRCODE='23514';
    END IF;
    RETURN OLD;
  END IF;
  IF TG_OP='INSERT' THEN
    IF NEW.identity_baseline_cut<>0 THEN RAISE EXCEPTION 'New profile cannot claim retained baseline' USING ERRCODE='23514'; END IF;
    IF EXISTS(SELECT 1 FROM public.identity_meta WHERE singleton AND unresolved_erase_issued<>unresolved_erase_covered) THEN
      RAISE EXCEPTION 'User new birth lacks complete nonreuse coverage' USING ERRCODE='23514';
    END IF;
    IF active THEN
      IF to_regclass('pg_temp.user_identity_preparation') IS NOT NULL THEN
        EXECUTE 'SELECT association_id FROM pg_temp.user_identity_preparation WHERE transaction_id=txid_current() AND profile_id=$1 AND account_id=$2 AND kind=''BIRTH'''
          INTO prepared USING NEW.id,NEW.account_id;
      END IF;
      IF prepared IS NULL OR (NEW.identity_association_id IS NOT NULL AND NEW.identity_association_id<>prepared) THEN
        RAISE EXCEPTION 'New profile requires canonical birth preparation' USING ERRCODE='23514';
      END IF;
      NEW.identity_association_id:=prepared;
    ELSIF NEW.identity_association_id IS NOT NULL THEN
      RAISE EXCEPTION 'Canonical births require activated owner guards' USING ERRCODE='23514';
    END IF;
  ELSE
    IF NEW.identity_baseline_cut IS DISTINCT FROM OLD.identity_baseline_cut AND NOT identity_has_preparation(NULL,NULL,NULL,'ACTIVATE') THEN
      RAISE EXCEPTION 'User retained baseline is immutable' USING ERRCODE='23514';
    END IF;
    IF NEW.identity_association_id IS DISTINCT FROM OLD.identity_association_id THEN
      IF OLD.identity_association_id IS NOT NULL OR NEW.identity_association_id IS NULL OR NOT identity_has_preparation(NEW.id,NEW.account_id,NEW.identity_association_id,'RETAINED_NOW') THEN
        RAISE EXCEPTION 'User association reference cannot rebind' USING ERRCODE='23514';
      END IF;
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER identity_profile_registrar BEFORE INSERT OR UPDATE OR DELETE ON profiles
FOR EACH ROW EXECUTE FUNCTION identity_profile_registrar_guard();

CREATE FUNCTION identity_profile_lifecycle_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE next_revision bigint; inactive boolean;
BEGIN
  IF NEW.identity_association_id IS NULL OR (TG_OP='UPDATE' AND NEW.deleted_at IS NOT DISTINCT FROM OLD.deleted_at AND NEW.frozen_at IS NOT DISTINCT FROM OLD.frozen_at) THEN RETURN NULL; END IF;
  SELECT EXISTS(SELECT 1 FROM public.user_account_lifecycle WHERE account_id=NEW.account_id AND state='ACCOUNT_INACTIVE') INTO inactive;
  IF EXISTS(SELECT 1 FROM public.identity_associations a WHERE a.association_id=NEW.identity_association_id
    AND (a.deleted IS DISTINCT FROM (NEW.deleted_at IS NOT NULL) OR a.frozen IS DISTINCT FROM (NEW.frozen_at IS NOT NULL) OR a.account_inactive IS DISTINCT FROM inactive)) THEN
    SELECT history_head+1 INTO next_revision FROM public.identity_meta WHERE singleton;
    UPDATE public.identity_associations SET association_revision=next_revision,deleted=NEW.deleted_at IS NOT NULL,
      frozen=NEW.frozen_at IS NOT NULL,account_inactive=inactive,profile_revision=NEW.sdk_eligibility_revision WHERE association_id=NEW.identity_association_id;
    PERFORM identity_append_event(NEW.identity_association_id,next_revision,'LIFECYCLE',NEW.deleted_at IS NOT NULL,NEW.frozen_at IS NOT NULL,inactive);
  END IF;
  RETURN NULL;
END;
$$;
CREATE TRIGGER identity_profile_lifecycle AFTER INSERT OR UPDATE ON profiles
FOR EACH ROW EXECUTE FUNCTION identity_profile_lifecycle_changed();

CREATE FUNCTION identity_account_lifecycle_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE account uuid; inactive boolean; item record; next_revision bigint;
BEGIN
  IF TG_OP='DELETE' THEN account:=OLD.account_id; ELSE account:=NEW.account_id; END IF;
  SELECT EXISTS(SELECT 1 FROM public.user_account_lifecycle WHERE account_id=account AND state='ACCOUNT_INACTIVE') INTO inactive;
  FOR item IN SELECT a.association_id,a.deleted,a.frozen,p.sdk_eligibility_revision
    FROM public.identity_associations a JOIN public.profiles p ON p.identity_association_id=a.association_id
    WHERE p.account_id=account AND a.account_inactive IS DISTINCT FROM inactive ORDER BY a.association_id
  LOOP
    SELECT history_head+1 INTO next_revision FROM public.identity_meta WHERE singleton;
    UPDATE public.identity_associations SET association_revision=next_revision,account_inactive=inactive,
      profile_revision=item.sdk_eligibility_revision WHERE association_id=item.association_id;
    PERFORM identity_append_event(item.association_id,next_revision,'LIFECYCLE',item.deleted,item.frozen,inactive);
  END LOOP;
  RETURN NULL;
END;
$$;
CREATE TRIGGER identity_account_lifecycle AFTER INSERT OR UPDATE OR DELETE ON user_account_lifecycle
FOR EACH ROW EXECUTE FUNCTION identity_account_lifecycle_changed();

CREATE FUNCTION identity_birth_complete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.capture_origin='BIRTH' AND NOT EXISTS(SELECT 1 FROM public.profiles WHERE identity_association_id=NEW.association_id) THEN
    RAISE EXCEPTION 'User canonical birth must commit its profile' USING ERRCODE='23514';
  END IF;
  RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER identity_birth_requires_profile AFTER INSERT ON identity_associations
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION identity_birth_complete();

-- Receipts certify an immutable capture, never an editable interpretation of
-- an old cut. Parent/lifecycle enrichment has its own semantic journal revision.
CREATE FUNCTION identity_association_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP IN ('DELETE','TRUNCATE') THEN
    RAISE EXCEPTION 'User association retirement requires explicit owner cleanup' USING ERRCODE='55000';
  END IF;
  IF NEW.association_id IS DISTINCT FROM OLD.association_id OR NEW.capture_revision IS DISTINCT FROM OLD.capture_revision
    OR NEW.capture_origin IS DISTINCT FROM OLD.capture_origin OR NEW.captured_at IS DISTINCT FROM OLD.captured_at
    OR NEW.receipt_id IS DISTINCT FROM OLD.receipt_id OR NEW.receipt_digest IS DISTINCT FROM OLD.receipt_digest
    OR NEW.association_revision<OLD.association_revision
    OR (OLD.parent_handle IS NOT NULL AND NEW.parent_handle IS DISTINCT FROM OLD.parent_handle)
    OR (OLD.parent_lifecycle_revision IS NOT NULL AND (NEW.parent_lifecycle_revision IS NULL OR NEW.parent_lifecycle_revision<OLD.parent_lifecycle_revision))
    OR (OLD.parent_state='TERMINAL' AND NEW.parent_state IS DISTINCT FROM OLD.parent_state)
    OR (OLD.terminal_fact IS NOT NULL AND NEW.terminal_fact IS DISTINCT FROM OLD.terminal_fact) THEN
    RAISE EXCEPTION 'User canonical association cannot rebind or rewind' USING ERRCODE='23514';
  END IF;
  IF ROW(NEW.parent_handle,NEW.parent_lifecycle_revision,NEW.parent_state,NEW.parent_cut,NEW.deleted,NEW.frozen,NEW.account_inactive,NEW.terminal_fact)
    IS DISTINCT FROM ROW(OLD.parent_handle,OLD.parent_lifecycle_revision,OLD.parent_state,OLD.parent_cut,OLD.deleted,OLD.frozen,OLD.account_inactive,OLD.terminal_fact)
    AND NEW.association_revision<=OLD.association_revision THEN
    RAISE EXCEPTION 'User semantic association change requires new journal revision' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER identity_association_immutable BEFORE UPDATE OR DELETE ON identity_associations
FOR EACH ROW EXECUTE FUNCTION identity_association_guard();
CREATE TRIGGER identity_association_no_truncate BEFORE TRUNCATE ON identity_associations
FOR EACH STATEMENT EXECUTE FUNCTION identity_association_guard();

CREATE FUNCTION identity_append_only_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'User identity evidence cannot be rewritten; explicit owner compaction required' USING ERRCODE='55000';
END;
$$;
DO $$ DECLARE relation text; BEGIN
  FOREACH relation IN ARRAY ARRAY['identity_key_versions','association_selector_aliases','identity_journal','identity_account_parent_aliases'] LOOP
    EXECUTE format('CREATE TRIGGER identity_evidence_immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION identity_append_only_guard()',relation);
    EXECUTE format('CREATE TRIGGER identity_evidence_no_truncate BEFORE TRUNCATE ON %I FOR EACH STATEMENT EXECUTE FUNCTION identity_append_only_guard()',relation);
  END LOOP;
END $$;

CREATE FUNCTION identity_association_journal_complete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NOT EXISTS(SELECT 1 FROM public.identity_journal j WHERE j.association_id=NEW.association_id AND j.association_revision=NEW.association_revision AND j.revision=NEW.association_revision) THEN
    RAISE EXCEPTION 'User association requires committed owner journal event' USING ERRCODE='23514';
  END IF;
  RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER identity_association_requires_journal AFTER INSERT OR UPDATE ON identity_associations
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION identity_association_journal_complete();

CREATE FUNCTION identity_parent_fence_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP IN ('DELETE','TRUNCATE') THEN
    RAISE EXCEPTION 'User parent cleanup requires explicit owner retirement' USING ERRCODE='55000';
  END IF;
  IF NEW.owner_handle IS DISTINCT FROM OLD.owner_handle OR NEW.lifecycle_revision<OLD.lifecycle_revision
    OR (OLD.state='TERMINAL' AND NEW.state IS DISTINCT FROM OLD.state)
    OR NEW.owner_cut->>'Owner' IS DISTINCT FROM OLD.owner_cut->>'Owner'
    OR NEW.owner_cut->>'Stream' IS DISTINCT FROM OLD.owner_cut->>'Stream'
    OR NEW.owner_cut->>'HistoryGeneration' IS DISTINCT FROM OLD.owner_cut->>'HistoryGeneration'
    OR (NEW.owner_cut->>'ThroughRevision')::numeric<(OLD.owner_cut->>'ThroughRevision')::numeric
    OR (NEW.owner_cut->>'KeysetRevision')::numeric<(OLD.owner_cut->>'KeysetRevision')::numeric THEN
    RAISE EXCEPTION 'User parent fence cannot rebind or rewind' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER identity_parent_no_rewind BEFORE UPDATE OR DELETE ON identity_parent_fences
FOR EACH ROW EXECUTE FUNCTION identity_parent_fence_guard();
CREATE TRIGGER identity_parent_no_truncate BEFORE TRUNCATE ON identity_parent_fences
FOR EACH STATEMENT EXECUTE FUNCTION identity_parent_fence_guard();

-- One ordered server-side loop avoids a per-profile network roundtrip without
-- treating a set/sequence/MAX as a committed journal allocation.
CREATE FUNCTION identity_append_capture_batch() RETURNS void LANGUAGE plpgsql AS $$
DECLARE item record; allocated bigint;
BEGIN
  FOR item IN SELECT a.association_id,a.association_revision,a.deleted,a.frozen,a.account_inactive,p.profile_id,p.account_id,b.expected_revision
    FROM pg_temp.user_identity_capture_batch b JOIN public.identity_associations a USING(association_id)
    JOIN pg_temp.user_identity_preparation p USING(association_id)
    WHERE p.transaction_id=txid_current() AND p.kind='RETAINED_NOW'
    ORDER BY b.expected_revision
  LOOP
    IF item.association_revision<>item.expected_revision OR NOT identity_has_preparation(item.profile_id,item.account_id,item.association_id,'RETAINED_NOW') THEN
      RAISE EXCEPTION 'User capture batch preparation mismatch' USING ERRCODE='23514';
    END IF;
    allocated:=identity_append_event(item.association_id,item.association_revision,'CAPTURE',item.deleted,item.frozen,item.account_inactive);
    IF allocated<>item.expected_revision THEN RAISE EXCEPTION 'User capture batch cut mismatch' USING ERRCODE='23514'; END IF;
  END LOOP;
END;
$$;
