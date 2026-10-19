BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'membership guard DOWN requires READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
-- Match installation lock set/order. Never inspect an inherited old snapshot.
LOCK TABLE spaces,space_members,space_ordinary_member_heads,space_community_member_heads,
 community_roster_members,community_owner_authority,space_membership_ownership_heads,
 space_member_admission_intents,space_native_member_publications,space_native_creator_operations,
 space_bot_installation_attempts,space_bot_installation_heads,space_bot_installation_cancellations,
 space_effective_member_heads,space_bans,ownership_journal,space_lifecycle_aggregates,
 space_permanent_purge_fences,community_owner_recovery_operations,community_bootstrap_operations,
 space_lifecycle_operations,space_lifecycle_participants,space_lifecycle_completions,
 space_lifecycle_outbox,space_deletion_tombstones,native_membership_guard.spaces,native_membership_guard.frames IN ACCESS EXCLUSIVE MODE;
DO $$ DECLARE name text;BEGIN
 IF EXISTS(SELECT 1 FROM native_membership_guard.spaces) OR EXISTS(SELECT 1 FROM native_membership_guard.frames)
 OR EXISTS(SELECT 1 FROM space_effective_member_heads) THEN
  RAISE EXCEPTION 'established membership enforcement is roll-forward only' USING ERRCODE='55000';
 END IF;
 FOREACH name IN ARRAY native_membership_guard.coverage() LOOP
  EXECUTE format('DROP TRIGGER a000_native_membership_capture ON public.%I',name);
  EXECUTE format('DROP TRIGGER a000_native_membership_no_truncate ON public.%I',name);
  EXECUTE format('DROP TRIGGER zzzzzz_native_membership_observe ON public.%I',name);
 END LOOP;
END $$;
DROP TRIGGER zzzzz_native_membership_guard_terminal ON spaces;
DROP FUNCTION public.space_native_membership_begin(uuid,uuid[]);
DROP FUNCTION public.space_native_membership_profiles(uuid,uuid);
DROP FUNCTION public.space_native_membership_complete(uuid,uuid);
DROP FUNCTION public.space_native_membership_require(uuid);
DROP FUNCTION public.space_native_membership_initialize(uuid);
DROP SCHEMA native_membership_guard CASCADE;
COMMIT;
