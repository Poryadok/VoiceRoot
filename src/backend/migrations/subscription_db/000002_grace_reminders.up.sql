-- subscription_db v2 — track grace reminder days already emitted (D1/D3/D7)
ALTER TABLE subscriptions
	ADD COLUMN IF NOT EXISTS grace_reminders_sent INT[] NOT NULL DEFAULT '{}';

ALTER TABLE space_subscriptions
	ADD COLUMN IF NOT EXISTS grace_reminders_sent INT[] NOT NULL DEFAULT '{}';
