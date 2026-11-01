-- Forward-only repair for the missing GIS15–30 history, without rewriting32.
-- Requires exclusive privileged-DDL custody in addition to the migrator lock.
-- Layout identity is independently exercised on PostgreSQL16 and18.
BEGIN;
SET LOCAL search_path=pg_catalog,public;
SET LOCAL row_security=off;
LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE;
DO $lock$ DECLARE relation record; BEGIN
 IF (SELECT count(*) FROM public.schema_migrations)<>1 OR NOT EXISTS(
  SELECT 1 FROM public.schema_migrations WHERE (version=32 AND NOT dirty) OR (version=33 AND dirty)) THEN
  RAISE EXCEPTION 'GIS33 requires clean32 or migrator dirty33' USING ERRCODE='55000';
 END IF;
 FOR relation IN SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='public' AND c.relkind IN ('r','p') ORDER BY c.relname COLLATE "C" LOOP
  EXECUTE format('LOCK TABLE public.%I IN ACCESS EXCLUSIVE MODE',relation.relname);
 END LOOP;
END $lock$;
-- Session-local helper leaves no persistent qualification object or seal.
CREATE FUNCTION pg_temp.gis33_layout_identity() RETURNS text LANGUAGE sql STABLE
 SET search_path=pg_catalog,public AS $identity$
WITH objects AS (
 SELECT 'relation' k,c.relname::text n,jsonb_build_array(c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,c.relreplident,c.reloptions,c.relowner=(SELECT relowner FROM pg_class WHERE oid='public.applications'::regclass)) v
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
 UNION ALL
 SELECT 'column',c.relname||'.'||a.attnum,jsonb_build_array(a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,a.attgenerated,a.attisdropped,co.collname,pg_get_expr(d.adbin,d.adrelid))
 FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum LEFT JOIN pg_collation co ON co.oid=a.attcollation
 WHERE n.nspname='public' AND a.attnum>0
 UNION ALL
 SELECT 'constraint',c.relname||'.'||q.conname,jsonb_build_array(q.contype,q.convalidated,q.condeferrable,q.condeferred,pg_get_constraintdef(q.oid,false))
 FROM pg_constraint q JOIN pg_class c ON c.oid=q.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND q.contype<>'n'
 UNION ALL
 SELECT 'index',c.relname,jsonb_build_array(pg_get_indexdef(i.indexrelid),i.indisvalid,i.indisready,i.indislive,i.indisclustered,i.indisreplident)
 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
 UNION ALL
 SELECT 'trigger',c.relname||'.'||t.tgname,jsonb_build_array(t.tgenabled,pg_get_triggerdef(t.oid,false))
 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT t.tgisinternal
 UNION ALL
 SELECT 'constraint_trigger',c.relname||'.'||q.conname||'.'||t.tgtype,
 jsonb_build_array(t.tgenabled,replace(pg_get_triggerdef(t.oid,false),quote_ident(t.tgname),'$constraint_trigger'))
 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 JOIN pg_constraint q ON q.oid=t.tgconstraint WHERE n.nspname='public' AND t.tgisinternal
 UNION ALL
 SELECT 'function',p.proname||'('||pg_get_function_identity_arguments(p.oid)||')',to_jsonb(pg_get_functiondef(p.oid))
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public'
 UNION ALL
 SELECT 'policy',tablename||'.'||policyname,to_jsonb(p)-'schemaname' FROM pg_policies p WHERE schemaname='public'
 UNION ALL
 SELECT 'rule',c.relname||'.'||r.rulename,to_jsonb(pg_get_ruledef(r.oid,false))
 FROM pg_rewrite r JOIN pg_class c ON c.oid=r.ev_class JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
 UNION ALL
 SELECT 'inheritance',c.relname||'.'||i.inhseqno,to_jsonb(i.inhparent::regclass::text)
 FROM pg_inherits i JOIN pg_class c ON c.oid=i.inhrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
 UNION ALL
 SELECT 'sequence',c.relname,jsonb_build_array(format_type(s.seqtypid,NULL),s.seqstart,s.seqincrement,s.seqmax,s.seqmin,s.seqcache,s.seqcycle)
 FROM pg_sequence s JOIN pg_class c ON c.oid=s.seqrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
 UNION ALL
 SELECT 'type',t.typname,jsonb_build_array(t.typtype,format_type(t.typbasetype,t.typtypmod),t.typnotnull,t.typdefault,
 (SELECT jsonb_agg(e.enumlabel ORDER BY e.enumsortorder) FROM pg_enum e WHERE e.enumtypid=t.oid))
 FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typrelid=0 AND t.typelem=0
 UNION ALL
 SELECT 'relation_acl',c.relname||'.'||CASE WHEN a.grantee=c.relowner THEN '$owner' WHEN a.grantee=0 THEN '$public' ELSE pg_get_userbyid(a.grantee)::text END||'.'||a.privilege_type,
 jsonb_build_array(a.is_grantable,a.grantor=c.relowner)
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace,
 LATERAL aclexplode(coalesce(c.relacl,acldefault(CASE WHEN c.relkind='S' THEN 's'::"char" ELSE 'r'::"char" END,c.relowner))) a
 WHERE n.nspname='public' AND a.grantee<>c.relowner
 UNION ALL
 SELECT 'column_acl',c.relname||'.'||column_value.attname||'.'||CASE WHEN a.grantee=c.relowner THEN '$owner' WHEN a.grantee=0 THEN '$public' ELSE pg_get_userbyid(a.grantee)::text END||'.'||a.privilege_type,
 jsonb_build_array(a.is_grantable,a.grantor=c.relowner)
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute column_value ON column_value.attrelid=c.oid,
 LATERAL aclexplode(column_value.attacl) a WHERE n.nspname='public'
 UNION ALL
 SELECT 'function_acl',p.proname||'('||pg_get_function_identity_arguments(p.oid)||').'||CASE WHEN a.grantee=p.proowner THEN '$owner' WHEN a.grantee=0 THEN '$public' ELSE pg_get_userbyid(a.grantee)::text END||'.'||a.privilege_type,
 jsonb_build_array(a.is_grantable,a.grantor=p.proowner,p.proowner=(SELECT relowner FROM pg_class WHERE oid='public.applications'::regclass))
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace,
 LATERAL aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE n.nspname='public' AND a.grantee<>p.proowner
)
SELECT encode(sha256(convert_to(coalesce(jsonb_agg(jsonb_build_array(k,n,v) ORDER BY k COLLATE "C",n COLLATE "C",v::text COLLATE "C")::text,'[]'),'UTF8')),'hex') FROM objects
$identity$;
DO $forward$ DECLARE layout text; floor_before bigint; BEGIN
 -- PG16/18 initialize public under pg_database_owner. Migration3 adds only
 -- runtime USAGE. Compare the complete effective ACL, including grantors and
 -- grant options, so extra or revoked privileges cannot pass as known layout.
 IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname='public'
                AND nspowner='pg_database_owner'::regrole) OR EXISTS (
  WITH actual AS (
   SELECT a.grantee,a.grantor,a.privilege_type,a.is_grantable
   FROM pg_namespace n, LATERAL aclexplode(coalesce(n.nspacl,acldefault('n',n.nspowner))) a
   WHERE n.nspname='public'
  ), expected AS (
   SELECT grantee,'pg_database_owner'::regrole::oid AS grantor,privilege_type,false AS is_grantable
   FROM (VALUES ('pg_database_owner'::regrole::oid,'CREATE'),
                ('pg_database_owner'::regrole::oid,'USAGE'),
                (0::oid,'USAGE'),('gameintegration_runtime'::regrole::oid,'USAGE')) v(grantee,privilege_type)
  )
  (SELECT * FROM actual EXCEPT SELECT * FROM expected)
  UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
 ) THEN
  RAISE EXCEPTION 'GIS33 changed public schema owner or ACL; HOLD' USING ERRCODE='55000';
 END IF;
 IF (SELECT relowner FROM pg_class WHERE oid='public.applications'::regclass)<>current_user::regrole
 OR pg_get_userbyid((SELECT relowner FROM pg_class WHERE oid='public.applications'::regclass))='gameintegration_runtime' THEN
  RAISE EXCEPTION 'GIS33 requires the privileged catalog owner, never runtime ownership; HOLD' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM pg_operator o JOIN pg_namespace n ON n.oid=o.oprnamespace WHERE n.nspname='public')
 OR EXISTS(SELECT 1 FROM pg_collation o JOIN pg_namespace n ON n.oid=o.collnamespace WHERE n.nspname='public')
 OR EXISTS(SELECT 1 FROM pg_conversion o JOIN pg_namespace n ON n.oid=o.connamespace WHERE n.nspname='public')
 OR EXISTS(SELECT 1 FROM pg_opclass o JOIN pg_namespace n ON n.oid=o.opcnamespace WHERE n.nspname='public')
 OR EXISTS(SELECT 1 FROM pg_opfamily o JOIN pg_namespace n ON n.oid=o.opfnamespace WHERE n.nspname='public')
 OR EXISTS(SELECT 1 FROM pg_ts_config o JOIN pg_namespace n ON n.oid=o.cfgnamespace WHERE n.nspname='public')
 OR EXISTS(SELECT 1 FROM pg_ts_dict o JOIN pg_namespace n ON n.oid=o.dictnamespace WHERE n.nspname='public')
 OR EXISTS(SELECT 1 FROM pg_statistic_ext o JOIN pg_namespace n ON n.oid=o.stxnamespace WHERE n.nspname='public') THEN
  RAISE EXCEPTION 'GIS33 unknown source schema objects; HOLD' USING ERRCODE='55000';
 END IF;
 layout:=pg_temp.gis33_layout_identity();
 IF layout NOT IN ('840eac849dc88025e41742668f5f30b2e535cc10eb231c1f7bc041bc4e5d214c','cb51df954611ff3e06ba8869f4e7682d593ea6aadb688341ec8c06d8eeeb458c') THEN
  RAISE EXCEPTION 'GIS33 unknown, partial, or changed source layout; HOLD' USING ERRCODE='55000';
 END IF;
 IF (SELECT count(*) FROM public.gis_authority_revision)<>1 OR NOT EXISTS(
  SELECT 1 FROM public.gis_authority_revision WHERE singleton AND revision>0) THEN
  RAISE EXCEPTION 'GIS33 owning authority floor unavailable; HOLD' USING ERRCODE='55000';
 END IF;
 SELECT revision INTO STRICT floor_before FROM public.gis_authority_revision WHERE singleton;
 IF layout='840eac849dc88025e41742668f5f30b2e535cc10eb231c1f7bc041bc4e5d214c' THEN
  IF EXISTS(SELECT 1 FROM public.gis_sessions WHERE session_status IN ('provisioning','active','closing')) THEN
   RAISE EXCEPTION 'GIS33 live legacy sessions lack owning T32 facts; HOLD for documented recovery' USING ERRCODE='55000';
  END IF;
  -- No host/hash/lease, owner receipt, consent or Native qualification is inferred.
  -- Restored16 derives retention only from the terminal row own updated_at.
  PERFORM pg_catalog.set_config('search_path','public,pg_catalog',true);
  EXECUTE $restore$
-- Restored 000015_t32_session_authority.up.sql; SHA256 41cd6c25408e15f5fe9693fc0a2d29b2224bc032982f9bccc8267a9dae1fee8e
ALTER TABLE gis_sessions
    ADD COLUMN host_profile_id uuid,
    ADD COLUMN roster_hash bytea CHECK (roster_hash IS NULL OR octet_length(roster_hash)=32),
    ADD COLUMN lease_expires_at timestamptz,
    ADD COLUMN roster_receipt_id uuid,
    ADD COLUMN closed_at timestamptz;

ALTER TABLE gis_session_operations
    ADD COLUMN error_outcome text CHECK (error_outcome IN ('transient','permanent','unknown'));

-- The historical last-error migration also used version 14. Preserve the shipped
-- scoped-identity migration and restore its missing nullable outcome here.
UPDATE gis_session_operations SET error_outcome='unknown' WHERE error_code IS NOT NULL;

ALTER TABLE gis_session_operations
    DROP CONSTRAINT gis_session_operations_operation_kind_check,
    ADD CONSTRAINT gis_session_operations_operation_kind_check
        CHECK (operation_kind IN ('create','close','failure','roster'));

CREATE INDEX gis_sessions_roster_lease_due
    ON gis_sessions(lease_expires_at)
    WHERE session_status IN ('provisioning','active') AND lease_expires_at IS NOT NULL;

CREATE TABLE gis_session_roster_updates (
    operation_id uuid NOT NULL,
    session_id uuid NOT NULL,
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
    roster_revision bigint NOT NULL CHECK (roster_revision > 0),
    roster_hash bytea NOT NULL CHECK (octet_length(roster_hash)=32),
    members uuid[] NOT NULL,
    host_profile_id uuid,
    outcome text NOT NULL CHECK (outcome IN ('pending','stale','replayed','applied','failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id,environment_id,operation_id),
    FOREIGN KEY (application_id,environment_id,operation_id)
        REFERENCES gis_session_operations(application_id,environment_id,operation_id),
    FOREIGN KEY (session_id,application_id,environment_id)
        REFERENCES gis_sessions(id,application_id,environment_id)
);

CREATE INDEX gis_session_roster_updates_pending
    ON gis_session_roster_updates(created_at,operation_id)
    WHERE outcome='pending';


-- Restored 000016_t32_session_retention.up.sql; SHA256 3f3f2bf63bd886ce712fce4fd5d5b407d91d4c854b5c09ddf1467e7f3658a113
ALTER TABLE gis_sessions
    ADD COLUMN terminalized_at timestamptz,
    ADD COLUMN history_expires_at timestamptz;

UPDATE gis_sessions
SET terminalized_at=updated_at,
    history_expires_at=CASE WHEN kind='match' THEN updated_at+interval '30 days' ELSE NULL END
WHERE session_status IN ('closed','failed');

CREATE INDEX gis_sessions_retention_due
    ON gis_sessions(terminalized_at,id)
    WHERE session_status IN ('closed','failed') AND kind IN ('match','fleet');

ALTER TABLE gis_session_operations ADD COLUMN terminalized_at timestamptz;
UPDATE gis_session_operations SET terminalized_at=updated_at WHERE status IN ('succeeded','failed');

CREATE FUNCTION gis_session_operation_terminal_clock() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status IN ('succeeded','failed') AND (OLD.status='pending' OR OLD.terminalized_at IS NULL) THEN
        NEW.terminalized_at := clock_timestamp();
    ELSIF NEW.status='pending' THEN
        NEW.terminalized_at := NULL;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER gis_session_operation_terminal_clock
    BEFORE UPDATE OF status ON gis_session_operations
    FOR EACH ROW EXECUTE FUNCTION gis_session_operation_terminal_clock();

-- Deliberately contains no resource IDs, request hashes, receipts, or payload.
-- The external key remains reserved forever after content and receipts are purged.
CREATE TABLE gis_session_external_key_tombstones (
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('party','match','fleet')),
    external_key text NOT NULL,
    tombstoned_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(application_id,environment_id,kind,external_key)
);


-- Restored 000017_t17_conversion_authority.up.sql; SHA256 745685e62e58e6da316519fd1b90d765607bfdf2cc990fad2d43e79f599e4844
ALTER TABLE player_bindings
    ADD COLUMN authority_epoch BIGINT NOT NULL DEFAULT 1 CHECK (authority_epoch > 0),
    ADD COLUMN conversion_operation_id UUID,
    ADD COLUMN pending_target_account_id UUID,
    ADD COLUMN pending_target_profile_id UUID,
    ADD COLUMN active_grant_digest TEXT,
    DROP CONSTRAINT player_bindings_status_check,
    ADD CONSTRAINT player_bindings_status_check
        CHECK (status IN ('pending','active','revoking','revoked','frozen','transferring','retired'));

CREATE TABLE player_binding_conversion_receipts (
    operation_id UUID NOT NULL,
    stage TEXT NOT NULL CHECK (stage IN ('freeze','transfer','activate','retire')),
    transfer_generation INTEGER NOT NULL DEFAULT 0 CHECK (transfer_generation >= 0),
    binding_id UUID NOT NULL REFERENCES player_bindings(binding_id),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
    receipt_bytes BYTEA NOT NULL,
    receipt_hash BYTEA NOT NULL CHECK (octet_length(receipt_hash)=32),
    binding_revision BIGINT NOT NULL CHECK (binding_revision > 0),
    authority_epoch BIGINT NOT NULL CHECK (authority_epoch > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(operation_id,stage,transfer_generation)
);


-- Restored 000018_t51_game_events.up.sql; SHA256 f95858755686e9bd595d5bd25f8d60a70661659e8f17c0281e1bc948e11d8867
CREATE TABLE game_event_nonce_replays (
    credential_id UUID NOT NULL REFERENCES service_credentials(id),
    nonce UUID NOT NULL,
    event_id UUID NOT NULL,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    retain_until TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (credential_id, nonce)
);

CREATE INDEX game_event_nonce_replay_expiry_idx ON game_event_nonce_replays (retain_until);

CREATE TABLE game_event_operations (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    event_id UUID NOT NULL,
    operation_id UUID NOT NULL UNIQUE,
    application_owner_account_id UUID NOT NULL,
    installation_id UUID NOT NULL REFERENCES installations(id),
    bot_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    client_message_id UUID NOT NULL,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    canonical_payload BYTEA NOT NULL CHECK (octet_length(canonical_payload) BETWEEN 1 AND 16384),
    authority_revision BIGINT NOT NULL CHECK (authority_revision > 0),
    status TEXT NOT NULL CHECK (status IN ('queued','processing','succeeded','rejected','expired','cancelled','reconciliation_required')),
    message_id UUID,
    reason_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, event_id),
    FOREIGN KEY (application_id, environment_id) REFERENCES environments(application_id, id),
    CHECK ((status = 'succeeded') = (message_id IS NOT NULL))
);

CREATE INDEX game_event_operations_scope_operation_idx
    ON game_event_operations (application_id, environment_id, operation_id);

CREATE TABLE game_event_publication_outbox (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    event_id UUID NOT NULL,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, event_id),
    FOREIGN KEY (application_id, environment_id, event_id)
        REFERENCES game_event_operations(application_id, environment_id, event_id),
    CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL))
);

CREATE INDEX game_event_publication_claim_idx
    ON game_event_publication_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;


-- Restored 000019_t34_session_consent.up.sql; SHA256 a1c6b47179c75576937c1e68fcc72403e9c7ef6c91b5d3d6d30b36dd4137889d
CREATE TABLE gis_session_consent_receipts (
    operation_id UUID PRIMARY KEY,
    receipt_id UUID NOT NULL UNIQUE,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL REFERENCES gis_sessions(id),
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
    destination_key TEXT NOT NULL CHECK (length(destination_key) BETWEEN 1 AND 256),
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    receipt_bytes BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (session_id, roster_revision, profile_id)
);

CREATE INDEX gis_session_consent_current
    ON gis_session_consent_receipts(session_id, roster_revision, profile_id);


-- Restored 000020_t34_keep_group_operations.up.sql; SHA256 fe1d9fcc946744011519735b04642f83ff0009756898b27dffe48a22009c494b
CREATE TABLE gis_session_keep_group_operations (
    operation_id UUID PRIMARY KEY,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL REFERENCES gis_sessions(id),
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
    destination_key TEXT NOT NULL,
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    target_kind TEXT NOT NULL CHECK (target_kind IN ('new_chat', 'party_chat')),
    chat_id UUID,
    display_name TEXT NOT NULL,
    member_profiles UUID[] NOT NULL,
    create_operation_id UUID NOT NULL,
    create_receipt_id UUID,
    create_request_hash TEXT,
    member_operation_id UUID NOT NULL,
    member_receipt_id UUID,
    member_request_hash TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed')),
    receipt_bytes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((target_kind = 'new_chat') OR chat_id IS NOT NULL),
    CHECK ((status = 'pending' AND receipt_bytes IS NULL) OR (status = 'completed' AND receipt_bytes IS NOT NULL))
);

CREATE INDEX gis_session_keep_group_session_idx
    ON gis_session_keep_group_operations (session_id, created_at DESC);


-- Restored 000021_t37_community_bootstrap.up.sql; SHA256 2179a68401a26e3de534eb8801337818f225ae23fdb71be291962d6c47efaf4e
CREATE TABLE gis_community_bootstrap_operations (
    operation_id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    owner_account_id UUID NOT NULL,
    owner_profile_id UUID NOT NULL,
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    template_id TEXT NOT NULL CHECK (length(template_id) BETWEEN 1 AND 128),
    owner_generation BIGINT NOT NULL DEFAULT 1 CHECK (owner_generation > 0),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'succeeded')),
    operator_account_id UUID,
    approved_at TIMESTAMPTZ,
    approval_expires_at TIMESTAMPTZ,
    space_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'pending' AND operator_account_id IS NULL AND approved_at IS NULL AND approval_expires_at IS NULL AND space_id IS NULL)
        OR (status = 'approved' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND space_id IS NULL)
        OR (status = 'succeeded' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND space_id IS NOT NULL)),
    UNIQUE (application_id, environment_id, corporation_key)
);

CREATE INDEX gis_community_bootstrap_owner_idx
    ON gis_community_bootstrap_operations(owner_account_id, created_at DESC);

CREATE INDEX gis_community_bootstrap_approval_expiry_idx
    ON gis_community_bootstrap_operations(approval_expires_at)
    WHERE status = 'approved';


-- Restored 000022_t37_owner_recovery.up.sql; SHA256 f8d35bce5b5b8fdd17d00d0a6632771d608ba8247b8b1d161921ffd914e493e4
CREATE TABLE gis_community_owner_authority (
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    owner_account_id UUID NOT NULL,
    owner_profile_id UUID NOT NULL,
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'active', 'recovery_pending')),
    space_id UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id, environment_id, corporation_key),
    CHECK ((status = 'pending' AND space_id IS NULL) OR (status IN ('active','recovery_pending') AND space_id IS NOT NULL))
);

CREATE TABLE gis_community_owner_recovery_operations (
    operation_id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    previous_owner_account_id UUID NOT NULL,
    previous_owner_profile_id UUID NOT NULL,
    replacement_account_id UUID NOT NULL,
    replacement_profile_id UUID NOT NULL,
    expected_generation BIGINT NOT NULL CHECK (expected_generation > 0),
    reason_code TEXT NOT NULL CHECK (reason_code IN ('owner_lost', 'corporation_dissolved')),
    evidence_sha256 BYTEA NOT NULL CHECK (octet_length(evidence_sha256) = 32),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'succeeded', 'cancelled')),
    operator_account_id UUID,
    approved_at TIMESTAMPTZ,
    approval_expires_at TIMESTAMPTZ,
    cancelled_by_account_id UUID,
    cancelled_at TIMESTAMPTZ,
    space_id UUID,
    result_generation BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'pending' AND operator_account_id IS NULL AND approved_at IS NULL AND approval_expires_at IS NULL AND space_id IS NULL AND result_generation IS NULL)
        OR (status = 'approved' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND space_id IS NULL AND result_generation IS NULL)
        OR (status = 'succeeded' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND cancelled_by_account_id IS NULL AND cancelled_at IS NULL AND space_id IS NOT NULL AND result_generation = expected_generation + 1)
        OR (status = 'cancelled' AND cancelled_by_account_id IS NOT NULL AND cancelled_at IS NOT NULL AND space_id IS NULL AND result_generation IS NULL))
);

CREATE INDEX gis_community_owner_recovery_current
    ON gis_community_owner_recovery_operations(application_id,environment_id,corporation_key,expected_generation,status);

CREATE UNIQUE INDEX gis_community_owner_recovery_one_open
    ON gis_community_owner_recovery_operations(application_id,environment_id,corporation_key)
    WHERE status IN ('pending','approved');


-- Restored 000023_t38_community_roster.up.sql; SHA256 2330753f252a2b50be8e36d09649dbba88e918352bad3a812731ed1a5a685dd8
ALTER TABLE gis_community_owner_authority
    ADD COLUMN roster_source_revision BIGINT NOT NULL DEFAULT 0 CHECK (roster_source_revision >= 0),
    ADD COLUMN roster_sha256 BYTEA,
    ADD COLUMN roster_lease_expires_at TIMESTAMPTZ,
    ADD CONSTRAINT gis_community_roster_state_consistent CHECK (
        (roster_source_revision = 0 AND roster_sha256 IS NULL AND roster_lease_expires_at IS NULL)
        OR (roster_source_revision > 0 AND octet_length(roster_sha256) = 32 AND roster_lease_expires_at IS NOT NULL)
    );

CREATE TABLE gis_community_roster_snapshots (
    operation_id UUID PRIMARY KEY,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    corporation_key TEXT NOT NULL,
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    snapshot_sha256 BYTEA NOT NULL CHECK (octet_length(snapshot_sha256) = 32),
    page_count INTEGER NOT NULL CHECK (page_count BETWEEN 1 AND 1000),
    member_count INTEGER NOT NULL CHECK (member_count >= 0),
    outcome TEXT NOT NULL CHECK (outcome IN ('accepted', 'stale')),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (application_id, environment_id, corporation_key, owner_generation, source_revision),
    FOREIGN KEY (application_id, environment_id, corporation_key)
        REFERENCES gis_community_owner_authority(application_id, environment_id, corporation_key),
    CHECK ((outcome = 'accepted' AND lease_expires_at IS NOT NULL)
        OR (outcome = 'stale' AND lease_expires_at IS NULL))
);

CREATE TABLE gis_community_roster_pages (
    operation_id UUID NOT NULL REFERENCES gis_community_roster_snapshots(operation_id) ON DELETE CASCADE,
    page_index INTEGER NOT NULL CHECK (page_index >= 0),
    page_sha256 BYTEA NOT NULL CHECK (octet_length(page_sha256) = 32),
    canonical_page_bytes BYTEA NOT NULL,
    PRIMARY KEY (operation_id, page_index)
);

CREATE TABLE gis_community_grants (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    corporation_key TEXT NOT NULL,
    binding_id UUID NOT NULL,
    character_key TEXT NOT NULL CHECK (length(character_key) BETWEEN 1 AND 256),
    source_reason TEXT NOT NULL CHECK (source_reason IN ('membership', 'officer_rank')),
    source_rank TEXT NOT NULL CHECK (source_rank IN ('member', 'officer')),
    source_membership_revision BIGINT NOT NULL CHECK (source_membership_revision > 0),
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    lease_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revocation_reason TEXT CHECK (revocation_reason IS NULL OR revocation_reason IN (
        'snapshot_replaced', 'source_expired', 'binding_revoked', 'owner_generation_changed')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, corporation_key, binding_id, character_key, source_reason, owner_generation),
    FOREIGN KEY (application_id, environment_id, corporation_key)
        REFERENCES gis_community_owner_authority(application_id, environment_id, corporation_key),
    FOREIGN KEY (application_id, environment_id, binding_id)
        REFERENCES player_bindings(application_id, environment_id, binding_id),
    CHECK ((revoked_at IS NULL) = (revocation_reason IS NULL))
);

CREATE INDEX gis_community_grants_effective_idx
    ON gis_community_grants(application_id, environment_id, corporation_key, binding_id, owner_generation)
    WHERE revoked_at IS NULL;

CREATE INDEX gis_community_grants_expiry_idx
    ON gis_community_grants(lease_expires_at, application_id, environment_id)
    WHERE revoked_at IS NULL;


-- Restored 000024_t38_community_roster_space_receipt.up.sql; SHA256 5de5bbfe2b00f9ada647704411fab4462d59460b2791221b8dc4710ed1d2e074
ALTER TABLE gis_community_roster_snapshots
    ADD COLUMN space_projection_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (space_projection_status IN ('pending','applied')),
    ADD COLUMN space_receipt_id UUID,
    ADD COLUMN space_request_hash BYTEA,
    ADD CONSTRAINT gis_community_roster_space_receipt_consistent CHECK (
        (space_projection_status='pending' AND space_receipt_id IS NULL AND space_request_hash IS NULL) OR
        (space_projection_status='applied' AND space_receipt_id IS NOT NULL AND octet_length(space_request_hash)=32)
    );

CREATE INDEX gis_community_roster_projection_pending_idx
    ON gis_community_roster_snapshots(created_at,application_id,environment_id,corporation_key)
    WHERE outcome='accepted' AND space_projection_status='pending';


-- Restored 000025_t39_player_profile_aliases.up.sql; SHA256 03e1ec68edb94f8fb5b3588257a2100f414eddb6353076809961a9176a24d75a
CREATE TABLE gis_player_profile_aliases (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    opaque_reference UUID NOT NULL DEFAULT gen_random_uuid(),
    alias TEXT NOT NULL CHECK (length(alias) BETWEEN 1 AND 256),
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    consented_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    revoked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, profile_id),
    UNIQUE (application_id, environment_id, opaque_reference),
    CHECK ((revoked_at IS NULL) OR revoked_at >= consented_at)
);

CREATE INDEX gis_player_profile_aliases_search_idx
    ON gis_player_profile_aliases(application_id, environment_id, lower(alias))
    WHERE revoked_at IS NULL;


-- Restored 000026_t53_game_action_admission.up.sql; SHA256 5ca42fc0608f295ea3138257b6ad4b4fb56694d377f99854df31dfb782d35bc7
CREATE TABLE gis_game_action_invocations (
  invocation_id UUID PRIMARY KEY,
  account_id UUID NOT NULL,
  profile_id UUID NOT NULL,
  session_id UUID NOT NULL,
  session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
  roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  installation_id UUID NOT NULL,
  message_id UUID NOT NULL,
  chat_id UUID NOT NULL,
  card_revision BIGINT NOT NULL CHECK (card_revision > 0),
  action_id UUID NOT NULL,
  arguments_sha256 BYTEA NOT NULL CHECK (octet_length(arguments_sha256)=32),
  state_version TEXT NOT NULL CHECK (length(state_version) BETWEEN 1 AND 256),
  idempotency_key UUID NOT NULL,
  request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32),
  challenge_sha256 BYTEA NOT NULL UNIQUE CHECK (octet_length(challenge_sha256)=32),
  challenge_secret_ciphertext BYTEA NOT NULL,
  challenge_status TEXT NOT NULL CHECK (challenge_status IN ('unused','consumed','expired','revoked')),
  confirm_idempotency_key UUID,
  confirm_request_sha256 BYTEA CHECK (confirm_request_sha256 IS NULL OR octet_length(confirm_request_sha256)=32),
  expires_at TIMESTAMPTZ NOT NULL,
  operation_id UUID NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  consumed_at TIMESTAMPTZ,
  UNIQUE(account_id,profile_id,idempotency_key)
);

CREATE TABLE gis_game_action_operations (
  operation_id UUID PRIMARY KEY,
  invocation_id UUID NOT NULL UNIQUE REFERENCES gis_game_action_invocations(invocation_id),
  account_id UUID NOT NULL,
  profile_id UUID NOT NULL,
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  installation_id UUID NOT NULL,
  message_id UUID NOT NULL,
  card_revision BIGINT NOT NULL,
  action_id UUID NOT NULL,
  arguments_sha256 BYTEA NOT NULL CHECK (octet_length(arguments_sha256)=32),
  state_version TEXT NOT NULL,
  operation_status TEXT NOT NULL CHECK (operation_status IN ('accepted','cancelled','reconciliation_required')),
  accepted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX gis_game_action_expiry_idx ON gis_game_action_invocations(expires_at)
  WHERE challenge_status='unused';


-- Restored 000027_t54_game_command_outbox.up.sql; SHA256 8d0ddf73004b2896d91078449187776ffa14dbe4856e69353e7e83449f1361e8
ALTER TABLE gis_game_action_invocations
  ADD COLUMN character_binding_id UUID,
  ADD COLUMN binding_revision BIGINT CHECK (binding_revision IS NULL OR binding_revision > 0);

ALTER TABLE gis_game_action_operations
  ADD COLUMN character_binding_id UUID,
  ADD COLUMN binding_revision BIGINT CHECK (binding_revision IS NULL OR binding_revision > 0),
  ADD COLUMN action_type TEXT,
  ADD COLUMN arguments_json JSONB,
  ADD COLUMN command_id UUID UNIQUE;

CREATE TABLE gis_game_command_outbox (
  command_id UUID PRIMARY KEY,
  operation_id UUID NOT NULL UNIQUE REFERENCES gis_game_action_operations(operation_id),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  installation_id UUID NOT NULL,
  callback_url TEXT NOT NULL CHECK (length(callback_url) BETWEEN 1 AND 2048),
  credential_id UUID NOT NULL,
  canonical_body BYTEA NOT NULL CHECK (octet_length(canonical_body) BETWEEN 1 AND 16384),
  body_sha256 BYTEA NOT NULL CHECK (octet_length(body_sha256)=32),
  delivery_status TEXT NOT NULL CHECK (delivery_status IN ('pending','leased','accepted','reconciling','dead_letter')),
  attempt_count SMALLINT NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 6),
  next_attempt_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ,
  terminal_at TIMESTAMPTZ NOT NULL,
  lease_id UUID,
  lease_until TIMESTAMPTZ,
  last_http_status SMALLINT,
  last_error_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  CHECK ((lease_id IS NULL) = (lease_until IS NULL)),
  CHECK ((delivery_status='leased') = (lease_id IS NOT NULL))
);

CREATE INDEX gis_game_command_delivery_idx
  ON gis_game_command_outbox(next_attempt_at, created_at)
  WHERE delivery_status IN ('pending','leased','reconciling');


-- Restored 000028_t55_game_command_execution_permits.up.sql; SHA256 1b558dbe1c68fdf5dcae186e46b5997de153a71d5fbabc51d129ceeffe596d66
CREATE TABLE IF NOT EXISTS gis_game_command_execution_permits (
  permit_id UUID PRIMARY KEY,
  command_id UUID NOT NULL UNIQUE REFERENCES gis_game_command_outbox(command_id),
  operation_id UUID NOT NULL REFERENCES gis_game_action_operations(operation_id),
  binding_id UUID NOT NULL REFERENCES player_bindings(binding_id),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  binding_revision BIGINT NOT NULL CHECK (binding_revision > 0),
  permit_issued_at TIMESTAMPTZ NOT NULL,
  start_before TIMESTAMPTZ NOT NULL,
  complete_before TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('issued', 'committed', 'aborted', 'expired')),
  completion_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  CHECK (start_before > permit_issued_at AND start_before <= permit_issued_at + interval '10 seconds'),
  CHECK (complete_before > start_before AND complete_before <= permit_issued_at + interval '60 seconds'),
  CHECK ((status = 'issued' AND completion_at IS NULL) OR
         (status <> 'issued' AND completion_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS gis_game_command_permit_drain_idx
  ON gis_game_command_execution_permits(binding_id, complete_before)
  WHERE status = 'issued';


-- Restored 000029_t57_game_command_results.up.sql; SHA256 427a4ab6acc8287870a7dea769889d0e394f016cef156f77723b09af8916e19c
ALTER TABLE gis_game_action_operations
  DROP CONSTRAINT gis_game_action_operations_operation_status_check;
ALTER TABLE gis_game_action_operations
  ADD CONSTRAINT gis_game_action_operations_operation_status_check
  CHECK (operation_status IN ('accepted','cancelled','reconciliation_required','succeeded','failed'));

CREATE TABLE gis_game_command_results (
  command_id UUID PRIMARY KEY REFERENCES gis_game_command_outbox(command_id),
  operation_id UUID NOT NULL UNIQUE REFERENCES gis_game_action_operations(operation_id),
  result_id UUID NOT NULL UNIQUE,
  result_status TEXT NOT NULL CHECK (result_status IN ('succeeded','failed')),
  state_version TEXT NOT NULL CHECK (length(state_version) BETWEEN 1 AND 256),
  summary TEXT NOT NULL CHECK (length(summary) BETWEEN 1 AND 512),
  receipt_sha256 BYTEA NOT NULL CHECK (octet_length(receipt_sha256)=32),
  receipt_body BYTEA NOT NULL CHECK (octet_length(receipt_body) BETWEEN 1 AND 4096),
  received_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE OR REPLACE FUNCTION gis_game_command_result_reject_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'game command result receipts are immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER gis_game_command_results_update_guard BEFORE UPDATE ON gis_game_command_results
  FOR EACH ROW EXECUTE FUNCTION gis_game_command_result_reject_mutation();
CREATE TRIGGER gis_game_command_results_delete_guard BEFORE DELETE ON gis_game_command_results
  FOR EACH ROW EXECUTE FUNCTION gis_game_command_result_reject_mutation();
CREATE TRIGGER gis_game_command_results_truncate_guard BEFORE TRUNCATE ON gis_game_command_results
  FOR EACH STATEMENT EXECUTE FUNCTION gis_game_command_result_reject_mutation();

CREATE INDEX gis_game_command_result_operation_idx ON gis_game_command_results(operation_id,received_at);


-- Restored 000030_t57_game_action_projection_outbox.up.sql; SHA256 707ed7328f7b29703f61a03db88bdcadc63e67796133551f390aef44a30c92a9
CREATE TABLE gis_game_action_projection_outbox (
    operation_id UUID PRIMARY KEY REFERENCES gis_game_action_operations(operation_id),
    result_id UUID NOT NULL UNIQUE REFERENCES gis_game_command_results(result_id),
    message_id UUID NOT NULL,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    action_id UUID NOT NULL,
    state_version TEXT NOT NULL,
    result_status TEXT NOT NULL CHECK (result_status IN ('succeeded','failed')),
    safe_summary TEXT NOT NULL,
    delivery_status TEXT NOT NULL DEFAULT 'pending' CHECK (delivery_status IN ('pending','delivered')),
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    lease_id UUID,
    lease_until TIMESTAMPTZ,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    delivered_at TIMESTAMPTZ,
    CHECK ((lease_id IS NULL) = (lease_until IS NULL))
);
CREATE INDEX gis_game_action_projection_due_idx
    ON gis_game_action_projection_outbox(next_attempt_at,created_at)
    WHERE delivery_status='pending';

INSERT INTO gis_game_action_projection_outbox(operation_id,result_id,message_id,application_id,environment_id,action_id,state_version,result_status,safe_summary)
SELECT r.operation_id,r.result_id,o.message_id,o.application_id,o.environment_id,o.action_id,r.state_version,r.result_status,r.summary
FROM gis_game_command_results r JOIN gis_game_action_operations o ON o.operation_id=r.operation_id;

  $restore$;
  PERFORM pg_catalog.set_config('search_path','pg_catalog,public',true);
 END IF;
 IF pg_temp.gis33_layout_identity()<>'cb51df954611ff3e06ba8869f4e7682d593ea6aadb688341ec8c06d8eeeb458c' THEN
  RAISE EXCEPTION 'GIS33 canonical target mismatch; HOLD' USING ERRCODE='55000';
 END IF;
 IF (SELECT count(*) FROM public.gis_authority_revision)<>1 OR NOT EXISTS(
  SELECT 1 FROM public.gis_authority_revision WHERE singleton AND revision>=floor_before) THEN
  RAISE EXCEPTION 'GIS33 authority floor regressed; HOLD' USING ERRCODE='55000';
 END IF;
END $forward$;
DROP FUNCTION pg_temp.gis33_layout_identity();
COMMIT;
