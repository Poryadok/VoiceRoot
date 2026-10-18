-- NULL is explicit legacy provenance. Never infer opaque subjects from IDs.
ALTER TABLE reports
 ADD COLUMN reporter_subject_id uuid,
 ADD COLUMN reporter_subject_revision bigint,
 ADD COLUMN validated_target_subject_id uuid,
 ADD COLUMN validated_target_subject_revision bigint,
 ADD CONSTRAINT reports_reporter_subject_pair CHECK ((reporter_subject_id IS NULL AND reporter_subject_revision IS NULL) OR (reporter_subject_id IS NOT NULL AND reporter_subject_revision IS NOT NULL AND reporter_subject_revision > 0 AND reporter_account_id IS NOT NULL)),
 ADD CONSTRAINT reports_target_subject_pair CHECK ((validated_target_subject_id IS NULL AND validated_target_subject_revision IS NULL) OR (validated_target_subject_id IS NOT NULL AND validated_target_subject_revision IS NOT NULL AND validated_target_subject_revision > 0 AND validated_target_account_id IS NOT NULL));
CREATE INDEX reports_reporter_subject_idx ON reports(reporter_subject_id) WHERE reporter_subject_id IS NOT NULL;
CREATE INDEX reports_target_subject_idx ON reports(validated_target_subject_id) WHERE validated_target_subject_id IS NOT NULL;
