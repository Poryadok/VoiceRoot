CREATE TABLE quick_access_chats (
  profile_id UUID NOT NULL,
  chat_id UUID NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, chat_id)
);

CREATE INDEX quick_access_profile_order_idx ON quick_access_chats (profile_id, sort_order);
