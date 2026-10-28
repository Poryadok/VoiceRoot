-- Story owns recovery of every attempted File claim until publication commits.
CREATE TABLE story_media_reference_intents (
    story_id UUID PRIMARY KEY,
    media_file_id UUID NOT NULL,
    available_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX story_media_reference_intents_ready ON story_media_reference_intents(available_at,story_id);

-- No FK: archive cleanup may remove the Story while delete replay remains valid.
CREATE TABLE story_deletion_receipts (
    story_id UUID PRIMARY KEY,
    author_profile_id UUID NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX story_deletion_receipts_cleanup ON story_deletion_receipts(completed_at,story_id);
