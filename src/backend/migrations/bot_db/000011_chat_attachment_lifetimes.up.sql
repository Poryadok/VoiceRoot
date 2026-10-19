BEGIN;
-- Local lifetime identity is distinct from a Chat incarnation, read operation,
-- native generation or wall-clock timestamp. Original provenance never changes.
CREATE TABLE bot_chat_attachments (
    attachment_id UUID PRIMARY KEY CHECK (attachment_id <> '00000000-0000-0000-0000-000000000000'),
    bot_id UUID NOT NULL,
    space_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    installation_id UUID,
    intent_operation_id UUID,
    origin_scope_observation_id UUID REFERENCES bot_chat_scope_observations(observation_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (intent_operation_id,chat_id)
);
CREATE INDEX bot_chat_attachments_target_idx ON bot_chat_attachments(bot_id,space_id,chat_id);
ALTER TABLE bot_installation_intents ADD COLUMN chat_attachment_ids JSONB NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(chat_attachment_ids)='array');
ALTER TABLE bot_chat_whitelist ADD COLUMN attachment_id UUID REFERENCES bot_chat_attachments(attachment_id);

-- Capture only actual surviving local rows. The narrow native join reuses an
-- already persisted original read/installation binding; it invents no operation
-- or generation and proves no historical Space membership or unknown origin.
INSERT INTO bot_chat_attachments(attachment_id,bot_id,space_id,chat_id,installation_id,intent_operation_id,origin_scope_observation_id)
SELECT gen_random_uuid(),w.bot_id,w.space_id,w.chat_id,COALESCE(w.source_installation_id,i.id),n.operation_id,w.origin_scope_observation_id
FROM bot_chat_whitelist w LEFT JOIN bot_space_installations i ON i.bot_id=w.bot_id AND i.space_id=w.space_id
LEFT JOIN bot_chat_scope_observations o ON o.observation_id=w.origin_scope_observation_id AND o.bot_id=w.bot_id AND o.space_id=w.space_id AND o.chat_id=w.chat_id AND o.origin_kind='install_admission'
LEFT JOIN bot_installation_intents n ON n.operation_id=o.intent_operation_id AND n.bot_id=w.bot_id AND n.space_id=w.space_id AND n.installation_id=w.source_installation_id AND n.chat_scope_observation_ids ? o.observation_id::text;
UPDATE bot_chat_whitelist w SET attachment_id=a.attachment_id
FROM bot_chat_attachments a WHERE a.bot_id=w.bot_id AND a.space_id=w.space_id AND a.chat_id=w.chat_id;
COMMIT;
