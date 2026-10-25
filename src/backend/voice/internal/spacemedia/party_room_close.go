package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"math"
	"voice/backend/voice/internal/rosteroutbox"
)

// BeginPartyRoomClose participates in the caller's native transaction; it never
// commits it. The caller must already hold any required outer Space barrier and
// acquire all room heads before any identity locks. A P3 capture invokes this
// before making its head non-OPEN. Pending-admission abort/physical cleanup is
// still owned by that caller. Repeated CLOSING/ENDED heads emit no new facts.
func BeginPartyRoomClose(ctx context.Context, tx pgx.Tx, voiceRoomID string, generation uint64) error {
	if tx == nil || voiceRoomID == "" || generation == 0 || generation > math.MaxInt64 {
		return ErrAdmissionConflict
	}
	var state string
	var roomID uuid.UUID
	var version int64
	err := tx.QueryRow(ctx, `SELECT state,call_room_id,roster_version FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoomID, int64(generation)).Scan(&state, &roomID, &version)
	if err != nil {
		return err
	}
	if state == "CLOSING" || state == "ENDED" {
		return nil
	}
	if state != "OPEN" {
		return ErrRoomHeadPending
	}
	rows, err := tx.Query(ctx, `SELECT profile_id FROM voice_space_media_admissions WHERE voice_room_id=$1 AND room_generation=$2 AND state='COMMITTED' AND participant_state='ACTIVE' AND projection_applied_at IS NOT NULL ORDER BY profile_id`, voiceRoomID, int64(generation))
	if err != nil {
		return err
	}
	profiles := []uuid.UUID{}
	for rows.Next() {
		var profile uuid.UUID
		if err = rows.Scan(&profile); err != nil {
			rows.Close()
			return err
		}
		profiles = append(profiles, profile)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if version == math.MaxInt64 {
			return ErrAdmissionConflict
		}
		version++
		if err = rosteroutbox.Append(ctx, tx, roomID, profile, version, false); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE voice_space_media_room_heads SET state='CLOSING',roster_version=$3,updated_at=clock_timestamp() WHERE voice_room_id=$1 AND room_generation=$2`, voiceRoomID, int64(generation), version); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE voice_space_media_admissions SET participant_state='REVOKING',updated_at=clock_timestamp() WHERE voice_room_id=$1 AND room_generation=$2 AND participant_state='ACTIVE'`, voiceRoomID, int64(generation))
	return err
}
