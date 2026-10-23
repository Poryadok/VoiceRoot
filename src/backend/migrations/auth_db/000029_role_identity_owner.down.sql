-- Once compact owner history exists, rollback cannot destroy privacy fences or
-- reinterpret an admissible pre-erasure backup as an empty owner history.
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM role_identity_lifetimes)
      OR EXISTS(SELECT 1 FROM role_identity_owner_journal) THEN
    RAISE EXCEPTION 'identity owner history cannot be rolled back';
  END IF;
END $$;
DROP TRIGGER auth_role_identity_account_guard ON accounts;
DROP TRIGGER auth_role_identity_account_created ON accounts;
DROP TRIGGER role_identity_journal_immutable ON role_identity_owner_journal;
DROP FUNCTION auth_role_identity_account_guard();
DROP FUNCTION auth_role_identity_account_created();
DROP FUNCTION role_identity_journal_immutable();
DROP FUNCTION role_identity_materialize_handle(uuid);
DROP FUNCTION role_identity_register_keyset(bigint,integer,jsonb);
DROP FUNCTION role_identity_append(text,uuid);
ALTER TABLE accounts DROP COLUMN role_identity_keyset_revision;
ALTER TABLE accounts DROP COLUMN role_identity_account_selector;
ALTER TABLE accounts DROP COLUMN role_identity_selector_version;
ALTER TABLE accounts DROP COLUMN role_identity_handle;
DROP TABLE role_identity_account_selectors;
DROP TABLE role_identity_lifetimes;
DROP TABLE role_identity_owner_journal;
DROP TABLE role_identity_key_versions;
DROP TABLE role_identity_owner_state;
