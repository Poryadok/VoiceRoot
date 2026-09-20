CREATE TABLE sticker_packs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    thumb_file_id UUID NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    creator_profile_id UUID NULL,
    sticker_count INTEGER NOT NULL DEFAULT 0 CHECK (sticker_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((is_system AND creator_profile_id IS NULL) OR (NOT is_system AND creator_profile_id IS NOT NULL))
);

CREATE TABLE stickers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pack_id UUID NOT NULL REFERENCES sticker_packs(id) ON DELETE CASCADE,
    file_id UUID NOT NULL,
    emoji TEXT NULL,
    sort_order INTEGER NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    UNIQUE (pack_id, file_id),
    UNIQUE (pack_id, sort_order)
);

CREATE TABLE profile_installed_packs (
    profile_id UUID NOT NULL,
    pack_id UUID NOT NULL REFERENCES sticker_packs(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, pack_id)
);

CREATE INDEX profile_installed_packs_profile_order_idx ON profile_installed_packs (profile_id, sort_order, installed_at);
