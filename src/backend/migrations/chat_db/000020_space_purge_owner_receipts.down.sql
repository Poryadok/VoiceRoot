ALTER TABLE chat_space_lifecycle_operations
    DROP CONSTRAINT chat_space_lifecycle_owner_receipts_pair_check,
    DROP COLUMN file_release_receipt_bytes,
    DROP COLUMN messaging_receipt_bytes;
