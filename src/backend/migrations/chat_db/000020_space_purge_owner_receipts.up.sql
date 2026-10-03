ALTER TABLE chat_space_lifecycle_operations
    ADD COLUMN messaging_receipt_bytes BYTEA,
    ADD COLUMN file_release_receipt_bytes BYTEA,
    ADD CONSTRAINT chat_space_lifecycle_owner_receipts_pair_check
      CHECK ((messaging_receipt_bytes IS NULL) = (file_release_receipt_bytes IS NULL));
