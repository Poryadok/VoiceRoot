DROP INDEX IF EXISTS organization_verification_one_pending_profile_idx;
-- Superseded challenges remain expired: rollback must never revive old proofs.
