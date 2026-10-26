-- Unordered v1 projections are historical cleanup data, never v2 admission.
-- No ordering is guessed from timestamps, UUIDs or existing File identifiers.
ALTER TABLE catalog_asset_bindings RENAME TO catalog_legacy_asset_bindings;
ALTER TABLE catalog_legacy_asset_bindings RENAME CONSTRAINT catalog_asset_bindings_pkey TO catalog_legacy_asset_bindings_pkey;
ALTER INDEX catalog_asset_bindings_account RENAME TO catalog_legacy_asset_bindings_account;
ALTER INDEX catalog_asset_bindings_profile RENAME TO catalog_legacy_asset_bindings_profile;

-- Disposable cache incarnations contain no raw actor/provider/URL/operation.
-- File owns the permanent selection head and its high-watermark.
CREATE TABLE catalog_selector_slots (
    id UUID PRIMARY KEY,
    account_gate_id UUID NOT NULL,
    account_kind SMALLINT NOT NULL DEFAULT 1 CHECK (account_kind = 1),
    profile_gate_id UUID NOT NULL,
    profile_kind SMALLINT NOT NULL DEFAULT 2 CHECK (profile_kind = 2),
    account_generation UUID NOT NULL,
    profile_generation UUID NOT NULL,
    selection_revision BIGINT NOT NULL DEFAULT 0 CHECK (selection_revision >= 0),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (account_gate_id, account_kind) REFERENCES catalog_actor_gates(id, actor_kind),
    FOREIGN KEY (profile_gate_id, profile_kind) REFERENCES catalog_actor_gates(id, actor_kind)
);
CREATE INDEX catalog_selector_slots_profile ON catalog_selector_slots(profile_gate_id);
CREATE INDEX catalog_selector_slots_eviction ON catalog_selector_slots(selection_revision, last_used_at, id);

CREATE TABLE catalog_selector_aliases (
    key_id TEXT NOT NULL REFERENCES catalog_actor_keys(key_id),
    selector_hmac BYTEA NOT NULL CHECK (octet_length(selector_hmac) = 32),
    slot_id UUID NOT NULL REFERENCES catalog_selector_slots(id) ON DELETE CASCADE,
    PRIMARY KEY (key_id, selector_hmac)
);
CREATE INDEX catalog_selector_aliases_slot ON catalog_selector_aliases(slot_id);

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
    selector_slot_id UUID NOT NULL UNIQUE REFERENCES catalog_selector_slots(id) ON DELETE CASCADE,
    selection_revision BIGINT NOT NULL CHECK (selection_revision > 0),
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (account_gate_id, account_kind) REFERENCES catalog_actor_gates(id, actor_kind),
    FOREIGN KEY (profile_gate_id, profile_kind) REFERENCES catalog_actor_gates(id, actor_kind)
);
CREATE INDEX catalog_asset_bindings_account ON catalog_asset_bindings(account_id);
CREATE INDEX catalog_asset_bindings_profile ON catalog_asset_bindings(profile_id);
