ALTER TABLE player_binding_challenges
  ADD COLUMN request_sha256 CHAR(64);

UPDATE player_binding_challenges
SET request_sha256 = repeat('0', 64)
WHERE request_sha256 IS NULL;

ALTER TABLE player_binding_challenges
  ALTER COLUMN request_sha256 SET NOT NULL,
  ADD CONSTRAINT player_binding_challenges_request_sha256_check
    CHECK (request_sha256 ~ '^[0-9a-f]{64}$');
