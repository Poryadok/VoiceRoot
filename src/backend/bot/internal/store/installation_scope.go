package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CheckCurrentInstallationScope is a current owner decision, never a historical
// receipt. A revoked/mismatched installation is false; storage failure is unknown.
func (s *BotStore) CheckCurrentInstallationScope(ctx context.Context, botID, spaceID, actorID, installationID uuid.UUID, scope string) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("Bot installation storage unavailable")
	}
	if botID == uuid.Nil || spaceID == uuid.Nil || actorID == uuid.Nil || installationID == uuid.Nil || scope != "TEXT_CHAT_CREATE_IN_SPACE" {
		return false, errors.New("invalid installation binding")
	}
	var currentActor uuid.UUID
	var botStatus, scopes, lifecycle string
	err := s.Pool.QueryRow(ctx, `
SELECT b.actor_profile_id,b.status,b.scopes::text,COALESCE(h.desired_state,'LIVE')
FROM bot_space_installations i
JOIN bots b ON b.id=i.bot_id
LEFT JOIN bot_space_lifecycle_heads h ON h.space_id=i.space_id
WHERE i.id=$1 AND i.bot_id=$2 AND i.space_id=$3`, installationID, botID, spaceID).Scan(&currentActor, &botStatus, &scopes, &lifecycle)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return currentActor == actorID && botStatus == "live" && lifecycle == "LIVE" && ScopeAllows(scopes, scope), nil
}
