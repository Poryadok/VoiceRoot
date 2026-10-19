-- Run with psql -v ON_ERROR_STOP=1 against the native V28 fixture after the
-- five RoleIdentityOwnerJdbcIntegrationTest cases. Everything is rolled back.
-- Dedicated fixture key fingerprints only; no actual selector secret enters PG.
BEGIN;
DO $$
DECLARE first_fp text := encode(sha256(decode(repeat('13',32),'hex')),'hex');
        second_fp text := encode(sha256(decode(repeat('14',32),'hex')),'hex');
        first_keys jsonb; both_keys jsonb; handle uuid := gen_random_uuid();
        account_id uuid := gen_random_uuid(); before_revision bigint; after_revision bigint;
        rejected boolean;
BEGIN
  first_keys := jsonb_build_array(jsonb_build_object('version',1,'fingerprint',first_fp));
  both_keys := first_keys || jsonb_build_array(jsonb_build_object('version',2,'fingerprint',second_fp));
  PERFORM role_identity_register_keyset(1,1,first_keys);
  SELECT through_revision INTO before_revision FROM role_identity_owner_state;
  PERFORM role_identity_register_keyset(1,1,first_keys);
  SELECT through_revision INTO after_revision FROM role_identity_owner_state;
  IF after_revision<>before_revision THEN RAISE EXCEPTION 'exact keyset retry changed the cut'; END IF;

  PERFORM role_identity_register_keyset(2,2,both_keys);
  rejected := false;
  BEGIN
    PERFORM role_identity_register_keyset(3,2,jsonb_build_array(
      jsonb_build_object('version',2,'fingerprint',second_fp)));
  EXCEPTION WHEN raise_exception THEN
    IF SQLERRM<>'identity selector keyset unavailable' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'retained key version was dropped'; END IF;

  rejected := false;
  BEGIN
    PERFORM role_identity_register_keyset(3,1,jsonb_build_array(
      jsonb_build_object('version',1,'fingerprint',second_fp),
      jsonb_build_object('version',2,'fingerprint',first_fp)));
  EXCEPTION WHEN raise_exception THEN
    IF SQLERRM<>'identity selector keyset unavailable' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'historical key fingerprint was replaced'; END IF;

  rejected := false;
  BEGIN
    PERFORM role_identity_register_keyset(1,1,first_keys);
  EXCEPTION WHEN raise_exception THEN
    IF SQLERRM<>'identity selector keyset is stale' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'old keyset revision was accepted'; END IF;

  rejected := false;
  BEGIN
    INSERT INTO accounts(id,email,password_hash,type,status,role_identity_handle,
      role_identity_selector_version,role_identity_account_selector,role_identity_keyset_revision)
    VALUES(account_id,account_id||'@example.invalid','fixture','regular','active',handle,
      1,sha256(uuid_send(account_id)),1);
  EXCEPTION WHEN raise_exception THEN
    IF SQLERRM<>'identity selector keyset is stale' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'birth used stale selector capture'; END IF;

  rejected := false;
  BEGIN
    INSERT INTO accounts(email,password_hash,type,status,role_identity_selector_version)
    VALUES(gen_random_uuid()||'@example.invalid','fixture','regular','active',2);
  EXCEPTION WHEN check_violation THEN
    IF SQLERRM NOT LIKE '%accounts_role_identity_capture_shape%' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'partial capture tuple bypassed shape check'; END IF;

  INSERT INTO accounts(id,email,password_hash,type,status,role_identity_handle,
    role_identity_selector_version,role_identity_account_selector,role_identity_keyset_revision)
  VALUES(account_id,account_id||'@example.invalid','fixture','regular','active',handle,
    2,sha256(uuid_send(account_id)),2);
  IF NOT EXISTS(SELECT 1 FROM role_identity_account_selectors
      WHERE owner_handle=handle AND key_version=2) THEN
    RAISE EXCEPTION 'current keyset birth did not retain its selector';
  END IF;

  rejected := false;
  BEGIN
    DELETE FROM accounts WHERE id=account_id;
  EXCEPTION WHEN raise_exception THEN
    IF SQLERRM<>'account erasure boundary not reached' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'active account was erased without boundary'; END IF;

  rejected := false;
  BEGIN
    UPDATE role_identity_owner_journal SET event_kind='rewritten' WHERE revision=1;
  EXCEPTION WHEN raise_exception THEN
    IF SQLERRM<>'identity owner journal is append-only' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'journal update accepted'; END IF;

  rejected := false;
  BEGIN
    DELETE FROM role_identity_owner_journal WHERE revision=1;
  EXCEPTION WHEN raise_exception THEN
    IF SQLERRM<>'identity owner journal is append-only' THEN RAISE; END IF;
    rejected := true;
  END;
  IF NOT rejected THEN RAISE EXCEPTION 'journal deletion accepted'; END IF;
  RAISE NOTICE 'PASS: eight catalog negatives, exact retry and rotated birth';
END $$;
ROLLBACK;
