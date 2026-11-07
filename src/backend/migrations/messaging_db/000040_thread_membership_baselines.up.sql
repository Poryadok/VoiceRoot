-- A sealed Chat G/Q baseline may cover events never received by this inbox.
-- Existing origin-replay writers remain compatible with the zero default.
ALTER TABLE thread_list_membership_heads
    ADD COLUMN baseline_q bigint NOT NULL DEFAULT 0,
    ADD COLUMN baseline_snapshot_id uuid,
    ADD CONSTRAINT thread_membership_baseline_prefix CHECK (baseline_q >= 0 AND baseline_q <= applied_q);

CREATE TABLE thread_list_membership_baselines (
    chat_id uuid NOT NULL REFERENCES thread_list_membership_heads(chat_id),
    snapshot_id uuid NOT NULL,
    incarnation uuid NOT NULL,
    generation uuid NOT NULL,
    journal_offset bigint NOT NULL CHECK (journal_offset >= 0),
    pin_id uuid NOT NULL,
    retain_until timestamptz NOT NULL,
    member_count bigint NOT NULL CHECK (member_count >= 0),
    content_digest bytea NOT NULL CHECK (octet_length(content_digest)=32),
    binding_digest bytea NOT NULL CHECK (octet_length(binding_digest)=32),
    installed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id,snapshot_id)
);

CREATE INDEX thread_membership_generation ON thread_list_membership_baselines(chat_id,incarnation,generation);

-- A cached older capability must never make a previously retired Chat source
-- identity current again, even if a worker supplies a fresh local checkpoint.
CREATE TABLE thread_list_membership_retired_incarnations (
    chat_id uuid NOT NULL REFERENCES thread_list_membership_heads(chat_id),
    incarnation uuid NOT NULL,
    PRIMARY KEY (chat_id,incarnation)
);

-- The current complete baseline retains negative/tombstone facts and source
-- vectors even for pairs that have no received membership event. Omission can
-- revoke a previously known pair, but cannot manufacture an owner revision.
CREATE TABLE thread_list_membership_snapshot_pairs (
    chat_id uuid NOT NULL REFERENCES thread_list_membership_heads(chat_id),
    profile_id uuid NOT NULL,
    incarnation uuid NOT NULL,
    member_state bytea NOT NULL CHECK (octet_length(member_state)>0),
    omitted boolean NOT NULL DEFAULT false,
    PRIMARY KEY (chat_id,profile_id)
);
