DROP INDEX IF EXISTS space_tree_nodes_space_sort_idx;

ALTER TABLE space_tree_nodes
    DROP COLUMN IF EXISTS pin_order,
    DROP COLUMN IF EXISTS is_pinned;

CREATE INDEX space_tree_nodes_space_sort_idx ON space_tree_nodes (space_id, category_id, sort_order);
