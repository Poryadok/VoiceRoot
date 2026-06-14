-- space_db v5 — local Space Pro entitlement cache (synced from Subscription service)
CREATE TABLE IF NOT EXISTS space_subscriptions (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	space_id UUID NOT NULL,
	purchaser_account_id UUID NOT NULL,
	plan TEXT NOT NULL DEFAULT 'space_pro',
	status TEXT NOT NULL,
	provider TEXT NOT NULL DEFAULT 'paddle',
	provider_subscription_id TEXT NOT NULL DEFAULT '',
	current_period_start TIMESTAMPTZ NOT NULL DEFAULT now(),
	current_period_end TIMESTAMPTZ NOT NULL DEFAULT now(),
	grace_period_end TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS space_subscriptions_space_id_idx ON space_subscriptions (space_id);
