-- Soft-hide from feeds / non-author GetStory (stories.md §Модерация). Author retains access.

ALTER TABLE stories
    ADD COLUMN IF NOT EXISTS hidden_from_feed_at TIMESTAMPTZ;
