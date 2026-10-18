package spacelifecycle

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	commonv1 "voice.app/voice/common/v1"
)

// capture seals exact immutable references before a restrictive reservation can
// commit. Space barrier excludes native forward writers. SQL-only, no media IO.
func capture(ctx context.Context, tx pgx.Tx, req *commonv1.SpaceLifecycleFenceRequest, hash []byte) error {
	args := []any{req.SpaceId, int64(req.Generation), req.DeletionOperationId, hash}
	statements := []string{
		`INSERT INTO voice_space_lifecycle_targets(space_id,generation,target_id,source_kind,deletion_operation_id,request_sha256,target)
SELECT $1,$2,'native:'||a.operation_id,'native',$3,$4,
 jsonb_build_object('operation_id',a.operation_id,'generation',a.generation,'space_id',a.space_id,
 'account_id',a.account_id,'profile_id',a.profile_id,'room_id',a.room_id,'voice_room_id',a.voice_room_id,
 'room_generation',a.room_generation,'identity',a.livekit_identity,'sfu_room','voice-room-'||a.voice_room_id,
 'state',a.state,'participant_state',a.participant_state,
 'provider_bindings',COALESCE((SELECT jsonb_agg(DISTINCT g.claims->>'ProviderBinding') FROM voice_space_media_grants g WHERE g.operation_id=a.operation_id),'["UNKNOWN"]'::jsonb))
FROM voice_space_media_admissions a WHERE a.space_id=$1
AND (a.participant_state<>'ENDED' OR a.cleanup_completed_at IS NULL)
ON CONFLICT DO NOTHING`,
		`INSERT INTO voice_space_lifecycle_targets(space_id,generation,target_id,source_kind,deletion_operation_id,request_sha256,target)
SELECT $1,$2,'head:'||h.voice_room_id||':'||h.room_generation,'head',$3,$4,to_jsonb(h)
FROM voice_space_media_room_heads h WHERE h.space_id=$1 AND h.state<>'ENDED' ON CONFLICT DO NOTHING`,
		`INSERT INTO voice_space_lifecycle_targets(space_id,generation,target_id,source_kind,deletion_operation_id,request_sha256,target)
SELECT $1,$2,'legacy:'||r.room_id||':'||m.profile_id||':'||m.media_epoch,'legacy',$3,$4,
 jsonb_build_object('room_id',r.room_id,'profile_id',m.profile_id,'media_epoch',m.media_epoch,
 'sfu_room',r.livekit_room_name,'identity','profile:'||m.profile_id||':media:'||m.media_epoch)
FROM voice_room_instances r JOIN voice_room_memberships m ON m.room_id=r.room_id WHERE r.space_id=$1
ON CONFLICT DO NOTHING`,
		// Persisted targets precede native state mutation. Existing account fences
		// remain reserved until exact outside-lock cleanup succeeds.
		`UPDATE voice_space_media_room_heads SET state='CLOSING',updated_at=clock_timestamp() WHERE space_id=$1 AND state='OPEN'`,
		`UPDATE voice_space_media_admissions SET state='ABORTING',updated_at=clock_timestamp()
WHERE space_id=$1 AND (state IN ('PREPARED','FENCED') OR (state='COMMITTED' AND projection_applied_at IS NULL))`,
		`UPDATE voice_space_media_admissions SET participant_state='REVOKING',updated_at=clock_timestamp()
WHERE space_id=$1 AND state='COMMITTED' AND projection_applied_at IS NOT NULL AND participant_state='ACTIVE'`,
		`UPDATE voice_space_lifecycle_fence_receipts SET coverage_sealed=TRUE WHERE space_id=$1 AND generation=$2`,
	}
	for _, sql := range statements {
		// pgx rejects surplus parameters for updates whose statement has fewer.
		n := 4
		if sql[0:6] == "UPDATE" {
			n = 1
			if sql == statements[len(statements)-1] {
				n = 2
			}
		}
		if _, err := tx.Exec(ctx, sql, args[:n]...); err != nil {
			return err
		}
	}
	return nil
}

// CapturedTarget is operation-bound cleanup data, never reconstructed from a
// later Space roster. Credentials and provider secrets must never enter it.
type CapturedTarget struct {
	OperationID      string `json:"operation_id"`
	Generation       string `json:"generation"`
	SpaceID          string `json:"space_id"`
	AccountID        string `json:"account_id"`
	ProfileID        string `json:"profile_id"`
	RoomID           string `json:"room_id"`
	VoiceRoomID      string `json:"voice_room_id"`
	RoomGeneration   uint64 `json:"room_generation"`
	Identity         string `json:"identity"`
	SFURoom          string `json:"sfu_room"`
	MediaEpoch       string `json:"media_epoch"`
	State            string `json:"state"`
	ParticipantState string `json:"participant_state"`
}

func decodeTarget(raw []byte) (CapturedTarget, error) {
	var t CapturedTarget
	err := json.Unmarshal(raw, &t)
	return t, err
}
