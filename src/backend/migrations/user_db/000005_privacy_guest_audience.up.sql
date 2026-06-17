ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS show_online_include_guests BOOLEAN NOT NULL DEFAULT false;
