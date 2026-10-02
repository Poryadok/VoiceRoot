package store

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
)

var ErrSpaceLifecycleConflict = errors.New("bot space lifecycle conflict")

func (s *BotStore) ApplySpaceLifecycleFence(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if s == nil || s.Pool == nil || req == nil || req.GetProtocolVersion() != 1 || req.GetGeneration() == 0 {
		return nil, ErrSpaceLifecycleConflict
	}
	spaceID, err := lifecycleID(req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	opID, err := lifecycleID(req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	manifest := req.GetManifest()
	if manifest == nil || len(manifest.GetManifestSha256()) != sha256.Size {
		return nil, ErrSpaceLifecycleConflict
	}
	manifestID, err := lifecycleID(manifest.GetManifestId())
	if err != nil {
		return nil, err
	}
	if req.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN && req.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE && req.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED {
		return nil, ErrSpaceLifecycleConflict
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, err
	}
	requestBytes := lifecycleWrapped(raw)
	hash := lifecycleHash("voice.bot.v1.ApplySpaceLifecycleFenceRequest", requestBytes)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 71431))`, spaceID.String()); err != nil {
		return nil, err
	}
	var oldOp, oldState string
	var oldGeneration int64
	var oldManifest uuid.UUID
	var oldManifestHash []byte
	var oldCount int64
	err = tx.QueryRow(ctx, `SELECT deletion_operation_id::text,desired_state,generation,manifest_id,manifest_sha256,manifest_item_count FROM bot_space_lifecycle_heads WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&oldOp, &oldState, &oldGeneration, &oldManifest, &oldManifestHash, &oldCount)
	if err == nil {
		if oldOp != opID.String() || oldManifest != manifestID || !equalBytes(oldManifestHash, manifest.GetManifestSha256()) || oldCount != int64(manifest.GetItemCount()) {
			return nil, ErrSpaceLifecycleConflict
		}
		var savedRequest, savedReceipt []byte
		replayErr := tx.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM bot_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=$2`, spaceID, int64(req.GetGeneration())).Scan(&savedRequest, &savedReceipt)
		if replayErr == nil {
			if !equalBytes(savedRequest, requestBytes) {
				return nil, ErrSpaceLifecycleConflict
			}
			var receipt commonv1.SpaceLifecycleFenceReceipt
			if err := proto.Unmarshal(savedReceipt, &receipt); err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return &receipt, nil
		}
		if !errors.Is(replayErr, pgx.ErrNoRows) || int64(req.GetGeneration()) != oldGeneration+1 || !allowedBotLifecycleTransition(oldState, req.GetDesiredState()) {
			return nil, ErrSpaceLifecycleConflict
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else if req.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
		return nil, ErrSpaceLifecycleConflict
	}
	if req.GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE && oldState != "FROZEN" {
		return nil, ErrSpaceLifecycleConflict
	}
	now := timestamppb.Now()
	receipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: opID.String(), Generation: req.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, AppliedState: req.GetDesiredState(), RequestSha256: hash[:], ManifestSha256: append([]byte(nil), manifest.GetManifestSha256()...), AppliedAt: now}
	receiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	state := req.GetDesiredState().String()[len("LIFECYCLE_FENCE_STATE_"):]
	_, err = tx.Exec(ctx, `INSERT INTO bot_space_lifecycle_heads(space_id,deletion_operation_id,generation,desired_state,manifest_id,manifest_sha256,manifest_item_count) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(space_id) DO UPDATE SET deletion_operation_id=EXCLUDED.deletion_operation_id,generation=EXCLUDED.generation,desired_state=EXCLUDED.desired_state,updated_at=clock_timestamp()`, spaceID, opID, int64(req.GetGeneration()), state, manifestID, manifest.GetManifestSha256(), int64(manifest.GetItemCount()))
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_space_lifecycle_fence_receipts(space_id,deletion_operation_id,generation,request_sha256,request_bytes,receipt_bytes) VALUES($1,$2,$3,$4,$5,$6)`, spaceID, opID, int64(req.GetGeneration()), hash[:], requestBytes, receiptBytes)
	if err != nil {
		return nil, err
	}
	if req.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE {
		_, err = tx.Exec(ctx, `UPDATE bot_message_deliveries d SET status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE d.status IN ('pending','claimed') AND EXISTS(SELECT 1 FROM bot_chat_whitelist w WHERE w.bot_id=d.bot_id AND w.chat_id=d.chat_id AND w.space_id=$1)`, spaceID)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE bot_event_log e SET delivery_status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE e.delivery_status IN ('pending','deferred') AND e.space_id=$1`, spaceID)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *BotStore) PurgeSpace(ctx context.Context, req *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	if s == nil || s.Pool == nil || req == nil || req.GetProtocolVersion() != 1 || req.GetGeneration() == 0 || req.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_BOT || req.GetPurgeDecidedAt() == nil || req.GetPurgeDecidedAt().CheckValid() != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	spaceID, err := lifecycleID(req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	opID, err := lifecycleID(req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	if req.GetManifest() == nil {
		return nil, ErrSpaceLifecycleConflict
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, err
	}
	requestBytes := lifecycleWrapped(raw)
	hash := lifecycleHash("voice.bot.v1.PurgeSpaceRequest", requestBytes)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,71431))`, spaceID.String()); err != nil {
		return nil, err
	}
	var op string
	var generation int64
	var state string
	var savedManifest uuid.UUID
	var savedHash []byte
	var count int64
	if err = tx.QueryRow(ctx, `SELECT deletion_operation_id::text,generation,desired_state,manifest_id,manifest_sha256,manifest_item_count FROM bot_space_lifecycle_heads WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&op, &generation, &state, &savedManifest, &savedHash, &count); err != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	manifestID, err := lifecycleID(req.GetManifest().GetManifestId())
	if err != nil {
		return nil, err
	}
	if op != opID.String() || generation != int64(req.GetGeneration()) || manifestID != savedManifest || !equalBytes(savedHash, req.GetManifest().GetManifestSha256()) || count != int64(req.GetManifest().GetItemCount()) {
		return nil, ErrSpaceLifecycleConflict
	}
	var priorHash, priorRequest, priorReceipt []byte
	err = tx.QueryRow(ctx, `SELECT request_sha256,request_bytes,receipt_bytes FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1 AND generation=$2`, spaceID, generation).Scan(&priorHash, &priorRequest, &priorReceipt)
	if err == nil {
		if !equalBytes(priorRequest, requestBytes) || !equalBytes(priorHash, hash[:]) {
			return nil, ErrSpaceLifecycleConflict
		}
		var receipt commonv1.SpacePurgeReceipt
		if err := proto.Unmarshal(priorReceipt, &receipt); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if state != "PURGE_DECIDED" {
		return nil, ErrSpaceLifecycleConflict
	}
	_, err = tx.Exec(ctx, `UPDATE bot_message_deliveries d SET status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE EXISTS(SELECT 1 FROM bot_chat_whitelist w WHERE w.bot_id=d.bot_id AND w.chat_id=d.chat_id AND w.space_id=$1)`, spaceID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE bot_event_log SET delivery_status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE space_id=$1`, spaceID)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM bot_chat_whitelist WHERE space_id=$1`, spaceID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM bot_space_installations WHERE space_id=$1`, spaceID); err != nil {
		return nil, err
	}
	now := timestamppb.Now()
	receipt := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: opID.String(), Generation: req.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: hash[:], CompletedAt: now}
	receiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_space_lifecycle_heads SET desired_state='PURGED',updated_at=clock_timestamp() WHERE space_id=$1`, spaceID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO bot_space_lifecycle_purge_receipts(space_id,deletion_operation_id,generation,request_sha256,request_bytes,receipt_bytes) VALUES($1,$2,$3,$4,$5,$6)`, spaceID, opID, generation, hash[:], requestBytes, receiptBytes); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}

func lifecycleID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, ErrSpaceLifecycleConflict
	}
	return id, nil
}
func lifecycleWrapped(inner []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), inner)
}
func lifecycleHash(fqn string, raw []byte) [32]byte {
	return sha256.Sum256(append(append([]byte(fqn), 0), raw...))
}
func equalBytes(a, b []byte) bool { return string(a) == string(b) }
func allowedBotLifecycleTransition(old string, next commonv1.LifecycleFenceState) bool {
	switch old {
	case "FROZEN":
		return next == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE || next == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	case "LIVE":
		return false
	case "PURGE_DECIDED":
		return next == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	default:
		return false
	}
}
