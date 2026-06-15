CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS bots (
	id UUID PRIMARY KEY,
	owner_account_id UUID NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	avatar_url TEXT,
	token_hash TEXT NOT NULL,
	webhook_url TEXT,
	webhook_secret TEXT NOT NULL,
	is_polling_mode BOOLEAN NOT NULL DEFAULT false,
	scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
	status TEXT NOT NULL DEFAULT 'live',
	actor_profile_id UUID NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS bots_owner_account_id_idx ON bots (owner_account_id);

CREATE TABLE IF NOT EXISTS bot_commands (
	id UUID PRIMARY KEY,
	bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	parameters JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (bot_id, name)
);

CREATE TABLE IF NOT EXISTS bot_space_installations (
	id UUID PRIMARY KEY,
	bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
	space_id UUID NOT NULL,
	installed_by_profile_id UUID NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (bot_id, space_id)
);

CREATE INDEX IF NOT EXISTS bot_space_installations_space_id_idx ON bot_space_installations (space_id);

CREATE TABLE IF NOT EXISTS bot_chat_whitelist (
	bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
	chat_id UUID NOT NULL,
	space_id UUID NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT true,
	added_by_profile_id UUID NOT NULL,
	added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (bot_id, chat_id)
);

CREATE INDEX IF NOT EXISTS bot_chat_whitelist_chat_id_idx ON bot_chat_whitelist (chat_id);

CREATE TABLE IF NOT EXISTS bot_event_log (
	id UUID PRIMARY KEY,
	bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	delivery_status TEXT NOT NULL DEFAULT 'pending',
	attempts INT NOT NULL DEFAULT 0,
	interaction_token TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	delivered_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS bot_event_log_bot_id_status_idx ON bot_event_log (bot_id, delivery_status, created_at);
