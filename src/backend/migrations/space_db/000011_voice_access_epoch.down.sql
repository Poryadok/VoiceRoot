-- This DOWN is intentionally non-destructive. Epoch or owning-scope evidence
-- requires a separate reviewed archival/export-and-delete procedure.
DO $$
BEGIN
    -- Lock owning tables before evidence tables: mutations already hold their
    -- owning-table lock before their epoch/outbox trigger runs. The locks and
    -- the post-wait evidence check remain in this same transaction through
    -- either refusal or the final drop.
    LOCK TABLE spaces, space_members, voice_rooms,
               space_voice_access_epochs, space_voice_access_outbox
        IN ACCESS EXCLUSIVE MODE;

    IF EXISTS (SELECT 1 FROM space_voice_access_epochs)
       OR EXISTS (SELECT 1 FROM space_voice_access_outbox)
       OR EXISTS (SELECT 1 FROM spaces)
       OR EXISTS (SELECT 1 FROM space_members)
       OR EXISTS (SELECT 1 FROM voice_rooms)
    THEN
        RAISE EXCEPTION 'refusing to remove Space Voice access epoch contract while durable evidence exists'
            USING ERRCODE = '55000';
    END IF;

    DROP TRIGGER space_voice_access_outbox_no_change ON space_voice_access_outbox;
    DROP TRIGGER space_voice_access_room_change ON voice_rooms;
    DROP TRIGGER space_voice_access_member_change ON space_members;
    DROP TRIGGER space_voice_access_space_delete ON spaces;
    DROP TRIGGER space_voice_access_space_initialize ON spaces;

    DROP FUNCTION space_voice_access_outbox_immutable();
    DROP FUNCTION space_voice_access_space_deleted();
    DROP FUNCTION space_voice_access_room_changed();
    DROP FUNCTION space_voice_access_member_changed();
    DROP FUNCTION space_voice_access_bump(UUID, UUID, UUID);
    DROP FUNCTION space_voice_access_initialize();

    DROP TABLE space_voice_access_outbox;
    DROP TABLE space_voice_access_epochs;
END
$$;
