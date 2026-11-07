DROP TABLE thread_list_membership_snapshot_pairs;
DROP TABLE thread_list_membership_retired_incarnations;
DROP TABLE thread_list_membership_baselines;
ALTER TABLE thread_list_membership_heads
    DROP CONSTRAINT thread_membership_baseline_prefix,
    DROP COLUMN baseline_snapshot_id,
    DROP COLUMN baseline_q;
