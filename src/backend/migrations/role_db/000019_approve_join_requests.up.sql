-- Frozen permission bit42; separate from settings/invites/ban authority.
UPDATE roles SET permissions=permissions | (1::bigint << 42),updated_at=clock_timestamp()
WHERE is_system AND name IN ('Owner','Admin','Moderator');
