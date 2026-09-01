CREATE TABLE folder_chats (
  profile_id UUID NOT NULL,
  folder_id UUID NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  chat_id UUID NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  is_pinned BOOLEAN NOT NULL DEFAULT false,
  pin_order INTEGER NULL,
  added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, folder_id, chat_id)
);

CREATE INDEX folder_chats_folder_pin_idx ON folder_chats (profile_id, folder_id, is_pinned DESC, pin_order NULLS LAST, sort_order);
