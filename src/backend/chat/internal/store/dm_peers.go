package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// DMPeerProfileIDs returns the other member's profile_id for each DM chat in chatIDs.
func (s *DMStore) DMPeerProfileIDs(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if len(chatIDs) == 0 {
		return map[uuid.UUID]uuid.UUID{}, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT m.chat_id, m.profile_id
FROM chat_members m
INNER JOIN chats c ON c.id = m.chat_id AND c.type = 'dm'
WHERE m.chat_id = ANY($1::uuid[])
  AND m.profile_id <> $2
`, chatIDs, viewerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]uuid.UUID, len(chatIDs))
	for rows.Next() {
		var chatID, peerID uuid.UUID
		if err := rows.Scan(&chatID, &peerID); err != nil {
			return nil, err
		}
		out[chatID] = peerID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
