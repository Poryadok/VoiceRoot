-- Once allocated, floors and unresolved evidence are forward-only.
BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'Bot membership rollback requires a fresh read committed snapshot' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE bot_space_membership_source,bot_space_membership_heads,bot_space_membership_operations,bot_space_membership_calls,bot_space_membership_completions,bot_space_membership_effects,bot_space_membership_chat_sources,bot_space_membership_cleanup_decisions,bot_space_membership_cleanup_chats IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM bot_space_membership_heads) OR EXISTS(SELECT 1 FROM bot_space_membership_operations) THEN
  RAISE EXCEPTION 'Bot membership history cannot be rolled back' USING ERRCODE='55000';
 END IF;
END $$;
DROP TABLE bot_space_membership_cleanup_decisions;
DROP TABLE bot_space_membership_chat_sources;
DROP TABLE bot_space_membership_effects;
DROP TABLE bot_space_membership_cleanup_chats;
DROP TABLE bot_space_membership_calls;
DROP TABLE bot_space_membership_completions;
DROP TABLE bot_space_membership_heads;
DROP TABLE bot_space_membership_operations;
DROP TABLE bot_space_membership_source;
DROP FUNCTION bot_membership_evidence_guard();
DROP FUNCTION bot_membership_head_guard();
DROP FUNCTION bot_membership_source_guard();
DROP FUNCTION bot_membership_snapshot_guard();
COMMIT;
