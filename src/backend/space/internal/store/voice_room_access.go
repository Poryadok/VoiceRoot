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
	SpaceID      uuid.UUID
	Member       bool
	Active       bool
	Discoverable bool
	AccessEpoch  uint64
}

type voiceRoomAccessQuerier interface {
	ResolveVoiceRoomAccessQuery(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*VoiceRoomAccessRow, error)
}

// ResolveVoiceRoomAccess serializes an exact path-bound room/membership decision
// with the Space-owned epoch. Undiscoverable and mismatched tuples all collapse
// to ErrVoiceRoomNotFound without returning canonical identity or epoch.
func (s *SpaceStore) ResolveVoiceRoomAccess(ctx context.Context, expectedSpaceID, voiceRoomID, profileID uuid.UUID) (*VoiceRoomAccessRow, error) {
	if s == nil {
		return nil, errors.New("space store: not configured")
	}
	if s.voiceRoomAccessQuery != nil {
		return s.voiceRoomAccessQuery.ResolveVoiceRoomAccessQuery(ctx, expectedSpaceID, voiceRoomID, profileID)
	}
	if s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{expectedSpaceID}, func(scoped *SpaceStore) (*VoiceRoomAccessRow, error) {
			return scoped.ResolveVoiceRoomAccess(ctx, expectedSpaceID, voiceRoomID, profileID)
		})
	}

	tx, err := s.db().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, expectedSpaceID.String()); err != nil {
		return nil, err
	}

	var out VoiceRoomAccessRow
	var epoch int64
	err = tx.QueryRow(ctx, `
SELECT vr.space_id,
       EXISTS (
           SELECT 1 FROM space_members sm
           WHERE sm.space_id = vr.space_id AND sm.profile_id = $3
       ),
       vae.access_epoch
FROM voice_rooms vr
JOIN spaces sp ON sp.id = vr.space_id
JOIN space_voice_access_epochs vae ON vae.space_id = vr.space_id
WHERE vr.id = $2 AND vr.space_id = $1
`, expectedSpaceID, voiceRoomID, profileID).Scan(&out.SpaceID, &out.Member, &epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVoiceRoomNotFound
	}
	if err != nil {
		return nil, err
	}
	if !out.Member {
		return nil, ErrVoiceRoomNotFound
	}
	if epoch <= 0 {
		return nil, errors.New("space store: invalid Voice access epoch")
	}
	out.Active = true
	out.Discoverable = true
	out.AccessEpoch = uint64(epoch)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}
