-- Inert admission preparation and immutable operation evidence. None of these
-- rows is an unqualified member_roles assignment or membership assertion.
BEGIN;
CREATE TABLE role_member_source_floors (
 space_id UUID NOT NULL,
 profile_id UUID NOT NULL,
 source_kind TEXT NOT NULL CHECK(source_kind IN ('ORDINARY','COMMUNITY')),
 retired_through BIGINT NOT NULL CHECK(retired_through>0),
 PRIMARY KEY(space_id,profile_id,source_kind)
);
CREATE TABLE role_member_admission_receipts (
 operation_id UUID PRIMARY KEY,
 operation_kind TEXT NOT NULL CHECK(operation_kind IN ('ENSURE','REMOVE')),
 protocol_version INTEGER NOT NULL CHECK(protocol_version=1),
 space_id UUID NOT NULL,
 profile_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),
 binding_sha256 BYTEA NOT NULL CHECK(octet_length(binding_sha256)=32),
 role_id UUID,
 receipt_sha256 BYTEA NOT NULL CHECK(octet_length(receipt_sha256)=32),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 CHECK((operation_kind='ENSURE')=(role_id IS NOT NULL))
);
-- No FK to the live Role row: historical receipts survive role edits/deletion.
CREATE TABLE role_member_admission_preparations (
 space_id UUID NOT NULL,
 profile_id UUID NOT NULL,
 membership_generation BIGINT NOT NULL CHECK(membership_generation>0),
 admission_operation_id UUID NOT NULL UNIQUE REFERENCES role_member_admission_receipts(operation_id),
 role_id UUID NOT NULL,
 receipt_sha256 BYTEA NOT NULL CHECK(octet_length(receipt_sha256)=32),
 PRIMARY KEY(space_id,profile_id,membership_generation)
);
CREATE FUNCTION role_member_admission_receipt_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND EXISTS(SELECT 1 FROM role_space_lifecycle
  WHERE space_id=OLD.space_id AND retired_at IS NOT NULL
  AND clock_timestamp()>=retired_at+interval '30 days') THEN
  RETURN OLD;
 END IF;
 RAISE EXCEPTION 'member admission receipt is immutable' USING ERRCODE='55000';
END $$;
CREATE TRIGGER role_member_admission_receipt_change BEFORE UPDATE OR DELETE ON role_member_admission_receipts
 FOR EACH ROW EXECUTE FUNCTION role_member_admission_receipt_immutable();
CREATE TRIGGER role_member_admission_receipt_truncate BEFORE TRUNCATE ON role_member_admission_receipts
 FOR EACH STATEMENT EXECUTE FUNCTION role_member_admission_receipt_immutable();
CREATE FUNCTION role_member_source_floor_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND EXISTS(SELECT 1 FROM role_space_lifecycle
  WHERE space_id=OLD.space_id AND retired_at IS NOT NULL
  AND clock_timestamp()>=retired_at+interval '30 days') THEN
  RETURN OLD;
 END IF;
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'member source floor cannot be removed' USING ERRCODE='55000';
 END IF;
 IF NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.profile_id IS DISTINCT FROM OLD.profile_id
  OR NEW.source_kind IS DISTINCT FROM OLD.source_kind OR NEW.retired_through<=OLD.retired_through THEN
  RAISE EXCEPTION 'member source floor must increase' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER role_member_source_floor_change BEFORE UPDATE OR DELETE ON role_member_source_floors
 FOR EACH ROW EXECUTE FUNCTION role_member_source_floor_guard();
CREATE TRIGGER role_member_source_floor_truncate BEFORE TRUNCATE ON role_member_source_floors
 FOR EACH STATEMENT EXECUTE FUNCTION role_member_source_floor_guard();
CREATE TRIGGER role_authority_revision_member_source_floors AFTER INSERT OR UPDATE OR DELETE ON role_member_source_floors
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_member_preparations AFTER INSERT OR UPDATE OR DELETE ON role_member_admission_preparations
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_member_preparations_truncate BEFORE TRUNCATE ON role_member_admission_preparations
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
-- Explicit current authorized contributions and owner writer evidence. These
-- tables do not infer provenance from preexisting member_roles rows. Existing
-- legacy evidence stays unresolved until a normal protected writer reconciles.
CREATE TABLE role_member_contributions (
 id UUID PRIMARY KEY,
 authorization_id UUID NOT NULL,
 space_id UUID NOT NULL,
 profile_id UUID NOT NULL,
 role_id UUID NOT NULL,
 source_binding BYTEA NOT NULL,
 source_kind TEXT NOT NULL CHECK(source_kind IN ('ORDINARY','COMMUNITY')),
 membership_generation BIGINT NOT NULL CHECK(membership_generation>=0),
 owner_generation BIGINT NOT NULL CHECK(owner_generation>=0),
 CHECK(source_kind='COMMUNITY' OR owner_generation=0),
 CHECK(source_kind='ORDINARY' OR (membership_generation>0 AND owner_generation>0))
);
CREATE INDEX role_member_contributions_subject ON role_member_contributions(space_id,profile_id,role_id);
CREATE FUNCTION role_member_contribution_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP<>'INSERT' THEN RAISE EXCEPTION 'qualified contribution identity is immutable' USING ERRCODE='55000'; END IF;
 IF NOT EXISTS(SELECT 1 FROM roles WHERE id=NEW.role_id AND space_id=NEW.space_id AND NOT(is_system AND name='Owner')) THEN
  RAISE EXCEPTION 'qualified contribution role unavailable' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER role_member_contribution_insert BEFORE INSERT OR UPDATE ON role_member_contributions FOR EACH ROW EXECUTE FUNCTION role_member_contribution_guard();
CREATE TRIGGER role_member_contribution_truncate BEFORE TRUNCATE ON role_member_contributions FOR EACH STATEMENT EXECUTE FUNCTION role_member_contribution_guard();
CREATE TRIGGER role_authority_revision_member_contributions AFTER INSERT OR DELETE ON role_member_contributions FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TABLE role_member_owner_authority (
 space_id UUID PRIMARY KEY,
 profile_id UUID NOT NULL,
 role_id UUID NOT NULL,
 origin TEXT NOT NULL CHECK(origin IN ('BOOTSTRAP','TRANSFER')),
 generation BIGINT NOT NULL CHECK(generation>0),
 operation_id UUID,
 CHECK((origin='TRANSFER')=(operation_id IS NOT NULL))
);
CREATE FUNCTION role_member_owner_authority_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=OLD.space_id AND retired_at IS NOT NULL AND clock_timestamp()>=retired_at+interval '30 days') THEN RETURN OLD; END IF;
 IF TG_OP IN ('DELETE','TRUNCATE') THEN RAISE EXCEPTION 'owner authority cannot be erased' USING ERRCODE='55000'; END IF;
 IF NOT EXISTS(SELECT 1 FROM roles WHERE id=NEW.role_id AND space_id=NEW.space_id AND is_system AND name='Owner') THEN RAISE EXCEPTION 'canonical Owner role required' USING ERRCODE='55000'; END IF;
 IF TG_OP='UPDATE' AND (NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.role_id IS DISTINCT FROM OLD.role_id OR NEW.generation<>OLD.generation+1 OR NEW.origin<>'TRANSFER') THEN RAISE EXCEPTION 'owner authority must advance through transfer' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER role_member_owner_authority_change BEFORE INSERT OR UPDATE OR DELETE ON role_member_owner_authority FOR EACH ROW EXECUTE FUNCTION role_member_owner_authority_guard();
CREATE TRIGGER role_member_owner_authority_truncate BEFORE TRUNCATE ON role_member_owner_authority FOR EACH STATEMENT EXECUTE FUNCTION role_member_owner_authority_guard();
CREATE TRIGGER role_authority_revision_member_owner_authority AFTER INSERT OR UPDATE OR DELETE ON role_member_owner_authority FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
COMMIT;
