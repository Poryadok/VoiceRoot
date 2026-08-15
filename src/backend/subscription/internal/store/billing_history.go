package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BillingHistoryItem is a billing event visible to an account.
type BillingHistoryItem struct {
	ID          uuid.UUID
	Type        string
	Provider    string
	Details     json.RawMessage
	OccurredAt  time.Time
}

// ListBillingHistory returns billing events for an account ordered newest first.
func (s *SubscriptionStore) ListBillingHistory(ctx context.Context, accountID uuid.UUID, limit int, afterOccurredAt *time.Time) ([]BillingHistoryItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	args := []any{accountID, limit}
	cursorClause := ""
	if afterOccurredAt != nil {
		cursorClause = " AND be.created_at < $3"
		args = append(args, afterOccurredAt.UTC())
	}

	query := `
SELECT be.id, be.type, be.provider, be.details, be.created_at
FROM billing_events be
LEFT JOIN subscriptions s ON s.id = be.subscription_id
LEFT JOIN space_subscriptions ss ON ss.id = be.space_subscription_id
WHERE (s.account_id = $1 OR ss.purchaser_account_id = $1)` + cursorClause + `
ORDER BY be.created_at DESC
LIMIT $2`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BillingHistoryItem
	for rows.Next() {
		var item BillingHistoryItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Provider, &item.Details, &item.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
