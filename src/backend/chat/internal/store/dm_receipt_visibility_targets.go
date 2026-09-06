package store

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"
)

// DMReceiptVisibilityTarget is the peer side of one durable direct DM.
// Archive and request inbox state are deliberately not filters: privacy
// revocation must cover every DM in which a peer could see a read receipt.
type DMReceiptVisibilityTarget struct {
	ChatID        uuid.UUID
	PeerProfileID uuid.UUID
}

var ErrInvalidDMReceiptVisibilityCursor = errors.New("invalid dm receipt visibility cursor")

func encodeDMReceiptVisibilityCursor(chatID uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(chatID.String()))
}

func decodeDMReceiptVisibilityCursor(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return uuid.Nil, ErrInvalidDMReceiptVisibilityCursor
	}
	id, err := uuid.Parse(string(b))
	if err != nil {
		return uuid.Nil, ErrInvalidDMReceiptVisibilityCursor
	}
	return id, nil
}

// ListDMReceiptVisibilityTargets returns each two-member direct DM containing
// profileID. It is stable-paginated by chat UUID and excludes non-DMs and
// malformed legacy rows with any member count other than exactly two.
func (s *DMStore) ListDMReceiptVisibilityTargets(ctx context.Context, profileID uuid.UUID, cursor string, limit int) ([]DMReceiptVisibilityTarget, string, error) {
	if s == nil || s.Pool == nil {
		return nil, "", errors.New("dm store: pool not configured")
	}
	if profileID == uuid.Nil {
		return nil, "", errors.New("profile id is required")
	}
	if limit < 1 {
		limit = 1
	}
	after, err := decodeDMReceiptVisibilityCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	rows, err := s.Pool.Query(ctx, `
SELECT c.id, peer.profile_id
FROM chats c
INNER JOIN chat_members self ON self.chat_id = c.id AND self.profile_id = $1
INNER JOIN chat_members peer ON peer.chat_id = c.id AND peer.profile_id <> $1
WHERE c.type = 'dm'
  AND ($2::uuid IS NULL OR c.id > $2)
  AND (SELECT COUNT(*) FROM chat_members members WHERE members.chat_id = c.id) = 2
ORDER BY c.id ASC
LIMIT $3
`, profileID, nullableUUID(after), limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([]DMReceiptVisibilityTarget, 0, limit)
	for rows.Next() {
		var target DMReceiptVisibilityTarget
		if err := rows.Scan(&target.ChatID, &target.PeerProfileID); err != nil {
			return nil, "", err
		}
		out = append(out, target)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > limit {
		next = encodeDMReceiptVisibilityCursor(out[limit-1].ChatID)
		out = out[:limit]
	}
	return out, next, nil
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
