ALTER TABLE player_binding_challenges
  DROP CONSTRAINT IF EXISTS player_binding_challenges_request_sha256_check,
  DROP COLUMN IF EXISTS request_sha256;
