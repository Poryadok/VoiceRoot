-- Space-issued membership incarnations are permanent anti-ABA heads, not a
-- projection of mutable timestamps. No cascading FK may erase their floors.
CREATE TABLE space_ordinary_member_heads (
    space_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation >= 0),
    retired_through BIGINT NOT NULL DEFAULT 0 CHECK (retired_through >= 0),
    publication_id UUID NOT NULL CHECK (publication_id <> '00000000-0000-0000-0000-000000000000'),
    state TEXT NOT NULL CHECK (state IN ('NONE','PREPARING','ACTIVE','RETIRED')),
    legacy BOOLEAN NOT NULL DEFAULT false,
    account_id UUID,
    admission_binding_sha256 BYTEA,
    role_receipt_sha256 BYTEA,
    publication_kind SMALLINT NOT NULL DEFAULT 0 CHECK(publication_kind BETWEEN 0 AND 6),
    native_publication_sha256 BYTEA,
    PRIMARY KEY (space_id,profile_id),
    CHECK (NOT legacy OR (generation=0 AND state='ACTIVE')),
    CHECK (admission_binding_sha256 IS NULL OR octet_length(admission_binding_sha256)=32),
    CHECK (role_receipt_sha256 IS NULL OR octet_length(role_receipt_sha256)=32),
    CHECK (generation>=retired_through)
);

CREATE TABLE space_community_member_heads (
    space_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation>0),
    retired_through BIGINT NOT NULL DEFAULT 0 CHECK (retired_through>=0),
    owner_generation BIGINT NOT NULL CHECK (owner_generation>0),
    episode_id UUID NOT NULL CHECK (episode_id <> '00000000-0000-0000-0000-000000000000'),
    state TEXT NOT NULL CHECK (state IN ('ACTIVE','RETIRED')),
    PRIMARY KEY (space_id,profile_id),
    CHECK (generation>=retired_through)
);

CREATE TABLE space_member_admission_intents (
 operation_id UUID PRIMARY KEY,
 space_id UUID NOT NULL, profile_id UUID NOT NULL, account_id UUID NOT NULL,
 invite_id UUID, guest BOOLEAN NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 binding_sha256 BYTEA NOT NULL CHECK(octet_length(binding_sha256)=32),
 policy_version BIGINT NOT NULL CHECK(policy_version>0),
 state TEXT NOT NULL CHECK(state IN ('PREPARING','PUBLISHED','RETIRED')),
 role_id UUID, role_receipt_sha256 BYTEA, published_at TIMESTAMPTZ,
 CHECK ((state='PUBLISHED')=(role_id IS NOT NULL AND role_receipt_sha256 IS NOT NULL AND published_at IS NOT NULL)),
 CHECK (role_receipt_sha256 IS NULL OR octet_length(role_receipt_sha256)=32)
);
CREATE INDEX space_member_admission_subject ON space_member_admission_intents(space_id,profile_id);
CREATE TABLE space_member_admission_outbox (
 event_id UUID PRIMARY KEY, operation_id UUID NOT NULL UNIQUE,
 space_id UUID NOT NULL, profile_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0), occurred_at TIMESTAMPTZ NOT NULL,
 delivered_at TIMESTAMPTZ
);
CREATE TABLE space_member_role_cleanup (
 operation_id UUID PRIMARY KEY, space_id UUID NOT NULL, profile_id UUID NOT NULL,
 removal_floor BIGINT NOT NULL CHECK(removal_floor>0), completed_at TIMESTAMPTZ
);
CREATE TABLE space_membership_ownership_heads (
 space_id UUID PRIMARY KEY, generation BIGINT NOT NULL CHECK(generation>0),
 owner_profile_id UUID, state TEXT NOT NULL CHECK(state IN ('ACTIVE','RETIRED')),
 CHECK ((state='ACTIVE')=(owner_profile_id IS NOT NULL))
);
INSERT INTO space_membership_ownership_heads(space_id,generation,owner_profile_id,state)
 SELECT s.id,1,s.owner_profile_id,'ACTIVE' FROM spaces s
 WHERE NOT EXISTS(SELECT 1 FROM ownership_journal j WHERE j.space_id=s.id AND j.state NOT IN ('completed','aborted'))
 AND NOT EXISTS(SELECT 1 FROM community_owner_authority a WHERE a.space_id=s.id AND a.status<>'active');
CREATE TABLE space_native_member_publications (
 operation_id UUID PRIMARY KEY,space_id UUID NOT NULL,profile_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),kind SMALLINT NOT NULL CHECK(kind BETWEEN 3 AND 6),
 binding_bytes BYTEA NOT NULL,binding_sha256 BYTEA NOT NULL CHECK(octet_length(binding_sha256)=32),
 state TEXT NOT NULL CHECK(state IN ('RESERVED','PUBLISHED','INVALIDATED')),published_at TIMESTAMPTZ
);
CREATE FUNCTION space_membership_ownership_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  INSERT INTO space_membership_ownership_heads(space_id,generation,owner_profile_id,state)
   VALUES(NEW.id,1,NEW.owner_profile_id,'ACTIVE') ON CONFLICT(space_id) DO NOTHING;
  RETURN NEW;
 ELSIF TG_OP='DELETE' THEN
  UPDATE space_membership_ownership_heads SET generation=generation+1,owner_profile_id=NULL,state='RETIRED' WHERE space_id=OLD.id;
  RETURN OLD;
 ELSIF NEW.owner_profile_id IS DISTINCT FROM OLD.owner_profile_id THEN
  UPDATE space_membership_ownership_heads SET generation=generation+1,owner_profile_id=NEW.owner_profile_id
   WHERE space_id=NEW.id AND state='ACTIVE' AND owner_profile_id=OLD.owner_profile_id;
  IF NOT FOUND THEN RAISE EXCEPTION 'known ownership authority required' USING ERRCODE='55000'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER space_membership_ownership_changed AFTER INSERT OR UPDATE OF owner_profile_id OR DELETE ON spaces
 FOR EACH ROW EXECUTE FUNCTION space_membership_ownership_changed();

-- These are explicit existing publications. No Role readiness receipt is
-- synthesized for them, and reads never invent another publication UUID.
INSERT INTO space_ordinary_member_heads(space_id,profile_id,generation,publication_id,state,legacy,publication_kind)
SELECT space_id,profile_id,0,gen_random_uuid(),'ACTIVE',true,2 FROM space_members;

INSERT INTO space_community_member_heads(space_id,profile_id,generation,owner_generation,episode_id,state)
SELECT m.space_id,m.profile_id,1,m.owner_generation,gen_random_uuid(),'ACTIVE'
FROM community_roster_members m JOIN community_owner_authority a
ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation
WHERE a.status='active' AND m.revoked_at IS NULL
AND m.lease_expires_at>clock_timestamp() AND a.roster_lease_expires_at>clock_timestamp();

ALTER TABLE space_members ADD COLUMN membership_generation BIGINT,
    ADD COLUMN admission_operation_id UUID;
UPDATE space_members m SET membership_generation=h.generation,admission_operation_id=h.publication_id
FROM space_ordinary_member_heads h WHERE h.space_id=m.space_id AND h.profile_id=m.profile_id;

CREATE FUNCTION space_member_head_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'permanent membership head cannot be erased' USING ERRCODE='55000';
 END IF;
 IF NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.profile_id IS DISTINCT FROM OLD.profile_id
 OR NEW.generation<OLD.generation OR NEW.retired_through<OLD.retired_through THEN
  RAISE EXCEPTION 'membership identity or floor cannot regress' USING ERRCODE='55000';
 END IF;
 IF NEW.generation=OLD.generation THEN
  IF TG_TABLE_NAME='space_ordinary_member_heads' THEN
   IF NEW.publication_id IS DISTINCT FROM OLD.publication_id OR NEW.legacy IS DISTINCT FROM OLD.legacy
   OR (OLD.state='RETIRED' AND NEW.state<>'RETIRED') THEN
    RAISE EXCEPTION 'ordinary membership incarnation cannot be rebound' USING ERRCODE='55000';
   END IF;
  ELSIF NEW.episode_id IS DISTINCT FROM OLD.episode_id OR NEW.owner_generation IS DISTINCT FROM OLD.owner_generation
   OR (OLD.state='RETIRED' AND NEW.state<>'RETIRED') THEN
   RAISE EXCEPTION 'community membership incarnation cannot be rebound' USING ERRCODE='55000';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER ordinary_member_head_guard BEFORE UPDATE OR DELETE ON space_ordinary_member_heads
FOR EACH ROW EXECUTE FUNCTION space_member_head_guard();
CREATE TRIGGER ordinary_member_head_truncate BEFORE TRUNCATE ON space_ordinary_member_heads
FOR EACH STATEMENT EXECUTE FUNCTION space_member_head_guard();
CREATE TRIGGER community_member_head_guard BEFORE UPDATE OR DELETE ON space_community_member_heads
FOR EACH ROW EXECUTE FUNCTION space_member_head_guard();
CREATE TRIGGER community_member_head_truncate BEFORE TRUNCATE ON space_community_member_heads
FOR EACH STATEMENT EXECUTE FUNCTION space_member_head_guard();

CREATE FUNCTION space_ordinary_publication_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE head space_ordinary_member_heads%ROWTYPE;
BEGIN
 IF TG_OP='DELETE' THEN
  UPDATE space_ordinary_member_heads SET generation=generation+1,retired_through=generation+1,
    state='RETIRED',legacy=false WHERE space_id=OLD.space_id AND profile_id=OLD.profile_id;
  UPDATE space_member_admission_intents SET state='RETIRED'
   WHERE space_id=OLD.space_id AND profile_id=OLD.profile_id AND state='PREPARING';
  INSERT INTO space_member_role_cleanup(operation_id,space_id,profile_id,removal_floor)
   SELECT gen_random_uuid(),space_id,profile_id,retired_through FROM space_ordinary_member_heads
   WHERE space_id=OLD.space_id AND profile_id=OLD.profile_id;
  RETURN OLD;
 END IF;
 IF TG_OP='UPDATE' AND (NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.profile_id IS DISTINCT FROM OLD.profile_id
 OR NEW.membership_generation IS DISTINCT FROM OLD.membership_generation
 OR NEW.admission_operation_id IS DISTINCT FROM OLD.admission_operation_id) THEN
  RAISE EXCEPTION 'published membership binding is immutable' USING ERRCODE='55000';
 END IF;
 SELECT * INTO head FROM space_ordinary_member_heads WHERE space_id=NEW.space_id AND profile_id=NEW.profile_id;
 IF NOT FOUND OR head.state<>'ACTIVE' OR NEW.membership_generation IS DISTINCT FROM head.generation
 OR NEW.admission_operation_id IS DISTINCT FROM head.publication_id THEN
  RAISE EXCEPTION 'exact published membership authority required' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER space_ordinary_publication_guard BEFORE INSERT OR UPDATE OR DELETE ON space_members
FOR EACH ROW EXECUTE FUNCTION space_ordinary_publication_guard();

CREATE FUNCTION space_community_episode_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE head space_community_member_heads%ROWTYPE; live_owner BOOLEAN; continuous BOOLEAN;
BEGIN
 IF TG_OP='DELETE' THEN
  UPDATE space_community_member_heads SET retired_through=generation,state='RETIRED'
   WHERE space_id=OLD.space_id AND profile_id=OLD.profile_id;
  RETURN OLD;
 END IF;
 IF TG_OP='UPDATE' AND (NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.profile_id IS DISTINCT FROM OLD.profile_id) THEN
  RAISE EXCEPTION 'community member scope is immutable' USING ERRCODE='55000';
 END IF;
 SELECT * INTO head FROM space_community_member_heads WHERE space_id=NEW.space_id AND profile_id=NEW.profile_id FOR UPDATE;
 SELECT EXISTS(SELECT 1 FROM community_owner_authority WHERE space_id=NEW.space_id
  AND owner_generation=NEW.owner_generation AND status='active' AND roster_lease_expires_at>clock_timestamp()) INTO live_owner;
 continuous:=TG_OP='UPDATE' AND head.state='ACTIVE' AND OLD.owner_generation=NEW.owner_generation
  AND head.owner_generation=NEW.owner_generation AND OLD.revoked_at IS NULL
  AND OLD.lease_expires_at>clock_timestamp() AND live_owner;
 IF NEW.revoked_at IS NOT NULL OR NEW.lease_expires_at<=clock_timestamp() THEN
  UPDATE space_community_member_heads SET state='RETIRED',retired_through=generation
   WHERE space_id=NEW.space_id AND profile_id=NEW.profile_id;
 ELSIF NOT continuous THEN
  INSERT INTO space_community_member_heads(space_id,profile_id,generation,owner_generation,episode_id,state)
  VALUES(NEW.space_id,NEW.profile_id,1,NEW.owner_generation,gen_random_uuid(),'ACTIVE')
  ON CONFLICT(space_id,profile_id) DO UPDATE SET
   generation=space_community_member_heads.generation+1,
   retired_through=space_community_member_heads.generation,
   owner_generation=EXCLUDED.owner_generation,episode_id=EXCLUDED.episode_id,state='ACTIVE';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER space_community_episode_changed AFTER INSERT OR UPDATE OR DELETE ON community_roster_members
FOR EACH ROW EXECUTE FUNCTION space_community_episode_changed();

CREATE FUNCTION space_community_owner_episode_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  UPDATE space_community_member_heads SET state='RETIRED',retired_through=generation WHERE space_id=OLD.space_id;
  RETURN OLD;
 END IF;
 IF NEW.owner_generation IS DISTINCT FROM OLD.owner_generation OR NEW.status<>'active'
 OR NEW.roster_lease_expires_at<=clock_timestamp() THEN
  UPDATE space_community_member_heads SET state='RETIRED',retired_through=generation WHERE space_id=NEW.space_id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER space_community_owner_episode_changed AFTER UPDATE OR DELETE ON community_owner_authority
FOR EACH ROW EXECUTE FUNCTION space_community_owner_episode_changed();

-- Existing databases without the unrelated Federation-source preflight still
-- get permanent heads. Full production migration order supplies its revision
-- function; new inputs participate in exactly that same monotonic Space clock.
DO $$ DECLARE name TEXT; BEGIN
 IF to_regprocedure('space_authority_revision_changed()') IS NOT NULL THEN
  FOREACH name IN ARRAY ARRAY['space_ordinary_member_heads','space_community_member_heads','space_entry_policies','space_member_admission_intents','space_membership_ownership_heads','space_native_member_publications'] LOOP
   EXECUTE format('CREATE TRIGGER %I AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed(''space_id'')','space_authority_revision_'||name,name);
   EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON %I FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard()','space_authority_revision_'||name||'_truncate',name);
  END LOOP;
 END IF;
END $$;
