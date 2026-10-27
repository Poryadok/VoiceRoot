ALTER TABLE scheduled_messages ADD COLUMN request_rpc TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduled_messages ADD COLUMN request_sha256 TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduled_messages ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE scheduled_messages ADD COLUMN dispatch_message_id UUID;

-- One transactional namespace covers every insert path, including attachment
-- recovery and internal publishers. Existing historical keys retain ownership.
CREATE TABLE message_send_namespace (
 chat_id UUID NOT NULL,
 sender_profile_id UUID NOT NULL,
 client_message_id UUID NOT NULL,
 result_kind TEXT NOT NULL CHECK (result_kind IN ('message','schedule')),
 PRIMARY KEY(chat_id,sender_profile_id,client_message_id)
);
INSERT INTO message_send_namespace SELECT chat_id,sender_profile_id,client_message_id,'message' FROM messages WHERE client_message_id IS NOT NULL;
INSERT INTO message_send_namespace SELECT chat_id,sender_profile_id,client_message_id,'schedule' FROM scheduled_messages WHERE client_message_id IS NOT NULL;
CREATE FUNCTION messaging_claim_send_namespace() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE claimed TEXT;
BEGIN
 IF NEW.client_message_id IS NULL THEN RETURN NEW; END IF;
 INSERT INTO message_send_namespace VALUES(NEW.chat_id,NEW.sender_profile_id,NEW.client_message_id,TG_ARGV[0]) ON CONFLICT DO NOTHING;
 SELECT result_kind INTO claimed FROM message_send_namespace WHERE chat_id=NEW.chat_id AND sender_profile_id=NEW.sender_profile_id AND client_message_id=NEW.client_message_id;
 IF claimed <> TG_ARGV[0] THEN RAISE EXCEPTION 'send namespace conflict' USING ERRCODE='23505', CONSTRAINT='message_send_namespace_kind'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER messages_send_namespace BEFORE INSERT ON messages FOR EACH ROW EXECUTE FUNCTION messaging_claim_send_namespace('message');
CREATE TRIGGER scheduled_send_namespace BEFORE INSERT ON scheduled_messages FOR EACH ROW EXECUTE FUNCTION messaging_claim_send_namespace('schedule');

-- The namespace has the same physical lifetime as its owning result. It must
-- not retain profile/chat/client identifiers after the result is purged.
CREATE FUNCTION messaging_release_send_namespace() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.client_message_id IS NOT NULL THEN
  DELETE FROM message_send_namespace WHERE chat_id=OLD.chat_id AND sender_profile_id=OLD.sender_profile_id AND client_message_id=OLD.client_message_id AND result_kind=TG_ARGV[0];
 END IF;
 RETURN OLD;
END $$;
CREATE TRIGGER messages_release_send_namespace AFTER DELETE ON messages FOR EACH ROW EXECUTE FUNCTION messaging_release_send_namespace('message');
CREATE TRIGGER scheduled_release_send_namespace AFTER DELETE ON scheduled_messages FOR EACH ROW EXECUTE FUNCTION messaging_release_send_namespace('schedule');
