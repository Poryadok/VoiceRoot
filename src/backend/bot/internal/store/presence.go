package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const DefaultPresenceTTL = 90 * time.Second

// TouchPresence records bot liveness (heartbeat / poll / webhook activity).
func (s *BotStore) TouchPresence(ctx context.Context, botID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO bot_presence (bot_id, last_seen_at) VALUES ($1, now())
ON CONFLICT (bot_id) DO UPDATE SET last_seen_at = now()`, botID)
	return err
}

// IsBotOnline reports whether the bot was seen within ttl.
func (s *BotStore) IsBotOnline(ctx context.Context, botID uuid.UUID, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = DefaultPresenceTTL
	}
	var last time.Time
	err := s.Pool.QueryRow(ctx, `SELECT last_seen_at FROM bot_presence WHERE bot_id = $1`, botID).Scan(&last)
	if err != nil {
		return false, nil
	}
	return time.Since(last) <= ttl, nil
}

// IncrementDailyChatCreates returns the new count for today; enforces platform limit externally.
func (s *BotStore) IncrementDailyChatCreates(ctx context.Context, botID uuid.UUID) (int, error) {
	var count int
	err := s.Pool.QueryRow(ctx, `
INSERT INTO bot_daily_chat_creates (bot_id, day, count) VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (bot_id, day) DO UPDATE SET count = bot_daily_chat_creates.count + 1
RETURNING count`, botID).Scan(&count)
	return count, err
}

// MarkEventDeferred sets delivery_status to deferred for an interaction token.
func (s *BotStore) MarkEventDeferred(ctx context.Context, botID uuid.UUID, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'deferred'
WHERE bot_id = $1 AND interaction_token = $2 AND delivery_status = 'pending'`, botID, token)
	return err
}

// ListDeferredTokens returns interaction tokens awaiting async completion.
func (s *BotStore) ListDeferredTokens(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT interaction_token FROM bot_event_log
WHERE delivery_status = 'deferred' AND interaction_token IS NOT NULL AND interaction_token <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, rows.Err()
}

const PrivilegedScopeReadHistory = "TEXT_CHAT_READ_HISTORY"

// HasPrivilegedScope reports whether scopes JSON includes a privileged install scope.
func HasPrivilegedScope(scopesJSON string) bool {
	return ScopeAllows(scopesJSON, PrivilegedScopeReadHistory)
}
