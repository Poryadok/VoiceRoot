-- Role-purpose owner history. No account/profile ID is retained in these tables.
-- Account rows retain their ordinary raw identity only under Auth's existing
-- policy; due raw deletion never calls a key provider or waits for a consumer.
CREATE TABLE role_identity_owner_state (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  stream_id uuid NOT NULL DEFAULT gen_random_uuid(),
  history_generation uuid NOT NULL DEFAULT gen_random_uuid(),
  through_revision bigint NOT NULL DEFAULT 0 CHECK (through_revision >= 0),
  head_digest bytea NOT NULL CHECK (octet_length(head_digest)=32),
  keyset_revision bigint NOT NULL DEFAULT 0 CHECK (keyset_revision >= 0),
  active_version integer,
  enforcement_enabled boolean NOT NULL DEFAULT false
);
INSERT INTO role_identity_owner_state(head_digest)
VALUES(sha256(convert_to('voice-auth-role-owner-journal-v1','UTF8')));

CREATE TABLE role_identity_key_versions (
  version integer PRIMARY KEY CHECK (version > 0),
  fingerprint bytea NOT NULL UNIQUE CHECK (octet_length(fingerprint)=32)
);

CREATE TABLE role_identity_lifetimes (
  owner_handle uuid PRIMARY KEY,
  lifecycle_revision bigint NOT NULL CHECK (lifecycle_revision > 0),
  state text NOT NULL CHECK (state IN ('ACTIVE','RECOVERABLE','TERMINAL')),
  eligible boolean NOT NULL,
  deletion_cycle uuid,
  erase_at timestamptz,
  selector_complete boolean NOT NULL DEFAULT false,
  CHECK ((state='ACTIVE' AND deletion_cycle IS NULL AND erase_at IS NULL)
      OR (state IN ('RECOVERABLE','TERMINAL') AND deletion_cycle IS NOT NULL AND erase_at IS NOT NULL)),
  CHECK (state='ACTIVE' OR NOT eligible)
);

CREATE TABLE role_identity_account_selectors (
  key_version integer NOT NULL REFERENCES role_identity_key_versions(version),
  selector bytea NOT NULL CHECK (octet_length(selector)=32),
  owner_handle uuid NOT NULL REFERENCES role_identity_lifetimes(owner_handle),
  PRIMARY KEY(key_version,selector),
  UNIQUE(owner_handle,key_version)
);

CREATE TABLE role_identity_owner_journal (
  revision bigint PRIMARY KEY CHECK (revision > 0),
  event_kind text NOT NULL,
  owner_handle uuid,
  lifecycle_revision bigint,
  state text,
  eligible boolean,
  deletion_cycle uuid,
  erase_at timestamptz,
  selector_complete boolean,
  previous_digest bytea NOT NULL CHECK (octet_length(previous_digest)=32),
  digest bytea NOT NULL CHECK (octet_length(digest)=32)
);

ALTER TABLE accounts ADD COLUMN role_identity_handle uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE accounts ADD CONSTRAINT accounts_role_identity_handle_unique UNIQUE(role_identity_handle);
ALTER TABLE accounts ADD COLUMN role_identity_selector_version integer;
ALTER TABLE accounts ADD COLUMN role_identity_account_selector bytea;
ALTER TABLE accounts ADD COLUMN role_identity_keyset_revision bigint;
ALTER TABLE accounts ADD CONSTRAINT accounts_role_identity_capture_shape CHECK (
  (role_identity_selector_version IS NULL AND role_identity_account_selector IS NULL AND role_identity_keyset_revision IS NULL)
  OR (role_identity_selector_version IS NOT NULL AND role_identity_account_selector IS NOT NULL
      AND role_identity_keyset_revision IS NOT NULL AND role_identity_selector_version > 0
      AND octet_length(role_identity_account_selector)=32 AND role_identity_keyset_revision > 0)
);

-- Allocate under a transactional singleton row, not nextval(): rollback removes
-- both the domain mutation and its revision. This local chain is not CURRENT
-- recovery evidence without independently retained complete owner history.
CREATE FUNCTION role_identity_append(p_kind text,p_handle uuid) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE s role_identity_owner_state%ROWTYPE; f role_identity_lifetimes%ROWTYPE;
        selectors_digest bytea; keys_digest bytea; payload bytea; next_digest bytea; next_revision bigint;
BEGIN
  SELECT * INTO STRICT s FROM role_identity_owner_state FOR UPDATE;
  SELECT * INTO f FROM role_identity_lifetimes WHERE owner_handle=p_handle;
  SELECT sha256(COALESCE(string_agg(int4send(key_version)||selector,''::bytea ORDER BY key_version),''::bytea))
    INTO selectors_digest FROM role_identity_account_selectors WHERE owner_handle=p_handle;
  SELECT sha256(COALESCE(string_agg(int4send(version)||fingerprint,''::bytea ORDER BY version),''::bytea))
    INTO keys_digest FROM role_identity_key_versions;
  next_revision := s.through_revision+1;
  payload := int8send(next_revision)||uuid_send(s.stream_id)||uuid_send(s.history_generation)
    ||convert_to(p_kind,'UTF8')||decode('00','hex')
    ||uuid_send(COALESCE(p_handle,'00000000-0000-0000-0000-000000000000'::uuid))
    ||int8send(COALESCE(f.lifecycle_revision,0))||convert_to(COALESCE(f.state,''),'UTF8')||decode('00','hex')
    ||CASE WHEN f.eligible IS NULL THEN decode('02','hex')
           WHEN f.eligible THEN decode('01','hex') ELSE decode('00','hex') END
    ||uuid_send(COALESCE(f.deletion_cycle,'00000000-0000-0000-0000-000000000000'::uuid))
    ||int8send(COALESCE((extract(epoch FROM f.erase_at)*1000000)::bigint,-1))
    ||CASE WHEN f.selector_complete THEN decode('01','hex') ELSE decode('00','hex') END
    ||int8send(s.keyset_revision)||int4send(COALESCE(s.active_version,0))||selectors_digest||keys_digest;
  next_digest := sha256(s.head_digest||payload);
  INSERT INTO role_identity_owner_journal VALUES(next_revision,p_kind,p_handle,f.lifecycle_revision,
    f.state,f.eligible,f.deletion_cycle,f.erase_at,f.selector_complete,s.head_digest,next_digest);
  UPDATE role_identity_owner_state SET through_revision=next_revision,head_digest=next_digest;
  RETURN next_revision;
END $$;

CREATE FUNCTION role_identity_register_keyset(p_revision bigint,p_active integer,p_keys jsonb)
RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE s role_identity_owner_state%ROWTYPE; item jsonb; v integer; fp bytea; old_fp bytea; old_key record;
BEGIN
  IF p_revision < 1 OR p_active < 1 OR jsonb_typeof(p_keys) IS DISTINCT FROM 'array'
     OR jsonb_array_length(p_keys)=0 THEN RAISE EXCEPTION 'invalid identity selector keyset'; END IF;
  IF EXISTS(SELECT 1 FROM jsonb_array_elements(p_keys) k WHERE jsonb_typeof(k)<>'object'
      OR NOT (k ?& ARRAY['version','fingerprint']) OR (k-ARRAY['version','fingerprint'])<>'{}'::jsonb
      OR (k->>'version') !~ '^[1-9][0-9]*$' OR (k->>'fingerprint') !~ '^[a-f0-9]{64}$')
     OR (SELECT count(DISTINCT k->>'version') FROM jsonb_array_elements(p_keys) k)<>jsonb_array_length(p_keys)
     OR (SELECT count(DISTINCT k->>'fingerprint') FROM jsonb_array_elements(p_keys) k)<>jsonb_array_length(p_keys)
     OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(p_keys) k WHERE (k->>'version')::integer=p_active)
     THEN RAISE EXCEPTION 'invalid identity selector keyset'; END IF;
  SELECT * INTO STRICT s FROM role_identity_owner_state FOR UPDATE;
  IF p_revision < s.keyset_revision THEN RAISE EXCEPTION 'identity selector keyset is stale'; END IF;
  FOR old_key IN SELECT * FROM role_identity_key_versions LOOP
    IF NOT EXISTS(SELECT 1 FROM jsonb_array_elements(p_keys) k WHERE (k->>'version')::integer=old_key.version
        AND decode(k->>'fingerprint','hex')=old_key.fingerprint) THEN
      RAISE EXCEPTION 'identity selector keyset unavailable';
    END IF;
  END LOOP;
  IF p_revision=s.keyset_revision THEN
    IF p_active IS DISTINCT FROM s.active_version OR jsonb_array_length(p_keys)<>(SELECT count(*) FROM role_identity_key_versions)
      THEN RAISE EXCEPTION 'identity selector keyset conflict'; END IF;
    RETURN s.keyset_revision;
  END IF;
  FOR item IN SELECT * FROM jsonb_array_elements(p_keys) LOOP
    v := (item->>'version')::integer; fp := decode(item->>'fingerprint','hex');
    SELECT fingerprint INTO old_fp FROM role_identity_key_versions WHERE version=v;
    IF FOUND AND old_fp<>fp THEN RAISE EXCEPTION 'identity selector keyset conflict'; END IF;
    INSERT INTO role_identity_key_versions VALUES(v,fp) ON CONFLICT DO NOTHING;
  END LOOP;
  UPDATE role_identity_owner_state SET keyset_revision=p_revision,active_version=p_active;
  PERFORM role_identity_append('KEYSET',NULL);
  RETURN p_revision;
END $$;

CREATE FUNCTION role_identity_materialize_handle(p_handle uuid) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE f role_identity_lifetimes%ROWTYPE;
BEGIN
  -- The same account -> lifetime -> journal order is used by all domain writers.
  PERFORM 1 FROM accounts WHERE role_identity_handle=p_handle FOR UPDATE;
  SELECT * INTO STRICT f FROM role_identity_lifetimes WHERE owner_handle=p_handle FOR UPDATE;
  IF f.state='RECOVERABLE' AND clock_timestamp()>=f.erase_at THEN
    UPDATE role_identity_lifetimes SET state='TERMINAL',eligible=false,
      lifecycle_revision=lifecycle_revision+1 WHERE owner_handle=p_handle;
    PERFORM role_identity_append('TERMINAL',p_handle);
    RETURN f.lifecycle_revision+1;
  END IF;
  RETURN f.lifecycle_revision;
END $$;

-- Existing retained rows get a distinct lifetime handle, not a fabricated
-- historical selector. Missing pre-capture hard-deletion history stays unknown.
UPDATE accounts SET status='deleted' WHERE deleted_at IS NOT NULL AND status<>'deleted';
INSERT INTO role_identity_lifetimes(owner_handle,lifecycle_revision,state,eligible,deletion_cycle,erase_at)
SELECT role_identity_handle,1,
  CASE WHEN deleted_at IS NULL THEN 'ACTIVE'
       WHEN deleted_at+interval '30 days'<=clock_timestamp() THEN 'TERMINAL' ELSE 'RECOVERABLE' END,
  status='active' AND deleted_at IS NULL,
  CASE WHEN deleted_at IS NOT NULL THEN gen_random_uuid() END,deleted_at+interval '30 days'
FROM accounts;
DO $$ DECLARE h uuid; BEGIN
  FOR h IN SELECT owner_handle FROM role_identity_lifetimes ORDER BY owner_handle LOOP
    PERFORM role_identity_append('BASELINE',h);
  END LOOP;
END $$;

CREATE FUNCTION auth_role_identity_account_created() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s role_identity_owner_state%ROWTYPE; captured boolean;
BEGIN
  SELECT * INTO STRICT s FROM role_identity_owner_state FOR UPDATE;
  IF EXISTS(SELECT 1 FROM role_identity_lifetimes WHERE owner_handle=NEW.role_identity_handle)
    THEN RAISE EXCEPTION 'identity lifetime cannot be recreated'; END IF;
  captured := NEW.role_identity_selector_version IS NOT NULL AND NEW.role_identity_account_selector IS NOT NULL
    AND NEW.role_identity_keyset_revision IS NOT NULL;
  IF s.enforcement_enabled AND NOT captured THEN RAISE EXCEPTION 'identity birth capture required'; END IF;
  IF captured AND (NEW.role_identity_selector_version IS DISTINCT FROM s.active_version
      OR NEW.role_identity_keyset_revision<>s.keyset_revision) THEN RAISE EXCEPTION 'identity selector keyset is stale'; END IF;
  INSERT INTO role_identity_lifetimes VALUES(NEW.role_identity_handle,1,
    CASE WHEN NEW.deleted_at IS NULL THEN 'ACTIVE'
         WHEN NEW.deleted_at+interval '30 days'<=clock_timestamp() THEN 'TERMINAL' ELSE 'RECOVERABLE' END,
    NEW.status='active' AND NEW.deleted_at IS NULL,
    CASE WHEN NEW.deleted_at IS NOT NULL THEN gen_random_uuid() END,NEW.deleted_at+interval '30 days',captured);
  IF captured THEN
    INSERT INTO role_identity_account_selectors VALUES(NEW.role_identity_selector_version,
      NEW.role_identity_account_selector,NEW.role_identity_handle);
  END IF;
  PERFORM role_identity_append('BIRTH',NEW.role_identity_handle);
  RETURN NEW;
END $$;
CREATE TRIGGER auth_role_identity_account_created AFTER INSERT ON accounts
FOR EACH ROW EXECUTE FUNCTION auth_role_identity_account_created();

CREATE FUNCTION auth_role_identity_account_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE f role_identity_lifetimes%ROWTYPE; s role_identity_owner_state%ROWTYPE; changed boolean := false;
BEGIN
  -- UPDATE/DELETE already hold the exact account row lock. A transaction-start
  -- clock must never authorize an expired restore after waiting on that lock.
  SELECT * INTO STRICT f FROM role_identity_lifetimes WHERE owner_handle=OLD.role_identity_handle FOR UPDATE;
  IF TG_OP='DELETE' THEN
    IF f.state<>'TERMINAL' AND (OLD.deleted_at IS NULL OR clock_timestamp()<OLD.deleted_at+interval '30 days')
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
    IF clock_timestamp()>=OLD.deleted_at+interval '30 days' THEN
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
                 WHEN NEW.deleted_at+interval '30 days'<=clock_timestamp() THEN 'TERMINAL' ELSE 'RECOVERABLE' END,
      eligible=NEW.status='active' AND NEW.deleted_at IS NULL,
      deletion_cycle=CASE WHEN NEW.deleted_at IS NOT NULL THEN gen_random_uuid() END,
      erase_at=NEW.deleted_at+interval '30 days' WHERE owner_handle=f.owner_handle;
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
CREATE TRIGGER auth_role_identity_account_guard BEFORE UPDATE OR DELETE ON accounts
FOR EACH ROW EXECUTE FUNCTION auth_role_identity_account_guard();

CREATE FUNCTION role_identity_journal_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'identity owner journal is append-only'; END $$;
CREATE TRIGGER role_identity_journal_immutable BEFORE UPDATE OR DELETE ON role_identity_owner_journal
FOR EACH ROW EXECUTE FUNCTION role_identity_journal_immutable();
