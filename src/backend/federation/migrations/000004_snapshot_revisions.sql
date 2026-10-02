CREATE TABLE federation_snapshot_revision_events (
 space_id UUID NOT NULL,
 node_id UUID NOT NULL,
 revision BIGINT NOT NULL CHECK (revision > 0),
 snapshot_hash TEXT NOT NULL CHECK (length(snapshot_hash) = 64),
 valid_until TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (space_id,node_id,revision),
 FOREIGN KEY (space_id,node_id) REFERENCES federation_placements(space_id,node_id) ON DELETE RESTRICT
);

INSERT INTO federation_snapshot_revision_events(space_id,node_id,revision,snapshot_hash,valid_until)
SELECT space_id,node_id,revision,snapshot_hash,valid_until
FROM federation_placements
WHERE revision > 0 AND snapshot IS NOT NULL AND snapshot_hash IS NOT NULL AND valid_until IS NOT NULL
ON CONFLICT (space_id,node_id,revision) DO NOTHING;

CREATE INDEX federation_snapshot_revision_events_created
 ON federation_snapshot_revision_events(space_id,node_id,revision);
