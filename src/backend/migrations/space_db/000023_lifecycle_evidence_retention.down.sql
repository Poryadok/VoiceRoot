DO $$ BEGIN IF EXISTS(SELECT 1 FROM space_lifecycle_aggregates WHERE evidence_compacted) THEN RAISE EXCEPTION 'compacted lifecycle evidence cannot be restored'; END IF; END $$;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM space_lifecycle_outbox o WHERE NOT EXISTS(SELECT 1 FROM space_lifecycle_aggregates a WHERE a.space_id=o.space_id)) THEN RAISE EXCEPTION 'independent lifecycle delivery rows cannot regain aggregate FK'; END IF; END $$;
DROP INDEX space_lifecycle_delivered_expiry_idx;
ALTER TABLE space_lifecycle_outbox DROP COLUMN purge_decided_at;
ALTER TABLE space_lifecycle_outbox ADD CONSTRAINT space_lifecycle_outbox_space_id_fkey FOREIGN KEY(space_id) REFERENCES space_lifecycle_aggregates(space_id) ON DELETE CASCADE;
ALTER TABLE space_lifecycle_aggregates DROP CONSTRAINT space_lifecycle_compacted_terminal;
DROP INDEX space_lifecycle_evidence_expiry_idx;
ALTER TABLE space_lifecycle_aggregates DROP COLUMN evidence_compacted;
DROP TABLE space_lifecycle_completions;
