-- Profile-level custom status is durable profile metadata. Presence custom_status remains
-- an ephemeral Redis value owned by UpdatePresence.
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS custom_status TEXT NULL;
