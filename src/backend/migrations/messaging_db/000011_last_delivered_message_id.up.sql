-- Durable delivery cursor for DM list ticks (docs/microservices/messaging-service.md § Durable delivery).
ALTER TABLE read_receipts
  ADD COLUMN last_delivered_message_id UUID NULL;
