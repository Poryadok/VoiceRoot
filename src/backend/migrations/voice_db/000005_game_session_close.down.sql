BEGIN;

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM voice_game_session_closures) THEN
        RAISE EXCEPTION 'cannot remove game session close receipts while managed rooms exist';
    END IF;
END $$;
DROP TRIGGER voice_game_session_closure_delete_guard ON voice_game_session_closures;
DROP TRIGGER voice_game_session_closure_transition_guard ON voice_game_session_closures;
DROP FUNCTION voice_game_session_closure_delete_guard_fn();
DROP FUNCTION voice_game_session_closure_transition_guard_fn();
DROP TABLE voice_game_session_closures;
ALTER TABLE voice_game_session_operations DROP COLUMN session_id;
ALTER TABLE voice_room_instances DROP CONSTRAINT voice_room_instances_closed_at_check;
ALTER TABLE voice_room_instances ADD CONSTRAINT voice_room_instances_closed_at_check CHECK (
    (state = 'active' AND closed_at IS NULL)
    OR (state = 'closed' AND closed_at IS NOT NULL)
);
ALTER TABLE voice_room_instances DROP CONSTRAINT voice_room_instances_state_check;
ALTER TABLE voice_room_instances ADD CONSTRAINT voice_room_instances_state_check CHECK (state IN ('active', 'closed'));

COMMIT;
