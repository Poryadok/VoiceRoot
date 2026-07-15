-- user_db v7 — per-profile accent color (multi-profile.md visual indicator)
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS accent_color VARCHAR(7) NULL
  CHECK (accent_color IS NULL OR accent_color ~ '^#[0-9A-Fa-f]{6}$');
