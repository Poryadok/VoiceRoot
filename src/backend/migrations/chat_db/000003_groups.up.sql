-- Phase 4 standalone groups — docs/PLAN.md, docs/microservices/chat-service.md
ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_type_check;
ALTER TABLE chats ADD CONSTRAINT chats_type_check CHECK (type IN ('dm', 'group', 'channel'));
