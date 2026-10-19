-- Roll back only while successor activation is disabled and no root publisher
-- or GC worker is running. All nodes are rebuildable projection data.
DROP TABLE IF EXISTS thread_list_nodes;
DROP FUNCTION IF EXISTS thread_list_nodes_reject_update();
