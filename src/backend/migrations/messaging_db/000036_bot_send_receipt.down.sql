-- Refuse rollback while Bot receipts exist; never delete or relabel authority.
-- This single ALTER is atomic, retaining the current constraint on failure.
ALTER TABLE message_send_receipts
  DROP CONSTRAINT message_send_receipts_request_rpc_check,
  ADD CONSTRAINT message_send_receipts_request_rpc_check CHECK (request_rpc IN (
    '/voice.messaging.v1.MessagingService/SendMessage',
    '/voice.messaging.v1.MessagingService/SendGameEventMessage'
  ));
