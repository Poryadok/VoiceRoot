-- DM read-receipt visibility (docs/features/privacy.md). Existing profiles
-- retain the documented default: read receipts enabled.
ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS show_read_receipts BOOLEAN NOT NULL DEFAULT true;
