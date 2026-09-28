ALTER TABLE player_binding_challenges
  ADD COLUMN source_account_id UUID,
  ADD COLUMN source_actor_id UUID,
  ADD COLUMN source_device_id UUID,
  ADD COLUMN source_generation BIGINT,
  ADD COLUMN target_account_id UUID,
  ADD COLUMN target_profile_id UUID,
  ADD COLUMN profile_revision BIGINT,
  ADD COLUMN consent_revision BIGINT,
  ADD COLUMN policy_revision BIGINT,
  ADD COLUMN scopes TEXT[];

ALTER TABLE player_binding_challenges
  ADD CONSTRAINT player_binding_challenges_consent_tuple_check CHECK (
    (source_account_id IS NULL AND source_actor_id IS NULL AND source_device_id IS NULL AND source_generation IS NULL
      AND target_account_id IS NULL AND target_profile_id IS NULL AND profile_revision IS NULL
      AND consent_revision IS NULL AND policy_revision IS NULL AND scopes IS NULL)
    OR
    (source_account_id IS NOT NULL AND source_actor_id IS NOT NULL AND source_device_id IS NOT NULL
      AND source_generation > 0 AND target_account_id IS NOT NULL AND target_profile_id IS NOT NULL
      AND profile_revision > 0 AND consent_revision > 0 AND policy_revision > 0
      AND scopes IS NOT NULL AND cardinality(scopes) BETWEEN 1 AND 16)
  );
