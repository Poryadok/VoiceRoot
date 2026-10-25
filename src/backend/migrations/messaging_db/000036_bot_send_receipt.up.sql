-- Preserve the distinct signed wrapper identity for protected Bot sends.
ALTER TABLE message_send_receipts
  DROP CONSTRAINT message_send_receipts_request_rpc_check,
  ADD CONSTRAINT message_send_receipts_request_rpc_check CHECK (request_rpc IN (
    '/voice.messaging.v1.MessagingService/SendMessage',
    '/voice.messaging.v1.MessagingService/SendGameEventMessage',
    '/voice.messaging.v1.MessagingService/SendBotMessage'
  ));
