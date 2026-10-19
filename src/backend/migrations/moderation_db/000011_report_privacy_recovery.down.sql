DO $$ BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'report privacy downgrade requires read committed';
 END IF;
 LOCK TABLE reports IN ACCESS EXCLUSIVE MODE;
 -- Snapshot/export readers take metadata before journal rows.
 LOCK TABLE report_privacy_journal_metadata IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE report_privacy_journal IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM report_privacy_journal)
 OR EXISTS(SELECT 1 FROM reports WHERE reporter_provenance='OWNER_RECOVERED_LEGACY' OR validated_target_provenance='OWNER_RECOVERED_LEGACY') THEN
  RAISE EXCEPTION 'report privacy recovery provenance cannot be discarded';
 END IF;
 DROP TABLE report_privacy_journal;
 DROP TABLE report_privacy_journal_metadata;
 ALTER TABLE reports DROP CONSTRAINT reports_recovered_provenance_binding;
 ALTER TABLE reports DROP COLUMN reporter_provenance;
 ALTER TABLE reports DROP CONSTRAINT reports_recovered_target_binding;
 ALTER TABLE reports DROP COLUMN validated_target_provenance;
END $$;
