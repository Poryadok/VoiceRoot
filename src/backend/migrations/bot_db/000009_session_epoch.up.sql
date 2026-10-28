ALTER TABLE bots ADD COLUMN session_epoch BIGINT NOT NULL DEFAULT 1 CHECK (session_epoch > 0);
ALTER TABLE bot_event_log ADD COLUMN session_epoch BIGINT CHECK (session_epoch > 0);
ALTER TABLE bot_event_log ADD COLUMN interaction_completed_at TIMESTAMPTZ;

-- Legacy interaction capabilities have no trusted issuing epoch. Ordinary
-- message recipients are not capabilities and must survive credential upkeep.
UPDATE bot_event_log SET delivery_status='canceled', payload='{}'::jsonb,
    claimed_until=NULL
WHERE event_type <> 'message' AND delivery_status IN ('pending','deferred');

CREATE INDEX bot_autocomplete_result_idx ON bot_event_log
    (bot_id,session_epoch,space_id,(payload->>'invoker_profile_id'),(payload->>'cache_key'),created_at DESC)
WHERE event_type='autocomplete' AND delivery_status='delivered' AND payload ? 'autocomplete_choices';

CREATE INDEX bot_event_session_idx ON bot_event_log(bot_id,session_epoch,id)
WHERE event_type <> 'message' AND delivery_status IN ('pending','deferred');
