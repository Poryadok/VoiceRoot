-- A reader's local position drives unread counters. Public DM receipts remain
-- in read_receipts and are written only when both participants opt in.
CREATE TABLE read_positions (
  chat_id UUID NOT NULL,
  profile_id UUID NOT NULL,
  last_read_message_id UUID NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (chat_id, profile_id)
);

CREATE INDEX read_positions_profile_id_idx ON read_positions (profile_id);

INSERT INTO read_positions (chat_id, profile_id, last_read_message_id, updated_at)
SELECT chat_id, profile_id, last_read_message_id, updated_at
FROM read_receipts
WHERE last_read_message_id IS NOT NULL
ON CONFLICT (chat_id, profile_id) DO NOTHING;

ALTER TABLE read_receipts
  ALTER COLUMN last_read_message_id DROP NOT NULL;
