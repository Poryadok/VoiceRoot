DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM report_target_rejections) OR EXISTS(SELECT 1 FROM reports WHERE target_id IS NULL) THEN
  RAISE EXCEPTION 'cannot remove report rejection replay protection';
 END IF;
END $$;
ALTER TABLE reports DROP CONSTRAINT reports_target_representation;
ALTER TABLE reports ALTER COLUMN target_id SET NOT NULL;
DROP TABLE report_target_rejections;
