package spacelifecycle

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/livekit"
)

type MediaLifecycle interface {
	RemoveParticipant(context.Context, string, string) error
	CloseRoom(context.Context, string) error
}

type Effects struct {
	pool        *pgxpool.Pool
	media       MediaLifecycle
	NativeDrain interface {
		ResumeCapturedDrain(context.Context, CapturedTarget) error
	}
	ProjectionFence interface {
		SetSpaceLifecycle(context.Context, string, int64, bool) error
	}
}

func NewEffects(pool *pgxpool.Pool, media MediaLifecycle) *Effects {
	return &Effects{pool: pool, media: media}
}

// ResumeFrozen consumes only targets sealed in the restrictive transaction.
// There is deliberately no provider retirement success inferred here.
func (e *Effects) ResumeFrozen(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest) error {
	if e == nil || e.pool == nil || e.media == nil || e.ProjectionFence == nil {
		return ErrUnavailable
	}
	if err := e.ProjectionFence.SetSpaceLifecycle(ctx, req.SpaceId, int64(req.Generation), false); err != nil {
		return err
	}
	rows, err := e.pool.Query(ctx, `SELECT target_id,source_kind,target FROM voice_space_lifecycle_targets WHERE space_id=$1 AND generation=$2 ORDER BY target_id`, req.SpaceId, int64(req.Generation))
	if err != nil {
		return err
	}
	type work struct {
		id, kind string
		target   CapturedTarget
	}
	var targets []work
	for rows.Next() {
		var id, kind string
		var raw []byte
		if err := rows.Scan(&id, &kind, &raw); err != nil {
			rows.Close()
			return err
		}
		target, err := decodeTarget(raw)
		if err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, work{id, kind, target})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, w := range targets {
		if w.kind == "head" {
			continue
		}
		if err := e.media.RemoveParticipant(ctx, w.target.SFURoom, w.target.Identity); err != nil {
			return fmt.Errorf("eject captured P3 participant: %w", err)
		}
		if w.kind == "native" {
			if e.NativeDrain == nil {
				return ErrUnavailable
			}
			if err := e.NativeDrain.ResumeCapturedDrain(ctx, w.target); err != nil {
				return err
			}
		}
		if w.kind == "legacy" {
			if _, err := e.pool.Exec(ctx, `DELETE FROM voice_room_memberships WHERE room_id=$1 AND profile_id=$2 AND media_epoch=$3`, w.target.RoomID, w.target.ProfileID, w.target.MediaEpoch); err != nil {
				return err
			}
		}
		if _, err := e.pool.Exec(ctx, `UPDATE voice_space_lifecycle_targets SET ejected_at=clock_timestamp(),cleanup_completed_at=COALESCE(cleanup_completed_at,clock_timestamp()) WHERE space_id=$1 AND generation=$2 AND target_id=$3`, req.SpaceId, int64(req.Generation), w.id); err != nil {
			return err
		}
	}
	for _, w := range targets {
		if w.kind != "head" {
			continue
		}
		if _, err := e.pool.Exec(ctx, `UPDATE voice_space_media_room_heads h SET state='ENDED',updated_at=clock_timestamp() WHERE h.voice_room_id=$1 AND h.room_generation=$2 AND h.state='CLOSING'
AND NOT EXISTS(SELECT 1 FROM voice_space_media_admissions a WHERE a.voice_room_id=h.voice_room_id AND a.room_generation=h.room_generation AND a.participant_state<>'ENDED')`, w.target.VoiceRoomID, int64(w.target.RoomGeneration)); err != nil {
			return err
		}
	}
	return nil
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
