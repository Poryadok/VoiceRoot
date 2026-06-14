-- user_db v3 — profile banner_url and frozen_at for subscription tier limits
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS banner_url TEXT NULL;
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS frozen_at TIMESTAMPTZ NULL;
