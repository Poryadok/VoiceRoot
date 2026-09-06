-- Expand/contract rollback: restore a non-null public cursor without dropping
-- receipt rows. If a legacy delivery-only row cannot be represented by the old
-- NOT NULL schema, abort rather than silently losing cursor data.
UPDATE read_receipts AS rr
SET last_read_message_id = COALESCE(
  (SELECT rp.last_read_message_id
   FROM read_positions AS rp
   WHERE rp.chat_id = rr.chat_id AND rp.profile_id = rr.profile_id),
  rr.last_delivered_message_id
)
WHERE rr.last_read_message_id IS NULL;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM read_receipts WHERE last_read_message_id IS NULL) THEN
    RAISE EXCEPTION 'cannot safely roll back 000013: delivery-only read_receipts row has no cursor';
  END IF;
END $$;

ALTER TABLE read_receipts
  ALTER COLUMN last_read_message_id SET NOT NULL;

DROP TABLE IF EXISTS read_positions;
