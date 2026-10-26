-- Messaging-owned views. Cross-service profile identifiers have no foreign key.
CREATE TABLE message_views (
    message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL,
    viewed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, profile_id)
);
CREATE INDEX message_views_profile_idx ON message_views(profile_id, message_id);

-- FOR_EVERYONE is a soft delete; erase personal view rows in that same transaction.
CREATE FUNCTION erase_deleted_message_views() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN
        DELETE FROM message_views WHERE message_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER messages_erase_views AFTER UPDATE OF deleted_at ON messages
FOR EACH ROW EXECUTE FUNCTION erase_deleted_message_views();
