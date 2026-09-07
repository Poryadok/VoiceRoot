-- Legacy standalone chats were created while 000007 defaulted this flag to true.
-- Preserve memberships, but require an explicit owner/admin opt-in for every
-- future guest admission. Space chat admission remains owned by Space/Role.
UPDATE chats
SET allow_guests = false,
    updated_at = now()
WHERE space_id IS NULL
  AND type IN ('group', 'channel')
  AND allow_guests = true;

ALTER TABLE chats
  ALTER COLUMN allow_guests SET DEFAULT false;
