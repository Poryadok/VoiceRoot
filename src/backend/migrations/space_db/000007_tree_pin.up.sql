-- space_db v7 — tree pin columns (docs/microservices/space-service.md)

ALTER TABLE space_tree_nodes
    ADD COLUMN is_pinned BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN pin_order INTEGER NULL;

DROP INDEX IF EXISTS space_tree_nodes_space_sort_idx;

CREATE INDEX space_tree_nodes_space_sort_idx ON space_tree_nodes (
    space_id,
    category_id,
    is_pinned DESC,
    pin_order NULLS LAST,
    sort_order
);
