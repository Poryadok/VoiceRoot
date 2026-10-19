BEGIN;
LOCK TABLE voice_ordinary_cutover_works,voice_ordinary_cutover_targets,voice_ordinary_cutover_acceptances IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM voice_ordinary_cutover_works)
 OR EXISTS(SELECT 1 FROM voice_ordinary_cutover_targets)
 OR EXISTS(SELECT 1 FROM voice_ordinary_cutover_acceptances) THEN
  RAISE EXCEPTION 'ordinary cutover evidence requires reconciliation before rollback' USING ERRCODE='55000';
 END IF;
END $$;
DROP TABLE voice_ordinary_cutover_targets,voice_ordinary_cutover_acceptances,voice_ordinary_cutover_works;
DROP FUNCTION voice_ordinary_cutover_preserve();
COMMIT;
