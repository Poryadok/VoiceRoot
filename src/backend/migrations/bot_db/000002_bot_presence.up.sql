CREATE TABLE IF NOT EXISTS bot_presence (
	bot_id UUID PRIMARY KEY REFERENCES bots(id) ON DELETE CASCADE,
	last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS bot_presence_last_seen_idx ON bot_presence (last_seen_at);

CREATE TABLE IF NOT EXISTS bot_daily_chat_creates (
	bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
	day DATE NOT NULL DEFAULT CURRENT_DATE,
	count INT NOT NULL DEFAULT 0,
	PRIMARY KEY (bot_id, day)
);
