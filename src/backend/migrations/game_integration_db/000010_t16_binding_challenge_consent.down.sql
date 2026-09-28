ALTER TABLE player_binding_challenges
  DROP CONSTRAINT IF EXISTS player_binding_challenges_consent_tuple_check,
  DROP COLUMN IF EXISTS source_account_id,
  DROP COLUMN IF EXISTS source_actor_id,
  DROP COLUMN IF EXISTS source_device_id,
  DROP COLUMN IF EXISTS source_generation,
  DROP COLUMN IF EXISTS target_account_id,
  DROP COLUMN IF EXISTS target_profile_id,
  DROP COLUMN IF EXISTS profile_revision,
  DROP COLUMN IF EXISTS consent_revision,
  DROP COLUMN IF EXISTS policy_revision,
  DROP COLUMN IF EXISTS scopes;
