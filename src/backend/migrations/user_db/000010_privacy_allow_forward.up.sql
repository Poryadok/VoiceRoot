-- Binary privacy: allow others to forward this profile's messages (docs/features/privacy.md, forward-messages.md).
ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS allow_forward BOOLEAN NOT NULL DEFAULT true;
