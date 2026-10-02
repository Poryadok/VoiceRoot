-- Existing room-less routes stay intact and cannot authorize media. Their
-- canonical owner must append a new generation with an explicit RTC name.
ALTER TABLE federation_hosted_resources ADD COLUMN room_name TEXT NOT NULL DEFAULT '';
ALTER TABLE federation_hosted_resources ADD CONSTRAINT federation_hosted_room_name
 CHECK (room_name='' OR (resource_type='voice_room' AND octet_length(room_name) BETWEEN 1 AND 256));
-- Preserve historical rows while enforcing placement for every new generation.
ALTER TABLE federation_hosted_resources ADD CONSTRAINT federation_hosted_exact_placement
 FOREIGN KEY(space_id,home_node_id) REFERENCES federation_placements(space_id,node_id) NOT VALID;

CREATE TABLE federation_voice_room_bindings (
 resource_id UUID PRIMARY KEY,
 node_id UUID NOT NULL REFERENCES federation_nodes(id) ON DELETE RESTRICT,
 space_id UUID NOT NULL,
 room_name TEXT NOT NULL CHECK (octet_length(room_name) BETWEEN 1 AND 256),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(node_id,room_name),
 FOREIGN KEY(space_id,node_id) REFERENCES federation_placements(space_id,node_id) ON DELETE RESTRICT
);
CREATE TRIGGER federation_voice_room_bindings_append_only
 BEFORE UPDATE OR DELETE ON federation_voice_room_bindings
 FOR EACH ROW EXECUTE FUNCTION reject_federation_hosted_resource_mutation();
