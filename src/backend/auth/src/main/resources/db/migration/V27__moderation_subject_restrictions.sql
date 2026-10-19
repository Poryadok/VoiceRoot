-- Auth owns the opaque anti-abuse subject. It is never a public account ID.
ALTER TABLE accounts ADD COLUMN moderation_subject_id uuid DEFAULT gen_random_uuid();
ALTER TABLE accounts ADD COLUMN moderation_lifecycle_revision bigint NOT NULL DEFAULT 1 CHECK (moderation_lifecycle_revision > 0);
ALTER TABLE accounts ADD COLUMN moderation_deletion_cycle_id uuid;
ALTER TABLE accounts ADD COLUMN moderation_purge_at timestamptz;
CREATE UNIQUE INDEX accounts_moderation_subject_idx ON accounts(moderation_subject_id) WHERE moderation_subject_id IS NOT NULL;
UPDATE accounts SET moderation_deletion_cycle_id=gen_random_uuid(),moderation_purge_at=deleted_at+interval '30 days' WHERE deleted_at IS NOT NULL;

-- Survives account erasure; contains no account/profile mapping or content.
CREATE TABLE moderation_terminal_subjects (
  subject_id uuid PRIMARY KEY,
  purged_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE moderation_account_restrictions (
  account_id uuid PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  subject_id uuid NOT NULL UNIQUE,
  desired_revision bigint NOT NULL DEFAULT 0 CHECK (desired_revision >= 0),
  desired_banned boolean NOT NULL DEFAULT false,
  -- Unknown existing suspension is independent from this new projection.
  legacy_suspended boolean NOT NULL DEFAULT false
);
INSERT INTO moderation_account_restrictions(account_id,subject_id,legacy_suspended)
SELECT id,moderation_subject_id,status='suspended' FROM accounts;

CREATE FUNCTION auth_moderation_account_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE restricted boolean;
BEGIN
  IF TG_OP='DELETE' THEN
    IF OLD.moderation_subject_id IS NOT NULL THEN
      INSERT INTO moderation_terminal_subjects(subject_id) VALUES(OLD.moderation_subject_id) ON CONFLICT DO NOTHING;
    END IF;
    RETURN OLD;
  END IF;
  IF TG_OP='INSERT' AND NEW.deleted_at IS NOT NULL THEN
    NEW.moderation_deletion_cycle_id := gen_random_uuid();
    NEW.moderation_purge_at := NEW.deleted_at + interval '30 days';
  END IF;
  IF TG_OP='UPDATE' THEN
    IF NEW.moderation_subject_id IS NULL AND OLD.moderation_subject_id IS NOT NULL AND
       (OLD.moderation_purge_at <= clock_timestamp() OR EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=OLD.moderation_subject_id)) THEN
      -- Also fences a restored backup whose account row predates deletion.
      -- Imported terminal authority wins over that stale live row.
      NEW.status := 'deleted';
      NEW.deleted_at := COALESCE(OLD.deleted_at,clock_timestamp());
      NEW.moderation_lifecycle_revision := OLD.moderation_lifecycle_revision + 1;
      NEW.moderation_deletion_cycle_id := NULL;
      NEW.moderation_purge_at := NULL;
      RETURN NEW;
    END IF;
    IF NEW.moderation_subject_id IS DISTINCT FROM OLD.moderation_subject_id AND
       NOT (NEW.moderation_subject_id IS NULL AND OLD.moderation_purge_at <= clock_timestamp()) THEN
      RAISE EXCEPTION 'moderation subject is immutable';
    END IF;
    IF NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
      IF OLD.moderation_subject_id IS NULL OR EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=OLD.moderation_subject_id) OR
          OLD.moderation_purge_at <= clock_timestamp() THEN
        RAISE EXCEPTION 'terminal moderation subject cannot be restored';
      END IF;
      NEW.moderation_lifecycle_revision := OLD.moderation_lifecycle_revision + 1;
      IF NEW.deleted_at IS NULL THEN
        NEW.moderation_deletion_cycle_id := NULL;
        NEW.moderation_purge_at := NULL;
      ELSE
        NEW.moderation_deletion_cycle_id := gen_random_uuid();
        NEW.moderation_purge_at := NEW.deleted_at + interval '30 days';
      END IF;
    ELSIF NEW.moderation_lifecycle_revision IS DISTINCT FROM OLD.moderation_lifecycle_revision OR
        NEW.moderation_deletion_cycle_id IS DISTINCT FROM OLD.moderation_deletion_cycle_id OR
        NEW.moderation_purge_at IS DISTINCT FROM OLD.moderation_purge_at THEN
      RAISE EXCEPTION 'moderation lifecycle is Auth-owned';
    END IF;
    IF NEW.status='suspended' AND OLD.status IS DISTINCT FROM 'suspended' AND
       current_setting('voice.moderation_projection',true) IS DISTINCT FROM 'on' THEN
      UPDATE moderation_account_restrictions SET legacy_suspended=true WHERE account_id=OLD.id;
    END IF;
  END IF;
  IF NEW.deleted_at IS NOT NULL THEN
    NEW.status := 'deleted';
  ELSIF NEW.moderation_subject_id IS NULL OR EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=NEW.moderation_subject_id) THEN
    RAISE EXCEPTION 'terminal moderation subject cannot become active';
  ELSE
    SELECT desired_banned OR legacy_suspended INTO restricted FROM moderation_account_restrictions WHERE account_id=NEW.id;
    IF COALESCE(restricted,false) THEN NEW.status := 'suspended'; END IF;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER auth_moderation_account_guard BEFORE INSERT OR UPDATE OR DELETE ON accounts FOR EACH ROW EXECUTE FUNCTION auth_moderation_account_guard();

CREATE FUNCTION auth_moderation_account_created() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO moderation_account_restrictions(account_id,subject_id,legacy_suspended)
  VALUES(NEW.id,NEW.moderation_subject_id,NEW.status='suspended');
  RETURN NEW;
END $$;
CREATE TRIGGER auth_moderation_account_created AFTER INSERT ON accounts FOR EACH ROW EXECUTE FUNCTION auth_moderation_account_created();

-- One revision represents the complete current effective Moderation ban set.
-- Equal-revision/different-value is a conflict, never a last-writer-wins clear.
CREATE FUNCTION auth_reconcile_moderation(account uuid, subject uuid, revision bigint, banned boolean)
RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE a accounts%ROWTYPE; r moderation_account_restrictions%ROWTYPE; previous_setting text;
BEGIN
  IF revision < 1 THEN RAISE EXCEPTION 'invalid moderation revision'; END IF;
  SELECT * INTO a FROM accounts WHERE id=account FOR UPDATE;
  IF NOT FOUND OR a.moderation_subject_id IS DISTINCT FROM subject OR
      a.moderation_purge_at <= clock_timestamp() OR
      EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=subject) THEN
    RAISE EXCEPTION 'terminal or unknown moderation subject';
  END IF;
  SELECT * INTO STRICT r FROM moderation_account_restrictions WHERE account_id=account FOR UPDATE;
  IF revision < r.desired_revision THEN RETURN r.desired_revision; END IF;
  IF revision = r.desired_revision THEN
    IF banned IS DISTINCT FROM r.desired_banned THEN RAISE EXCEPTION 'moderation revision conflict'; END IF;
    RETURN r.desired_revision;
  END IF;
  UPDATE moderation_account_restrictions SET desired_revision=revision,desired_banned=banned WHERE account_id=account;
  previous_setting := current_setting('voice.moderation_projection',true);
  PERFORM set_config('voice.moderation_projection','on',true);
  UPDATE accounts SET status=CASE WHEN deleted_at IS NOT NULL THEN 'deleted'
                                 WHEN banned OR r.legacy_suspended THEN 'suspended' ELSE 'active' END,
                      updated_at=clock_timestamp() WHERE id=account;
  PERFORM set_config('voice.moderation_projection',COALESCE(previous_setting,'off'),true);
  RETURN revision;
END $$;
