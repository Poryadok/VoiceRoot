ALTER TABLE federation_placements
 ADD CONSTRAINT federation_placements_space_node_unique UNIQUE(space_id,node_id);

CREATE TABLE federation_hosted_resources (
 resource_id UUID NOT NULL,
 routing_generation BIGINT NOT NULL CHECK (routing_generation>0),
 resource_type TEXT NOT NULL CHECK (resource_type IN ('space_content','chat','file','search_index','voice_room')),
 space_id UUID NOT NULL,
 home_node_id UUID NOT NULL,
 lifecycle_state TEXT NOT NULL CHECK (lifecycle_state IN ('active','frozen','purging','tombstoned')),
 capabilities TEXT[] NOT NULL CHECK (
   capabilities <@ ARRAY['messaging','file','search','voice']::TEXT[]
   AND cardinality(capabilities) BETWEEN 1 AND 4
 ),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(resource_id,routing_generation),
 FOREIGN KEY(space_id) REFERENCES federation_placements(space_id) ON DELETE RESTRICT,
 FOREIGN KEY(home_node_id) REFERENCES federation_nodes(id) ON DELETE RESTRICT
);
CREATE INDEX federation_hosted_resources_current
 ON federation_hosted_resources(resource_id,routing_generation DESC);

CREATE FUNCTION reject_federation_hosted_resource_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'hosted resource route generations are append-only';
END;
$$;
CREATE TRIGGER federation_hosted_resources_append_only
 BEFORE UPDATE OR DELETE ON federation_hosted_resources
 FOR EACH ROW EXECUTE FUNCTION reject_federation_hosted_resource_mutation();
