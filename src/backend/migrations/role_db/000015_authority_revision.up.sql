-- Complete owner revision coverage for bounded authority snapshots.
-- Preserve the original v11 counter/outbox data and never reset a saved floor.
BEGIN;
LOCK TABLE public.chat_overrides,public.member_roles,public.ownership_transfer_v2,public.role_space_deletion_fences,public.role_space_lifecycle,public.role_voice_policy_epochs,public.role_voice_policy_outbox,public.roles,public.voice_room_overrides IN ACCESS EXCLUSIVE MODE;
DO $$ DECLARE target REGCLASS; BEGIN
 FOREACH target IN ARRAY ARRAY['public.chat_overrides'::regclass,'public.member_roles'::regclass,'public.ownership_transfer_v2'::regclass,'public.role_space_deletion_fences'::regclass,'public.role_space_lifecycle'::regclass,'public.role_voice_policy_epochs'::regclass,'public.role_voice_policy_outbox'::regclass,'public.roles'::regclass,'public.voice_room_overrides'::regclass] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=target AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=target)
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=target)
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=target OR inhparent=target) THEN
   RAISE EXCEPTION 'Role authority relation behavior is not canonical' USING ERRCODE='55000';
  END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM public.role_voice_policy_outbox o LEFT JOIN public.role_voice_policy_epochs e USING(space_id)
           WHERE e.policy_epoch IS NULL OR o.policy_epoch>e.policy_epoch) THEN
  RAISE EXCEPTION 'role authority revision floor is inconsistent' USING ERRCODE='55000';
 END IF;
END $$;
CREATE FUNCTION role_authority_revision_floor_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'role authority revision floor cannot be removed' USING ERRCODE='55000';
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.policy_epoch::numeric<>OLD.policy_epoch::numeric+1 THEN
   RAISE EXCEPTION 'role authority revision must advance exactly once' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.policy_epoch<>1 THEN
  RAISE EXCEPTION 'role authority revision must start at one' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER role_authority_revision_floor_change BEFORE INSERT OR UPDATE OR DELETE ON role_voice_policy_epochs
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_floor_truncate BEFORE TRUNCATE ON role_voice_policy_epochs
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_outbox_truncate BEFORE TRUNCATE ON role_voice_policy_outbox
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE FUNCTION role_authority_revision_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 old_scope UUID;
 new_scope UUID;
 affected UUID;
BEGIN
 IF TG_OP<>'INSERT' THEN old_scope := (to_jsonb(OLD)->>TG_ARGV[0])::uuid; END IF;
 IF TG_OP<>'DELETE' THEN new_scope := (to_jsonb(NEW)->>TG_ARGV[0])::uuid; END IF;
 IF TG_ARGV[0]='role_id' THEN
  SELECT space_id INTO old_scope FROM public.roles WHERE id=old_scope;
  SELECT space_id INTO new_scope FROM public.roles WHERE id=new_scope;
  -- Cascading parent deletion has its own Space-wide invalidation.
 END IF;
 FOR affected IN SELECT DISTINCT scope FROM unnest(ARRAY[old_scope,new_scope]) scope WHERE scope IS NOT NULL ORDER BY scope LOOP
  PERFORM public.role_voice_policy_bump(affected,NULL,NULL);
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER role_voice_policy_role_change ON roles;
DROP TRIGGER role_voice_policy_member_change ON member_roles;
DROP TRIGGER role_voice_policy_ownership_change ON ownership_transfer_v2;
DROP TRIGGER role_voice_policy_lifecycle_change ON role_space_lifecycle;
DROP TRIGGER role_voice_policy_voice_override_change ON voice_room_overrides;
CREATE TRIGGER role_authority_revision_roles AFTER INSERT OR UPDATE OR DELETE ON roles
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_roles_truncate BEFORE TRUNCATE ON roles
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_member_roles AFTER INSERT OR UPDATE OR DELETE ON member_roles
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_member_roles_truncate BEFORE TRUNCATE ON member_roles
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_ownership_transfer_v2 AFTER INSERT OR UPDATE OR DELETE ON ownership_transfer_v2
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_ownership_transfer_v2_truncate BEFORE TRUNCATE ON ownership_transfer_v2
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_role_space_lifecycle AFTER INSERT OR UPDATE OR DELETE ON role_space_lifecycle
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_role_space_lifecycle_truncate BEFORE TRUNCATE ON role_space_lifecycle
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_role_space_deletion_fences AFTER INSERT OR UPDATE OR DELETE ON role_space_deletion_fences
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_role_space_deletion_fences_truncate BEFORE TRUNCATE ON role_space_deletion_fences
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_chat_overrides AFTER INSERT OR UPDATE OR DELETE ON chat_overrides
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('role_id');
CREATE TRIGGER role_authority_revision_chat_overrides_truncate BEFORE TRUNCATE ON chat_overrides
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_voice_room_overrides AFTER INSERT OR UPDATE OR DELETE ON voice_room_overrides
 FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('role_id');
CREATE TRIGGER role_authority_revision_voice_room_overrides_truncate BEFORE TRUNCATE ON voice_room_overrides
 FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
-- Invalidate every saved scope at activation, including legacy fence-only scopes.
DO $$ DECLARE affected UUID; BEGIN
 FOR affected IN SELECT space_id FROM role_voice_policy_epochs
  UNION SELECT space_id FROM roles
  UNION SELECT space_id FROM member_roles
  UNION SELECT space_id FROM ownership_transfer_v2
  UNION SELECT space_id FROM role_space_lifecycle
  UNION SELECT space_id FROM role_space_deletion_fences ORDER BY space_id LOOP
  PERFORM role_voice_policy_bump(affected,NULL,NULL);
 END LOOP;
END $$;
COMMIT;
