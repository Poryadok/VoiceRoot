package spacelifecycle

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/voice/internal/livekit"
)

type MediaLifecycle interface {
	RemoveParticipant(context.Context, string, string) error
	CloseRoom(context.Context, string) error
}

type Effects struct {
	pool  *pgxpool.Pool
	media MediaLifecycle
}

func NewEffects(pool *pgxpool.Pool, media MediaLifecycle) *Effects {
	return &Effects{pool: pool, media: media}
}

// FreezeSpace ejects every persisted participant before deleting the grant rows.
// Missing LiveKit participants are treated as successful retries by RoomLifecycle.
func (e *Effects) FreezeSpace(ctx context.Context, spaceID string) error {
	if e == nil || e.pool == nil || e.media == nil || !canonicalUUID(spaceID) {
		return ErrUnavailable
	}
	rows, err := e.pool.Query(ctx, `SELECT r.livekit_room_name,m.profile_id,m.media_epoch
FROM voice_room_instances r JOIN voice_room_memberships m ON m.room_id=r.room_id
WHERE r.space_id=$1 AND r.state IN ('active','closing') ORDER BY r.room_id,m.profile_id`, uuid.MustParse(spaceID))
	if err != nil {
		return err
	}
	type member struct{ roomName, identity string }
	var members []member
	for rows.Next() {
		var roomName string
		var profileID, mediaEpoch uuid.UUID
		if err := rows.Scan(&roomName, &profileID, &mediaEpoch); err != nil {
			rows.Close()
			return err
		}
		members = append(members, member{roomName: roomName, identity: "profile:" + profileID.String() + ":media:" + mediaEpoch.String()})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, participant := range members {
		if err := e.media.RemoveParticipant(ctx, participant.roomName, participant.identity); err != nil {
			return fmt.Errorf("eject Space voice participant: %w", err)
		}
	}
	_, err = e.pool.Exec(ctx, `DELETE FROM voice_room_memberships m USING voice_room_instances r
WHERE m.room_id=r.room_id AND r.space_id=$1`, uuid.MustParse(spaceID))
	return err
}

// PurgeSpace closes all associated SFU rooms and terminally closes Voice room
// records while deleting participant grant rows. The operation is retry safe.
func (e *Effects) PurgeSpace(ctx context.Context, spaceID string) error {
	if e == nil || e.pool == nil || e.media == nil || !canonicalUUID(spaceID) {
		return ErrUnavailable
	}
	rows, err := e.pool.Query(ctx, `SELECT livekit_room_name FROM voice_room_instances WHERE space_id=$1 AND state <> 'closed' ORDER BY room_id`, uuid.MustParse(spaceID))
	if err != nil {
		return err
	}
	var roomNames []string
	for rows.Next() {
		var roomName string
		if err := rows.Scan(&roomName); err != nil {
			rows.Close()
			return err
		}
		roomNames = append(roomNames, roomName)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, roomName := range roomNames {
		if err := e.media.CloseRoom(ctx, roomName); err != nil {
			return fmt.Errorf("close Space LiveKit room: %w", err)
		}
	}
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, spaceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM voice_room_memberships m USING voice_room_instances r
WHERE m.room_id=r.room_id AND r.space_id=$1`, uuid.MustParse(spaceID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_room_instances SET state='closed',closed_at=COALESCE(closed_at,clock_timestamp()),updated_at=clock_timestamp()
WHERE space_id=$1 AND state <> 'closed'`, uuid.MustParse(spaceID)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

var _ MediaLifecycle = (*livekit.RoomLifecycle)(nil)
