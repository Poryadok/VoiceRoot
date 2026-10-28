-- Local consumer state only. Producer transport/capability is not activated.
CREATE TABLE matchmaking_terminal_routing_heads (
    fence_id TEXT PRIMARY KEY,
    current_json JSONB NOT NULL
);

CREATE TABLE matchmaking_terminal_routing_cycles (
    fence_id TEXT NOT NULL,
    cycle_id TEXT NOT NULL,
    purge_at TIMESTAMPTZ NOT NULL,
    lifecycle_json JSONB NOT NULL,
    routing_erased BOOLEAN NOT NULL DEFAULT FALSE,
    coverage_revision NUMERIC(20,0),
    coverage_sha256 BYTEA,
    PRIMARY KEY (fence_id, cycle_id),
    CHECK (coverage_revision IS NULL OR (coverage_revision > 0 AND coverage_revision <= 18446744073709551615)),
    CHECK (coverage_sha256 IS NULL OR octet_length(coverage_sha256)=32)
);
CREATE INDEX matchmaking_terminal_routing_due ON matchmaking_terminal_routing_cycles(purge_at, fence_id, cycle_id);

ALTER TABLE matchmaking_terminal_search_releases
    ADD COLUMN routing_fence_id TEXT,
    ADD COLUMN routing_cycle_id TEXT,
    ADD COLUMN routing_purge_at TIMESTAMPTZ,
    ADD COLUMN projection_claim UUID,
    ADD COLUMN projection_claim_until TIMESTAMPTZ,
    ADD CONSTRAINT terminal_routing_binding_complete CHECK (
        (routing_fence_id IS NULL AND routing_cycle_id IS NULL AND routing_purge_at IS NULL) OR
        (routing_fence_id IS NOT NULL AND routing_cycle_id IS NOT NULL AND routing_purge_at IS NOT NULL)
    );
