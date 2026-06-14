-- Phase 15: DM opt-in E2E flag on chats (docs/features/encryption.md).
ALTER TABLE chats
    ADD COLUMN IF NOT EXISTS e2e_enabled BOOLEAN NOT NULL DEFAULT false;
