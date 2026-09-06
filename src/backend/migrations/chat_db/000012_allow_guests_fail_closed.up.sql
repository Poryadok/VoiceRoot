-- Existing chats retain their setting. New standalone chats must opt in to guest admission.
ALTER TABLE chats
  ALTER COLUMN allow_guests SET DEFAULT false;
