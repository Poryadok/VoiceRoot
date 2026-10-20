BEGIN;
-- This Role-only transition reuses the published25 INVOKER guard. No new
-- function, erased-source adoption or owner/provider authority is introduced.
DO $$ BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'Role terminal migration requires READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE public.schema_migrations,public.chat_overrides,public.member_roles,public.ownership_transfer_v2,public.role_space_deletion_fences,public.role_space_lifecycle,public.role_voice_policy_epochs,public.role_voice_policy_outbox,public.roles,public.voice_room_overrides,public.game_session_grant_sessions,public.game_session_grants,public.role_sdk_authority_revision,public.role_space_chat_grants,public.role_space_chat_policies,public.role_member_source_floors,public.role_member_admission_preparations,public.role_member_admission_receipts,public.role_member_contributions,public.role_member_owner_authority,public.role_bot_actor_bindings,public.role_bot_global_removals,public.role_bot_installation_removals,public.role_bot_removal_operations IN ACCESS SHARE MODE;
DO $$ DECLARE valid BOOLEAN; BEGIN
 SELECT count(*)=1 AND bool_and((version=25 AND NOT dirty) OR (version=26 AND dirty)) INTO valid FROM public.schema_migrations;
 IF valid IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'known Role25 migration source required'; END IF;
 SELECT count(*)=24 AND bool_and(c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
 AND NOT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=c.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid)) INTO valid
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY(ARRAY['schema_migrations','chat_overrides','member_roles','ownership_transfer_v2','role_space_deletion_fences','role_space_lifecycle','role_voice_policy_epochs','role_voice_policy_outbox','roles','voice_room_overrides','game_session_grant_sessions','game_session_grants','role_sdk_authority_revision','role_space_chat_grants','role_space_chat_policies','role_member_source_floors','role_member_admission_preparations','role_member_admission_receipts','role_member_contributions','role_member_owner_authority','role_bot_actor_bindings','role_bot_global_removals','role_bot_installation_removals','role_bot_removal_operations']);
 IF valid IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'Role25 relations unavailable'; END IF;
 WITH expected(name,arguments,result,body) AS (VALUES
('role_voice_policy_bump',3,'bigint','cfa8b3d1a003580fd28738ad5c404dff002f4fe0d7f775fc3a057fec1313d55a'),
('role_authority_revision_floor_guard',0,'trigger','8c9dc242cd302fdf434075cd9b793d28ac126f2416c7d82125c820de7671c327'),
('role_authority_revision_changed',0,'trigger','fc408928b9e427013dcc973e227b84f73d9ae04be3e4aba9bde2fee11b2999d4'),
('role_sdk_authority_floor_guard',0,'trigger','9a8f5f22fef761b6d6a318f08cd758556ef74fccb2ebfb08ca22dc7ecaa6df58'),
('role_sdk_authority_changed',0,'trigger','5506ca2bf08faa9c260fad3eff5d24e481b2eec3066fb8089d695b8341156383'),
('role_member_admission_receipt_immutable',0,'trigger','9ddc27f44d0eed03f3f8f5d7000397ea1456b1dacf069e123459937c88dbc444'),
('role_member_source_floor_guard',0,'trigger','854569c5d41901693538ddccfd2143394d0ee834521e2a3c13c2ba2217027ca2'),
('role_member_contribution_guard',0,'trigger','80ac96acb27904fed0c4af7d8ae657098492ad30e2ad8cc2fae67dac4906ac4c'),
('role_member_owner_authority_guard',0,'trigger','3a876182c5497394827048ba06f56ee547cfdbc18b38021007837a48788fb63b'),
('role_bot_removal_immutable',0,'trigger','4ca077ff39e0a267729a13374bf69e174fc9b8a7b39e9d68df726020e304f34e'))
 SELECT count(*)=10 AND bool_and(encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')=e.body) INTO valid
 FROM expected e JOIN pg_proc p ON p.proname=e.name AND p.pronargs=e.arguments AND p.prorettype=to_regtype(e.result)
 JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang
 WHERE n.nspname='public' AND NOT p.prosecdef AND p.provolatile='v' AND l.lanname='plpgsql';
 IF valid IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'Role25 functions unavailable'; END IF;
 WITH expected(relation,name,fn,kind,arg) AS (VALUES
('roles','role_authority_revision_roles','role_authority_revision_changed',29,'space_id'),
('roles','role_authority_revision_roles_truncate','role_authority_revision_floor_guard',34,''),
('member_roles','role_authority_revision_member_roles','role_authority_revision_changed',29,'space_id'),
('member_roles','role_authority_revision_member_roles_truncate','role_authority_revision_floor_guard',34,''),
('ownership_transfer_v2','role_authority_revision_ownership_transfer_v2','role_authority_revision_changed',29,'space_id'),
('ownership_transfer_v2','role_authority_revision_ownership_transfer_v2_truncate','role_authority_revision_floor_guard',34,''),
('role_space_lifecycle','role_authority_revision_role_space_lifecycle','role_authority_revision_changed',29,'space_id'),
('role_space_lifecycle','role_authority_revision_role_space_lifecycle_truncate','role_authority_revision_floor_guard',34,''),
('role_space_deletion_fences','role_authority_revision_role_space_deletion_fences','role_authority_revision_changed',29,'space_id'),
('role_space_deletion_fences','role_authority_revision_role_space_deletion_fences_truncate','role_authority_revision_floor_guard',34,''),
('chat_overrides','role_authority_revision_chat_overrides','role_authority_revision_changed',29,'role_id'),
('chat_overrides','role_authority_revision_chat_overrides_truncate','role_authority_revision_floor_guard',34,''),
('voice_room_overrides','role_authority_revision_voice_room_overrides','role_authority_revision_changed',29,'role_id'),
('voice_room_overrides','role_authority_revision_voice_room_overrides_truncate','role_authority_revision_floor_guard',34,''),
('role_voice_policy_epochs','role_authority_revision_floor_change','role_authority_revision_floor_guard',31,''),
('role_voice_policy_epochs','role_authority_revision_floor_truncate','role_authority_revision_floor_guard',34,''),
('role_voice_policy_outbox','role_authority_revision_outbox_truncate','role_authority_revision_floor_guard',34,''),
('role_sdk_authority_revision','role_sdk_authority_floor_change','role_sdk_authority_floor_guard',31,''),
('role_sdk_authority_revision','role_sdk_authority_floor_truncate','role_sdk_authority_floor_guard',34,''),
('game_session_grant_sessions','role_sdk_authority_sessions','role_sdk_authority_changed',28,''),
('game_session_grants','role_sdk_authority_grants','role_sdk_authority_changed',28,''),
('game_session_grant_sessions','role_sdk_authority_sessions_truncate','role_sdk_authority_floor_guard',34,''),
('game_session_grants','role_sdk_authority_grants_truncate','role_sdk_authority_floor_guard',34,''),
('role_space_chat_grants','role_authority_revision_space_chat_grants','role_authority_revision_changed',29,'space_id'),
('role_space_chat_grants','role_authority_revision_space_chat_grants_no_truncate','role_authority_revision_floor_guard',34,''),
('role_space_chat_policies','role_authority_revision_space_chat_policies','role_authority_revision_changed',29,'space_id'),
('role_space_chat_policies','role_authority_revision_space_chat_policies_no_truncate','role_authority_revision_floor_guard',34,''),
('role_member_source_floors','role_member_source_floor_change','role_member_source_floor_guard',27,''),
('role_member_source_floors','role_member_source_floor_truncate','role_member_source_floor_guard',34,''),
('role_member_source_floors','role_authority_revision_member_source_floors','role_authority_revision_changed',29,'space_id'),
('role_member_admission_preparations','role_authority_revision_member_preparations','role_authority_revision_changed',29,'space_id'),
('role_member_admission_preparations','role_authority_revision_member_preparations_truncate','role_authority_revision_floor_guard',34,''),
('role_member_admission_receipts','role_member_admission_receipt_change','role_member_admission_receipt_immutable',27,''),
('role_member_admission_receipts','role_member_admission_receipt_truncate','role_member_admission_receipt_immutable',34,''),
('role_member_contributions','role_member_contribution_insert','role_member_contribution_guard',23,''),
('role_member_contributions','role_member_contribution_truncate','role_member_contribution_guard',34,''),
('role_member_contributions','role_authority_revision_member_contributions','role_authority_revision_changed',13,'space_id'),
('role_member_owner_authority','role_member_owner_authority_change','role_member_owner_authority_guard',31,''),
('role_member_owner_authority','role_member_owner_authority_truncate','role_member_owner_authority_guard',34,''),
('role_member_owner_authority','role_authority_revision_member_owner_authority','role_authority_revision_changed',29,'space_id'),
('role_bot_actor_bindings','role_bot_actor_bindings_guard','role_bot_removal_immutable',58,''),
('role_bot_global_removals','role_bot_global_removals_guard','role_bot_removal_immutable',58,''),
('role_bot_installation_removals','role_bot_installation_removals_guard','role_bot_removal_immutable',58,''),
('role_bot_removal_operations','role_bot_removal_operations_change','role_bot_removal_immutable',27,''),
('role_bot_removal_operations','role_bot_removal_operations_truncate','role_bot_removal_immutable',34,''))
 SELECT count(*)=45 INTO valid FROM expected e
 JOIN pg_trigger t ON t.tgrelid=to_regclass(format('public.%I',e.relation)) AND t.tgname=e.name
 JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE NOT t.tgisinternal AND t.tgenabled='O' AND t.tgtype=e.kind AND n.nspname='public' AND p.proname=e.fn
 AND t.tgargs=CASE WHEN e.arg='' THEN ''::bytea ELSE convert_to(e.arg,'UTF8')||decode('00','hex') END;
 IF valid IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'Role25 triggers unavailable'; END IF;
END $$;
CREATE TABLE public.role_bot_global_terminals (
 bot_id UUID PRIMARY KEY CHECK(bot_id<>'00000000-0000-0000-0000-000000000000'),
 decision_id UUID NOT NULL UNIQUE CHECK(decision_id<>'00000000-0000-0000-0000-000000000000'),
 receipt_id UUID NOT NULL UNIQUE CHECK(receipt_id<>'00000000-0000-0000-0000-000000000000'),
 registered_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp() CHECK(isfinite(registered_at))
);
REVOKE ALL ON TABLE public.role_bot_global_terminals FROM PUBLIC;
CREATE TRIGGER role_bot_global_terminals_guard BEFORE UPDATE OR DELETE OR TRUNCATE ON public.role_bot_global_terminals FOR EACH STATEMENT EXECUTE FUNCTION public.role_bot_removal_immutable();
COMMIT;
