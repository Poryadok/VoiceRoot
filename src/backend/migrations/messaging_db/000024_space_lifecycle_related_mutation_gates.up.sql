CREATE OR REPLACE FUNCTION messaging_assert_space_chat_write(target_chat_id UUID, allow_purge_delete BOOLEAN) RETURNS VOID AS $$
DECLARE
    lifecycle_space UUID;
    lifecycle_operation UUID;
    lifecycle_state TEXT;
BEGIN
    FOR lifecycle_space IN
        SELECT DISTINCT item.space_id
        FROM messaging_space_chat_manifest_items item
        WHERE item.chat_id = target_chat_id
        ORDER BY item.space_id
    LOOP
        PERFORM pg_advisory_xact_lock((
            'x' || encode(substring(digest(
                convert_to('voice.space.mutation.v1', 'UTF8') || decode('00', 'hex') || uuid_send(lifecycle_space),
                'sha256'
            ) FROM 1 FOR 8), 'hex')
        )::bit(64)::bigint);
        SELECT fence.state, fence.deletion_operation_id
        INTO lifecycle_state, lifecycle_operation
        FROM messaging_space_lifecycle_fences fence
        WHERE fence.space_id = lifecycle_space;

        IF lifecycle_state IN ('FROZEN', 'PURGE_DECIDED', 'PURGED') THEN
            IF allow_purge_delete
               AND current_setting('voice.messaging_purge_space_id', TRUE) = lifecycle_space::text
               AND current_setting('voice.messaging_purge_operation_id', TRUE) = lifecycle_operation::text THEN
                CONTINUE;
            END IF;
            RAISE EXCEPTION 'Messaging Space lifecycle fence blocks chat mutation'
                USING ERRCODE = '55000';
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION messaging_guard_related_chat_mutation() RETURNS trigger AS $$
DECLARE
    affected_chat_id UUID;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        PERFORM messaging_assert_space_chat_write(OLD.chat_id, TG_OP = 'DELETE');
    END IF;
    IF TG_OP <> 'DELETE' THEN
        PERFORM messaging_assert_space_chat_write(NEW.chat_id, FALSE);
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION messaging_guard_reaction_mutation() RETURNS trigger AS $$
DECLARE
    target_message_id UUID;
    target_chat_id UUID;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        target_message_id := OLD.message_id;
        SELECT chat_id INTO target_chat_id FROM messages WHERE id = target_message_id;
        -- Parent-message cascades run after the parent row is gone. The
        -- message's BEFORE trigger already enforced purge authorization.
        IF target_chat_id IS NOT NULL THEN
            PERFORM messaging_assert_space_chat_write(target_chat_id, TG_OP = 'DELETE');
        END IF;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        target_message_id := NEW.message_id;
        SELECT chat_id INTO target_chat_id FROM messages WHERE id = target_message_id;
        IF target_chat_id IS NOT NULL THEN
            PERFORM messaging_assert_space_chat_write(target_chat_id, FALSE);
        END IF;
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER messaging_space_pins_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON pins
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_related_chat_mutation();
CREATE TRIGGER messaging_space_read_positions_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON read_positions
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_related_chat_mutation();
CREATE TRIGGER messaging_space_read_receipts_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON read_receipts
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_related_chat_mutation();
CREATE TRIGGER messaging_space_scheduled_messages_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON scheduled_messages
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_related_chat_mutation();
CREATE TRIGGER messaging_space_reactions_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON reactions
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_reaction_mutation();
CREATE TRIGGER messaging_space_message_hides_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON message_hides
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_reaction_mutation();
CREATE TRIGGER messaging_space_game_cards_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON message_game_cards
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_reaction_mutation();
CREATE TRIGGER messaging_space_game_action_results_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON message_game_action_results
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_reaction_mutation();
CREATE TRIGGER messaging_space_game_message_revisions_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON game_message_revisions
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_related_chat_mutation();
CREATE TRIGGER messaging_space_game_message_receipts_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON game_message_operation_receipts
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_related_chat_mutation();
CREATE TRIGGER messaging_space_game_message_tombstones_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON game_message_tombstone_actions
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_related_chat_mutation();
