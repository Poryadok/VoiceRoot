BEGIN;
-- Relation locks serialize first writers with rollback; never erase a retained
-- terminal marker, blocked obligation, known actor or completed result.
LOCK TABLE role_bot_actor_bindings,role_bot_global_removals,role_bot_installation_removals,role_bot_removal_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM role_bot_actor_bindings) OR EXISTS(SELECT 1 FROM role_bot_global_removals)
 OR EXISTS(SELECT 1 FROM role_bot_installation_removals) OR EXISTS(SELECT 1 FROM role_bot_removal_operations)
 THEN RAISE EXCEPTION 'Bot removal evidence prevents rollback' USING ERRCODE='55000'; END IF;
END $$;
DROP TABLE role_bot_removal_operations,role_bot_installation_removals,role_bot_global_removals,role_bot_actor_bindings;
DROP FUNCTION role_bot_removal_immutable();
COMMIT;
