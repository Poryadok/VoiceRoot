-- Lock before observing emptiness: a receipt-only import has no reports FK and
-- can commit while an ALTER waits. Keep the complete downgrade atomic even if
-- a migration executor handles top-level statements separately.
DO $$ BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'report rejection downgrade requires read committed';
 END IF;
 LOCK TABLE reports IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE report_target_rejections IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM report_target_rejections) OR EXISTS(SELECT 1 FROM reports WHERE target_id IS NULL) THEN
  RAISE EXCEPTION 'cannot remove report rejection replay protection';
 END IF;
 ALTER TABLE reports DROP CONSTRAINT reports_target_representation;
 ALTER TABLE reports ALTER COLUMN target_id SET NOT NULL;
 DROP TABLE report_target_rejections;
END $$;
