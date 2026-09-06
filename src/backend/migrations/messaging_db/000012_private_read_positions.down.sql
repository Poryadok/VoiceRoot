DROP TABLE IF EXISTS read_positions;

-- Down migrations are only supported before a receipt without a public read
-- cursor has been written.
ALTER TABLE read_receipts
  ALTER COLUMN last_read_message_id SET NOT NULL;
