-- Only active File projections contain personal/catalog data. Gate and event
-- aliases are restricted purpose-HMAC anti-resurrection state, not public IDs.
CREATE TABLE catalog_actor_keys (
    key_id TEXT PRIMARY KEY,
    fingerprint BYTEA NOT NULL CHECK (octet_length(fingerprint) = 32)
);

CREATE TABLE catalog_actor_gates (
    id UUID PRIMARY KEY,
    actor_kind SMALLINT NOT NULL CHECK (actor_kind IN (1, 2)),
    generation UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('LIVE', 'CLOSED')),
    authority_epoch BIGINT NOT NULL DEFAULT 0 CHECK (authority_epoch >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, actor_kind)
);

CREATE TABLE catalog_actor_aliases (
    key_id TEXT NOT NULL REFERENCES catalog_actor_keys(key_id),
    actor_kind SMALLINT NOT NULL CHECK (actor_kind IN (1, 2)),
    actor_hmac BYTEA NOT NULL CHECK (octet_length(actor_hmac) = 32),
    gate_id UUID NOT NULL,
    PRIMARY KEY (key_id, actor_kind, actor_hmac),
    FOREIGN KEY (gate_id, actor_kind) REFERENCES catalog_actor_gates(id, actor_kind)
);

CREATE TABLE catalog_actor_event_fences (
    key_id TEXT NOT NULL REFERENCES catalog_actor_keys(key_id),
    event_hmac BYTEA NOT NULL CHECK (octet_length(event_hmac) = 32),
    account_gate_id UUID NOT NULL,
    account_kind SMALLINT NOT NULL DEFAULT 1 CHECK (account_kind = 1),
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (key_id, event_hmac),
    FOREIGN KEY (account_gate_id, account_kind) REFERENCES catalog_actor_gates(id, actor_kind)
);

CREATE TABLE catalog_asset_bindings (
    file_id UUID PRIMARY KEY,
    source_id UUID NOT NULL,
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    provider TEXT NOT NULL CHECK (provider <> ''),
    provider_id TEXT NOT NULL CHECK (provider_id <> ''),
    account_gate_id UUID NOT NULL,
    account_kind SMALLINT NOT NULL DEFAULT 1 CHECK (account_kind = 1),
    profile_gate_id UUID NOT NULL,
    profile_kind SMALLINT NOT NULL DEFAULT 2 CHECK (profile_kind = 2),
    account_generation UUID NOT NULL,
    profile_generation UUID NOT NULL,
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (account_gate_id, account_kind) REFERENCES catalog_actor_gates(id, actor_kind),
    FOREIGN KEY (profile_gate_id, profile_kind) REFERENCES catalog_actor_gates(id, actor_kind)
);
CREATE INDEX catalog_asset_bindings_account ON catalog_asset_bindings(account_id);
CREATE INDEX catalog_asset_bindings_profile ON catalog_asset_bindings(profile_id);
