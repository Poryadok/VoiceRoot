DO $$
BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'report subject downgrade requires read committed';
 END IF;
 LOCK TABLE reports IN ACCESS EXCLUSIVE MODE;
 IF EXISTS (SELECT 1 FROM reports WHERE reporter_subject_id IS NOT NULL OR validated_target_subject_id IS NOT NULL) THEN
  RAISE EXCEPTION 'report subject provenance exists; rollback requires authorized privacy recovery';
 END IF;
 ALTER TABLE reports DROP COLUMN validated_target_subject_revision, DROP COLUMN validated_target_subject_id,
 DROP COLUMN reporter_subject_revision, DROP COLUMN reporter_subject_id;
END $$;
