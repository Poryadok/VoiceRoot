-- Complete owner revision coverage for bounded authority snapshots.
-- Preserve the original v11 counter/outbox data and never reset a saved floor.
BEGIN;
LOCK TABLE public.categories,public.community_owner_authority,public.community_owner_recovery_operations,public.community_roster_members,public.ownership_journal,public.space_bans,public.space_deletion_tombstones,public.space_lifecycle_aggregates,public.space_member_timeouts,public.space_members,public.space_subscriptions,public.space_tree_nodes,public.space_voice_access_epochs,public.space_voice_access_outbox,public.spaces,public.voice_rooms IN ACCESS EXCLUSIVE MODE;
DO $$ DECLARE target REGCLASS; BEGIN
 FOREACH target IN ARRAY ARRAY['public.categories'::regclass,'public.community_owner_authority'::regclass,'public.community_owner_recovery_operations'::regclass,'public.community_roster_members'::regclass,'public.ownership_journal'::regclass,'public.space_bans'::regclass,'public.space_deletion_tombstones'::regclass,'public.space_lifecycle_aggregates'::regclass,'public.space_member_timeouts'::regclass,'public.space_members'::regclass,'public.space_subscriptions'::regclass,'public.space_tree_nodes'::regclass,'public.space_voice_access_epochs'::regclass,'public.space_voice_access_outbox'::regclass,'public.spaces'::regclass,'public.voice_rooms'::regclass] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=target AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=target)
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=target)
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=target OR inhparent=target) THEN
   RAISE EXCEPTION 'Space authority relation behavior is not canonical' USING ERRCODE='55000';
  END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM public.space_voice_access_outbox o LEFT JOIN public.space_voice_access_epochs e USING(space_id)
           WHERE e.access_epoch IS NULL OR o.access_epoch>e.access_epoch) THEN
  RAISE EXCEPTION 'space authority revision floor is inconsistent' USING ERRCODE='55000';
 END IF;
END $$;
CREATE FUNCTION space_authority_revision_floor_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'space authority revision floor cannot be removed' USING ERRCODE='55000';
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.access_epoch::numeric<>OLD.access_epoch::numeric+1 THEN
   RAISE EXCEPTION 'space authority revision must advance exactly once' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.access_epoch<>1 THEN
  RAISE EXCEPTION 'space authority revision must start at one' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER space_authority_revision_floor_change BEFORE INSERT OR UPDATE OR DELETE ON space_voice_access_epochs
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_floor_truncate BEFORE TRUNCATE ON space_voice_access_epochs
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_outbox_truncate BEFORE TRUNCATE ON space_voice_access_outbox
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE FUNCTION space_authority_revision_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 old_scope UUID;
 new_scope UUID;
 affected UUID;
BEGIN
 IF TG_OP<>'INSERT' THEN old_scope := (to_jsonb(OLD)->>TG_ARGV[0])::uuid; END IF;
 IF TG_OP<>'DELETE' THEN new_scope := (to_jsonb(NEW)->>TG_ARGV[0])::uuid; END IF;
 FOR affected IN SELECT DISTINCT scope FROM unnest(ARRAY[old_scope,new_scope]) scope WHERE scope IS NOT NULL ORDER BY scope LOOP
  PERFORM public.space_voice_access_bump(affected,NULL,NULL);
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER space_voice_access_space_delete ON spaces;
DROP TRIGGER space_voice_access_member_change ON space_members;
DROP TRIGGER space_voice_access_room_change ON voice_rooms;
CREATE TRIGGER space_authority_revision_spaces AFTER INSERT OR UPDATE OR DELETE ON spaces
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('id');
CREATE TRIGGER space_authority_revision_spaces_truncate BEFORE TRUNCATE ON spaces
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_space_members AFTER INSERT OR UPDATE OR DELETE ON space_members
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_members_truncate BEFORE TRUNCATE ON space_members
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_voice_rooms AFTER INSERT OR UPDATE OR DELETE ON voice_rooms
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_voice_rooms_truncate BEFORE TRUNCATE ON voice_rooms
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_space_bans AFTER INSERT OR UPDATE OR DELETE ON space_bans
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_bans_truncate BEFORE TRUNCATE ON space_bans
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_space_member_timeouts AFTER INSERT OR UPDATE OR DELETE ON space_member_timeouts
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_member_timeouts_truncate BEFORE TRUNCATE ON space_member_timeouts
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_space_lifecycle_aggregates AFTER INSERT OR UPDATE OR DELETE ON space_lifecycle_aggregates
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_lifecycle_aggregates_truncate BEFORE TRUNCATE ON space_lifecycle_aggregates
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_space_deletion_tombstones AFTER INSERT OR UPDATE OR DELETE ON space_deletion_tombstones
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_deletion_tombstones_truncate BEFORE TRUNCATE ON space_deletion_tombstones
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_ownership_journal AFTER INSERT OR UPDATE OR DELETE ON ownership_journal
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_ownership_journal_truncate BEFORE TRUNCATE ON ownership_journal
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_space_subscriptions AFTER INSERT OR UPDATE OR DELETE ON space_subscriptions
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_subscriptions_truncate BEFORE TRUNCATE ON space_subscriptions
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_community_owner_authority AFTER INSERT OR UPDATE OR DELETE ON community_owner_authority
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_community_owner_authority_truncate BEFORE TRUNCATE ON community_owner_authority
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_community_owner_recovery_operations AFTER INSERT OR UPDATE OR DELETE ON community_owner_recovery_operations
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_community_owner_recovery_operations_truncate BEFORE TRUNCATE ON community_owner_recovery_operations
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_community_roster_members AFTER INSERT OR UPDATE OR DELETE ON community_roster_members
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_community_roster_members_truncate BEFORE TRUNCATE ON community_roster_members
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_space_tree_nodes AFTER INSERT OR UPDATE OR DELETE ON space_tree_nodes
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_space_tree_nodes_truncate BEFORE TRUNCATE ON space_tree_nodes
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
CREATE TRIGGER space_authority_revision_categories AFTER INSERT OR UPDATE OR DELETE ON categories
 FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed('space_id');
CREATE TRIGGER space_authority_revision_categories_truncate BEFORE TRUNCATE ON categories
 FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard();
-- Activation invalidates saved snapshots; deleted scopes retain their floors.
DO $$ DECLARE affected UUID; BEGIN
 FOR affected IN SELECT space_id FROM space_voice_access_epochs
  UNION SELECT id FROM spaces
  UNION SELECT space_id FROM space_members
  UNION SELECT space_id FROM voice_rooms
  UNION SELECT space_id FROM space_bans
  UNION SELECT space_id FROM space_member_timeouts
  UNION SELECT space_id FROM space_lifecycle_aggregates
  UNION SELECT space_id FROM space_deletion_tombstones
  UNION SELECT space_id FROM ownership_journal
  UNION SELECT space_id FROM space_subscriptions
  UNION SELECT space_id FROM community_owner_authority
  UNION SELECT space_id FROM community_owner_recovery_operations
  UNION SELECT space_id FROM community_roster_members
  UNION SELECT space_id FROM space_tree_nodes
  UNION SELECT space_id FROM categories ORDER BY space_id LOOP
  PERFORM space_voice_access_bump(affected,NULL,NULL);
 END LOOP;
END $$;
COMMIT;
