-- Phase 11 privacy_settings — docs/microservices/user-service.md, docs/features/privacy.md
CREATE TABLE privacy_settings (
    profile_id UUID PRIMARY KEY,
    preset VARCHAR(16) NOT NULL CHECK (preset IN ('personal', 'gaming', 'work')),
    show_online VARCHAR(32) NOT NULL,
    show_game_status VARCHAR(32) NOT NULL,
    show_mm_rating VARCHAR(32) NOT NULL,
    show_phone VARCHAR(32) NOT NULL,
    show_stories VARCHAR(32) NOT NULL,
    allow_dm VARCHAR(32) NOT NULL CHECK (allow_dm IN ('everyone', 'friends', 'friends_of_friends', 'nobody')),
    allow_friend_requests VARCHAR(32) NOT NULL CHECK (allow_friend_requests IN ('everyone', 'friends_of_friends', 'nobody')),
    allow_guest_dm BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
