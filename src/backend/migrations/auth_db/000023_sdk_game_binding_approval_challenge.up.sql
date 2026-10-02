ALTER TABLE sdk_authorizations
  ADD COLUMN game_binding_intent BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN game_binding_challenge_nonce VARCHAR(43),
  ADD COLUMN game_binding_consent_revision BIGINT;

UPDATE sdk_authorizations
SET game_binding_intent = TRUE
WHERE game_binding_operation_id IS NOT NULL;

ALTER TABLE sdk_authorizations
  DROP CONSTRAINT sdk_authorizations_binding_challenge_pair_check,
  ADD CONSTRAINT sdk_authorizations_binding_intent_tuple_check CHECK (
    game_binding_intent = (game_binding_operation_id IS NOT NULL)
    AND ((game_binding_challenge_id IS NULL) = (game_binding_challenge_nonce IS NULL))
    AND (game_binding_challenge_nonce IS NULL OR game_binding_challenge_nonce ~ '^[A-Za-z0-9_-]{43}$')
    AND (game_binding_consent_revision IS NULL OR game_binding_consent_revision > 0)
    AND (game_binding_challenge_id IS NULL OR game_binding_intent)
  );
