-- Messaging-owned immutable path-copy nodes for the ListThreads successor.
-- Root publication, membership/message journals and activation remain separate.
-- There are deliberately no cross-service or message-row foreign keys.
CREATE TABLE thread_list_nodes (
    node_id bytea PRIMARY KEY CHECK (octet_length(node_id) = 32),
    payload bytea NOT NULL CHECK (octet_length(payload) > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION thread_list_nodes_reject_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'thread list nodes are immutable';
END;
$$;

CREATE TRIGGER thread_list_nodes_immutable
BEFORE UPDATE ON thread_list_nodes
FOR EACH ROW EXECUTE FUNCTION thread_list_nodes_reject_update();

-- DELETE is reserved for reachability GC after READY heads and unexpired
-- cursor roots have been pinned. Age alone is never a deletion criterion.
