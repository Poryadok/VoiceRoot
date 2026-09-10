-- This DOWN is intentionally non-destructive. Epoch, outbox or owning Role
-- evidence requires a separate reviewed archival/export-and-delete procedure.
DO $$
BEGIN
    -- Lock owning tables before evidence tables: mutations already hold their
    -- owning-table lock before their epoch/outbox trigger runs. The locks and
    -- the post-wait evidence check remain in this same transaction through
    -- either refusal or the final drop.
    LOCK TABLE roles, member_roles, voice_room_overrides,
               ownership_transfer_v2, role_space_lifecycle,
               role_voice_policy_epochs, role_voice_policy_outbox
        IN ACCESS EXCLUSIVE MODE;

    IF EXISTS (SELECT 1 FROM role_voice_policy_epochs)
       OR EXISTS (SELECT 1 FROM role_voice_policy_outbox)
       OR EXISTS (SELECT 1 FROM roles)
       OR EXISTS (SELECT 1 FROM member_roles)
       OR EXISTS (SELECT 1 FROM voice_room_overrides)
       OR EXISTS (SELECT 1 FROM ownership_transfer_v2)
       OR EXISTS (SELECT 1 FROM role_space_lifecycle)
    THEN
        RAISE EXCEPTION 'refusing to remove Role Voice policy epoch contract while durable evidence exists'
            USING ERRCODE = '55000';
    END IF;

    DROP TRIGGER role_voice_policy_outbox_no_change ON role_voice_policy_outbox;
    DROP TRIGGER role_voice_policy_lifecycle_change ON role_space_lifecycle;
    DROP TRIGGER role_voice_policy_ownership_change ON ownership_transfer_v2;
    DROP TRIGGER role_voice_policy_voice_override_change ON voice_room_overrides;
    DROP TRIGGER role_voice_policy_member_change ON member_roles;
    DROP TRIGGER role_voice_policy_role_change ON roles;

    DROP FUNCTION role_voice_policy_outbox_immutable();
    DROP FUNCTION role_voice_policy_lifecycle_changed();
    DROP FUNCTION role_voice_policy_ownership_changed();
    DROP FUNCTION role_voice_policy_voice_override_changed();
    DROP FUNCTION role_voice_policy_member_changed();
    DROP FUNCTION role_voice_policy_role_changed();
    DROP FUNCTION role_voice_policy_bump(UUID, UUID, UUID);

    DROP TABLE role_voice_policy_outbox;
    DROP TABLE role_voice_policy_epochs;
END
$$;
