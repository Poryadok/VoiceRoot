-- space_db v2 — tree: categories, voice_rooms, space_tree_nodes (docs/microservices/space-service.md)

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX categories_space_id_sort_idx ON categories (space_id, sort_order);

CREATE TABLE voice_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX voice_rooms_space_id_idx ON voice_rooms (space_id);

CREATE TABLE space_tree_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    category_id UUID NULL REFERENCES categories (id) ON DELETE SET NULL,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('text_chat', 'voice_room')),
    chat_id UUID NULL,
    voice_room_id UUID NULL REFERENCES voice_rooms (id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT space_tree_nodes_kind_ref_check CHECK (
        (kind = 'text_chat' AND chat_id IS NOT NULL AND voice_room_id IS NULL)
        OR (kind = 'voice_room' AND voice_room_id IS NOT NULL AND chat_id IS NULL)
    )
);

CREATE INDEX space_tree_nodes_space_sort_idx ON space_tree_nodes (space_id, category_id, sort_order);
CREATE UNIQUE INDEX space_tree_nodes_voice_room_uidx ON space_tree_nodes (voice_room_id) WHERE voice_room_id IS NOT NULL;
CREATE UNIQUE INDEX space_tree_nodes_chat_uidx ON space_tree_nodes (chat_id) WHERE chat_id IS NOT NULL;
