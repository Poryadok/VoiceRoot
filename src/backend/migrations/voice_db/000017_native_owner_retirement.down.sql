DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM voice_native_owner_works) THEN
  RAISE EXCEPTION 'native owner retirement evidence cannot be discarded by DOWN';
 END IF;
END $$;
ALTER TABLE voice_space_lifecycle_fence_receipts DROP COLUMN retirement_owner_work_id;
DROP TABLE voice_native_owner_targets,voice_native_owner_acceptances,voice_native_owner_works;
DROP FUNCTION voice_native_owner_preserve_evidence();
