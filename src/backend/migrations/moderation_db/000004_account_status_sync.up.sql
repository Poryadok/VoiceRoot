-- Durable retries for reconciling Auth status from Moderation-owned effective bans.
CREATE TABLE moderation_account_status_sync (
  account_id uuid PRIMARY KEY,
  requested_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
