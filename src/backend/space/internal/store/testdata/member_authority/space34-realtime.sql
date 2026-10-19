-- Participant IDs1-10 retain their meaning. Realtime11 is mandatory before
-- scheduling, destructive side effects and terminal local hard deletion.
ALTER TABLE space_lifecycle_participants DROP CONSTRAINT space_lifecycle_participants_participant_id_check;
ALTER TABLE space_lifecycle_participants ADD CONSTRAINT space_lifecycle_participants_participant_id_check CHECK(participant_id BETWEEN 1 AND 11);
ALTER TABLE space_lifecycle_completions DROP CONSTRAINT space_lifecycle_completions_participant_id_check;
ALTER TABLE space_lifecycle_completions ADD CONSTRAINT space_lifecycle_completions_participant_id_check CHECK(participant_id BETWEEN 1 AND 11);

CREATE OR REPLACE FUNCTION space_terminal_purge_ready(target UUID) RETURNS boolean
LANGUAGE sql STABLE AS $$
SELECT EXISTS (
    SELECT 1 FROM space_lifecycle_aggregates a
    JOIN space_deletion_tombstones t ON t.space_id=a.space_id
    WHERE a.space_id=target AND a.phase='PURGED' AND a.local_purge_completed
      AND clock_timestamp() >= a.purge_after
      AND (SELECT count(*) FROM space_lifecycle_participants p
           WHERE p.space_id=a.space_id AND p.deletion_operation_id=a.deletion_operation_id
             AND p.generation=a.generation AND p.progress='COMPLETE'
             AND ((p.request_kind='ROLE_RETIREMENT' AND p.participant_id=1)
               OR (p.request_kind='PURGE' AND p.participant_id BETWEEN 2 AND 11)))=11
)
$$;
