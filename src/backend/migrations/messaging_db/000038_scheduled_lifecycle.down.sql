DROP TRIGGER scheduled_release_send_namespace ON scheduled_messages;
DROP TRIGGER messages_release_send_namespace ON messages;
DROP FUNCTION messaging_release_send_namespace();
DROP TRIGGER scheduled_send_namespace ON scheduled_messages;
DROP TRIGGER messages_send_namespace ON messages;
DROP FUNCTION messaging_claim_send_namespace();
DROP TABLE message_send_namespace;
ALTER TABLE scheduled_messages DROP COLUMN dispatch_message_id, DROP COLUMN revision, DROP COLUMN request_sha256, DROP COLUMN request_rpc;
