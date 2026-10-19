-- Initialized identity authority cannot be rolled back into an empty history.
-- Retiring live evidence requires an explicit reviewed owner migration, not a
-- generic downgrade which discards reservations or the CURRENT recovery cut.
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM identity_meta WHERE history_head<>0 OR retained_floor<>0 OR keyset_revision<>0 OR guards_active OR baseline_cut<>0)
    OR EXISTS(SELECT 1 FROM identity_key_versions)
    OR EXISTS(SELECT 1 FROM identity_profile_reservations)
    OR EXISTS(SELECT 1 FROM identity_associations)
    OR EXISTS(SELECT 1 FROM association_selector_aliases)
    OR EXISTS(SELECT 1 FROM identity_parent_fences)
    OR EXISTS(SELECT 1 FROM identity_account_parent_aliases)
    OR EXISTS(SELECT 1 FROM identity_journal)
    OR EXISTS(SELECT 1 FROM profiles WHERE identity_association_id IS NOT NULL OR identity_baseline_cut<>0) THEN
    RAISE EXCEPTION 'Cannot downgrade initialized User identity authority' USING ERRCODE='55000';
  END IF;
END $$;

DROP TRIGGER identity_account_lifecycle ON user_account_lifecycle;
DROP TRIGGER identity_lifecycle_gate ON user_account_lifecycle;
DROP TRIGGER identity_sdk_gate ON sdk_author_tombstones;
DROP TRIGGER identity_authority_clock_gate ON user_authority_revision;
DROP TRIGGER identity_profiles_gate ON profiles;
DROP TRIGGER identity_profile_registrar ON profiles;
DROP TRIGGER identity_profile_lifecycle ON profiles;
DROP TRIGGER user_profile_identity_immutable ON profiles;
ALTER TABLE profiles DROP COLUMN identity_association_id;
ALTER TABLE profiles DROP COLUMN identity_baseline_cut;
DROP TABLE identity_account_parent_aliases;
DROP TABLE association_selector_aliases;
DROP TABLE identity_profile_reservations;
DROP TABLE identity_parent_fences;
DROP TABLE identity_associations;
DROP TABLE identity_key_versions;
DROP TABLE identity_journal;
DROP TABLE identity_meta;
DROP FUNCTION identity_parent_fence_guard();
DROP FUNCTION identity_append_capture_batch();
DROP FUNCTION identity_association_journal_complete();
DROP FUNCTION identity_append_only_guard();
DROP FUNCTION identity_association_guard();
DROP FUNCTION identity_birth_complete();
DROP FUNCTION identity_account_lifecycle_changed();
DROP FUNCTION identity_profile_lifecycle_changed();
DROP FUNCTION identity_profile_registrar_guard();
DROP FUNCTION identity_has_preparation(uuid,uuid,uuid,text);
DROP FUNCTION identity_reservation_guard();
DROP FUNCTION identity_meta_guard();
DROP FUNCTION identity_append_event(uuid,bigint,text,boolean,boolean,boolean,bytea);
DROP FUNCTION identity_statement_gate();
DROP FUNCTION reject_user_profile_identity_rebind();
