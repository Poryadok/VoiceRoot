CREATE TABLE IF NOT EXISTS game_message_execution_permit_completions (
    permit_id UUID PRIMARY KEY,
    gis_permit_id UUID NOT NULL,
    operation_id UUID NOT NULL UNIQUE,
    request_sha256 CHAR(64) NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('committed', 'aborted')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK ((status = 'pending' AND completed_at IS NULL) OR (status = 'completed' AND completed_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS game_message_execution_permit_completions_pending_idx
    ON game_message_execution_permit_completions (created_at, permit_id)
    WHERE status = 'pending';
