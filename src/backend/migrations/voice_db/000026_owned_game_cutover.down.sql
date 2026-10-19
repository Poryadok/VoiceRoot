BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM voice_owned_game_cutover_work) THEN
  RAISE EXCEPTION 'retained owned/game cutover evidence blocks rollback' USING ERRCODE='55000';
 END IF;
END; $$;
DROP TABLE voice_owned_game_cutover_work;
DROP FUNCTION voice_owned_game_cutover_immutable_fn();
COMMIT;
