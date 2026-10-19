-- Lock before inspecting even empty tables. NOWAIT refuses active readers or
-- writers without waiting in the application's different table-lock orders.
-- Locks, retained-state checks and DDL share this single statement transaction.
DO $$
BEGIN
  LOCK TABLE messages, messaging_file_preview_snapshots,
    messaging_file_preview_pending, messaging_file_preview_reconciliation,
    messaging_file_preview_errors IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF current_setting('transaction_isolation') <> 'read committed' THEN
    RAISE EXCEPTION 'preview rollback requires fresh READ COMMITTED checks'
      USING ERRCODE = '55000';
  END IF;
  IF EXISTS (SELECT 1 FROM messaging_file_preview_snapshots)
     OR EXISTS (SELECT 1 FROM messaging_file_preview_pending)
     OR EXISTS (SELECT 1 FROM messaging_file_preview_errors)
     OR EXISTS (SELECT 1 FROM messages WHERE file_preview_bindings <> '[]'::jsonb)
     OR (SELECT count(*) FROM messaging_file_preview_reconciliation) <> 1
     OR EXISTS (SELECT 1 FROM messaging_file_preview_reconciliation
                WHERE after_id <> '00000000-0000-0000-0000-000000000000'::uuid
                   OR pass <> 0) THEN
    RAISE EXCEPTION 'preview rollback would erase Messaging authority or recovery evidence'
      USING ERRCODE = '55000';
  END IF;
  DROP TABLE messaging_file_preview_errors, messaging_file_preview_reconciliation,
    messaging_file_preview_pending, messaging_file_preview_snapshots;
  ALTER TABLE messages DROP COLUMN file_preview_bindings;
END $$;
