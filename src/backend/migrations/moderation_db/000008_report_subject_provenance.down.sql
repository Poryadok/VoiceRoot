DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM reports WHERE reporter_subject_id IS NOT NULL OR validated_target_subject_id IS NOT NULL) THEN
  RAISE EXCEPTION 'report subject provenance exists; rollback requires authorized privacy recovery';
 END IF;
END $$;
ALTER TABLE reports DROP COLUMN validated_target_subject_revision, DROP COLUMN validated_target_subject_id,
 DROP COLUMN reporter_subject_revision, DROP COLUMN reporter_subject_id;
