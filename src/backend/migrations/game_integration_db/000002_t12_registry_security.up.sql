CREATE TABLE installations (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    callback_url TEXT NOT NULL CHECK (char_length(callback_url) BETWEEN 1 AND 2048),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE registry_audit
    ADD COLUMN source TEXT NOT NULL DEFAULT 'system',
    ADD COLUMN result TEXT NOT NULL DEFAULT 'success' CHECK (result IN ('success', 'denied')),
    ADD COLUMN reason_code TEXT,
    ADD COLUMN installation_id UUID REFERENCES installations(id),
    ADD COLUMN quota_window_start TIMESTAMPTZ,
    ADD COLUMN denial_count INTEGER NOT NULL DEFAULT 1 CHECK (denial_count > 0);

UPDATE registry_audit
SET source = 'operator_approved',
    result = 'success',
    reason_code = COALESCE(reason_code, 'sandbox_admission_approved')
WHERE actor_kind = 'operator'
  AND action = 'approve_sandbox'
  AND source = 'system'
  AND result = 'success';

CREATE UNIQUE INDEX registry_audit_quota_denial_window_idx
    ON registry_audit (application_id, quota_window_start)
    WHERE action = 'quota_denied';

CREATE TABLE app_quota_windows (
    application_id UUID PRIMARY KEY REFERENCES applications(id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    request_count INTEGER NOT NULL CHECK (request_count BETWEEN 1 AND 120),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE applications
    ADD COLUMN suspended_from_status TEXT
        CHECK (suspended_from_status IN ('sandbox', 'active'));

ALTER TABLE registry_operations
    ADD COLUMN result_status TEXT,
    ADD COLUMN result_updated_at TIMESTAMPTZ;
