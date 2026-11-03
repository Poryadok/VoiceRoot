-- Messaging-owned projection of Chat's committed effective-membership prefix Q.
-- These tables grant no authority: page admission still requires fresh Chat proofs.
CREATE TABLE thread_list_membership_heads (
    chat_id uuid PRIMARY KEY,
    incarnation uuid NOT NULL,
    applied_q bigint NOT NULL DEFAULT 0 CHECK (applied_q >= 0),
    quarantined boolean NOT NULL DEFAULT false
);

CREATE TABLE thread_list_membership_inbox (
    chat_id uuid NOT NULL REFERENCES thread_list_membership_heads(chat_id),
    incarnation uuid NOT NULL,
    journal_offset bigint NOT NULL CHECK (journal_offset > 0),
    event_id uuid NOT NULL,
    profile_id uuid NOT NULL,
    membership_revision bigint NOT NULL CHECK (membership_revision > 0),
    payload bytea NOT NULL CHECK (octet_length(payload) > 0),
    PRIMARY KEY (chat_id, incarnation, journal_offset),
    UNIQUE (chat_id, incarnation, event_id),
    UNIQUE (chat_id, incarnation, profile_id, membership_revision)
);

CREATE TABLE thread_list_membership_pairs (
    chat_id uuid NOT NULL REFERENCES thread_list_membership_heads(chat_id),
    profile_id uuid NOT NULL,
    membership_revision bigint NOT NULL CHECK (membership_revision > 0),
    lineage uuid NOT NULL,
    status text NOT NULL CHECK (status IN ('BUILDING', 'REVOKED')),
    PRIMARY KEY (chat_id, profile_id)
);

-- Retain all retired lineages, not just the immediately preceding one.
CREATE TABLE thread_list_retired_lineages (
    chat_id uuid NOT NULL REFERENCES thread_list_membership_heads(chat_id),
    profile_id uuid NOT NULL,
    lineage uuid NOT NULL,
    PRIMARY KEY (chat_id, profile_id, lineage)
);
