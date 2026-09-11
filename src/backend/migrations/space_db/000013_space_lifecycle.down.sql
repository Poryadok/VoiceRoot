-- Refuse to discard any durable lifecycle evidence, including a concurrent
-- insert that committed while this migration waited for its table locks.
BEGIN;
LOCK TABLE space_lifecycle_operations,
    space_lifecycle_aggregates,
    space_lifecycle_participants,
    space_lifecycle_outbox,
    space_deletion_tombstones IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM space_lifecycle_operations)
        OR EXISTS (SELECT 1 FROM space_lifecycle_aggregates)
        OR EXISTS (SELECT 1 FROM space_lifecycle_participants)
        OR EXISTS (SELECT 1 FROM space_lifecycle_outbox)
        OR EXISTS (SELECT 1 FROM space_deletion_tombstones) THEN
        RAISE EXCEPTION 'cannot remove durable Space lifecycle evidence';
    END IF;
END
$$;

DROP TABLE space_deletion_tombstones;
DROP TABLE space_lifecycle_outbox;
DROP TABLE space_lifecycle_participants;
DROP TABLE space_lifecycle_aggregates;
DROP TABLE space_lifecycle_operations;
COMMIT;
