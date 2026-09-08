package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// VoiceRoomAccessRow is the exact, Space-owned evidence Voice needs before it
// evaluates Role permissions. Active currently means that the voice_rooms row
// exists; the documented data model has no lifecycle enum.
type VoiceRoomAccessRow struct {
	SpaceID uuid.UUID
	Member  bool
	Active  bool
}

type voiceRoomAccessQuerier interface {
	ResolveVoiceRoomAccessQuery(context.Context, uuid.UUID, uuid.UUID) (*VoiceRoomAccessRow, error)
}

// ResolveVoiceRoomAccess resolves the room primary key and membership EXISTS
// in one query. It deliberately has no ListSpaceMembersPage fallback.
func (s *SpaceStore) ResolveVoiceRoomAccess(ctx context.Context, voiceRoomID, profileID uuid.UUID) (*VoiceRoomAccessRow, error) {
	if s == nil {
		return nil, errors.New("space store: not configured")
	}
	if s.voiceRoomAccessQuery != nil {
		return s.voiceRoomAccessQuery.ResolveVoiceRoomAccessQuery(ctx, voiceRoomID, profileID)
	}
	if s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	var out VoiceRoomAccessRow
	err := s.Pool.QueryRow(ctx, `
SELECT vr.space_id,
       EXISTS (SELECT 1 FROM space_members sm WHERE sm.space_id = vr.space_id AND sm.profile_id = $2)
FROM voice_rooms vr
JOIN spaces sp ON sp.id = vr.space_id
WHERE vr.id = $1
`, voiceRoomID, profileID).Scan(&out.SpaceID, &out.Member)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVoiceRoomNotFound
	}
	if err != nil {
		return nil, err
	}
	out.Active = true
	return &out, nil
}
