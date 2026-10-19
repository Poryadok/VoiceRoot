BEGIN;
LOCK TABLE voice_ordinary_cutover_effect_plans,voice_ordinary_cutover_effect_plan_items IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM voice_ordinary_cutover_effect_plans)
 OR EXISTS(SELECT 1 FROM voice_ordinary_cutover_effect_plan_items) THEN
  RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='ordinary effect plans require reconciliation before rollback';
 END IF;
END $$;
DROP TABLE voice_ordinary_cutover_effect_plan_items;
DROP TABLE voice_ordinary_cutover_effect_plans;
DROP FUNCTION voice_ordinary_cutover_effect_plan_preserve();
COMMIT;
