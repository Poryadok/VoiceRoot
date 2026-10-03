package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	notificationv1 "voice.app/voice/notification/v1"
)

var ErrSpaceLifecycleNotLive = errors.New("notification Space lifecycle is not live")

type notificationLifecycleFence struct {
	OperationID uuid.UUID
	Generation  uint64
	State       commonv1.LifecycleFenceState
	ManifestID  string
	ManifestSHA []byte
	ItemCount   uint64
}

func notificationLifecycleUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, errors.New("invalid canonical lifecycle UUID")
	}
	return id, nil
}

func notificationLifecycleWrapperBytes(message proto.Message) ([]byte, []byte, error) {
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	if err != nil {
		return nil, nil, err
	}
	input := append(append([]byte(message.ProtoReflect().Descriptor().FullName()), 0), encoded...)
	hash := sha256.Sum256(input)
	return encoded, hash[:], nil
}

func notificationFenceWrapper(request *commonv1.SpaceLifecycleFenceRequest) *notificationv1.ApplySpaceLifecycleFenceRequest {
	return &notificationv1.ApplySpaceLifecycleFenceRequest{Fence: proto.Clone(request).(*commonv1.SpaceLifecycleFenceRequest)}
}

func notificationPurgeWrapper(request *commonv1.SpacePurgeRequest) *notificationv1.PurgeSpaceRequest {
	return &notificationv1.PurgeSpaceRequest{Purge: proto.Clone(request).(*commonv1.SpacePurgeRequest)}
}

func notificationManifestMatches(fence notificationLifecycleFence, manifest *commonv1.ManifestBinding) bool {
	return manifest != nil && fence.ManifestID == manifest.GetManifestId() && fence.ItemCount == manifest.GetItemCount() && bytes.Equal(fence.ManifestSHA, manifest.GetManifestSha256())
}

func loadNotificationLifecycleFence(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) (notificationLifecycleFence, bool, error) {
	var result notificationLifecycleFence
	var state int32
	var rawState string
	if err := tx.QueryRow(ctx, `SELECT deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count FROM notification_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(
		&result.OperationID, &result.Generation, &rawState, &result.ManifestID, &result.ManifestSHA, &result.ItemCount,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notificationLifecycleFence{}, false, nil
		}
		return notificationLifecycleFence{}, false, err
	}
	switch rawState {
	case "FROZEN":
		state = int32(commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	case "LIVE":
		state = int32(commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE)
	case "PURGE_DECIDED":
		state = int32(commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	case "PURGED":
		result.State = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
		return result, true, nil
	default:
		return notificationLifecycleFence{}, false, fmt.Errorf("unknown notification lifecycle state %q", rawState)
	}
	result.State = commonv1.LifecycleFenceState(state)
	return result, true, nil
}

func notificationLifecycleLock(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, spaceID.String())
	return err
}

func (s *SettingsStore) ApplySpaceLifecycleFence(ctx context.Context, request *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if s == nil || s.Pool == nil || request == nil || request.GetManifest() == nil {
		return nil, ErrNotImplemented
	}
	spaceID, err := notificationLifecycleUUID(request.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operationID, err := notificationLifecycleUUID(request.GetDeletionOperationId())
	if err != nil || request.GetProtocolVersion() != 1 || request.GetGeneration() == 0 {
		return nil, errors.New("invalid notification lifecycle fence")
	}
	manifest := request.GetManifest()
	if manifest.GetManifestId() == "" || len(manifest.GetManifestSha256()) != sha256.Size {
		return nil, errors.New("invalid notification lifecycle manifest")
	}
	if request.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN && request.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE && request.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED {
		return nil, errors.New("invalid notification lifecycle state")
	}
	wrapper := notificationFenceWrapper(request)
	requestBytes, requestHash, err := notificationLifecycleWrapperBytes(wrapper)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := notificationLifecycleLock(ctx, tx, spaceID); err != nil {
		return nil, err
	}
	var savedRequest, savedReceipt []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM notification_space_lifecycle_fence_receipts WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND desired_state=$4`, spaceID, operationID, request.GetGeneration(), request.GetDesiredState()).Scan(&savedRequest, &savedReceipt)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) {
			return nil, errors.New("notification lifecycle fence request conflict")
		}
		response := &notificationv1.ApplySpaceLifecycleFenceResponse{}
		if err := proto.Unmarshal(savedReceipt, response); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response.GetReceipt(), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	current, exists, err := loadNotificationLifecycleFence(ctx, tx, spaceID)
	if err != nil {
		return nil, err
	}
	var sealedManifestBytes []byte
	var scheduleGeneration uint64
	var sealed bool
	err = tx.QueryRow(ctx, `SELECT manifest_bytes,schedule_generation,sealed FROM notification_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2 FOR SHARE`, spaceID, operationID).Scan(&sealedManifestBytes, &scheduleGeneration, &sealed)
	if err != nil {
		return nil, ErrSpaceLifecycleNotLive
	}
	savedManifest := &commonv1.ManifestBinding{}
	if proto.Unmarshal(sealedManifestBytes, savedManifest) != nil || !sealed || !proto.Equal(savedManifest, manifest) {
		return nil, ErrSpaceLifecycleNotLive
	}
	if request.GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN && request.GetGeneration() != scheduleGeneration {
		return nil, ErrSpaceLifecycleNotLive
	}
	if exists {
		if current.State == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED {
			return nil, ErrSpaceLifecycleNotLive
		}
		if current.State == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE && request.GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
			if request.GetGeneration() <= current.Generation {
				return nil, ErrSpaceLifecycleNotLive
			}
		} else if current.OperationID != operationID || !notificationManifestMatches(current, manifest) {
			return nil, ErrSpaceLifecycleNotLive
		}
		switch request.GetDesiredState() {
		case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
			if current.State != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN || request.GetGeneration() != current.Generation+1 {
				return nil, ErrSpaceLifecycleNotLive
			}
		case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
			if current.State != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN || request.GetGeneration() != current.Generation+1 {
				return nil, ErrSpaceLifecycleNotLive
			}
		case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
			if current.State != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE || request.GetGeneration() <= current.Generation {
				return nil, ErrSpaceLifecycleNotLive
			}
		}
	} else if request.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
		return nil, ErrSpaceLifecycleNotLive
	}
	var appliedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&appliedAt); err != nil {
		return nil, err
	}
	receipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: request.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION, AppliedState: request.GetDesiredState(), RequestSha256: requestHash, ManifestSha256: append([]byte(nil), manifest.GetManifestSha256()...), AppliedAt: timestamppb.New(appliedAt)}
	response := &notificationv1.ApplySpaceLifecycleFenceResponse{Receipt: receipt}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, err
	}
	stateName := map[commonv1.LifecycleFenceState]string{
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:        "FROZEN",
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:          "LIVE",
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED: "PURGE_DECIDED",
	}[request.GetDesiredState()]
	_, err = tx.Exec(ctx, `INSERT INTO notification_space_lifecycle_fence_receipts(space_id,deletion_operation_id,generation,desired_state,request_bytes,receipt_bytes,applied_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, spaceID, operationID, request.GetGeneration(), request.GetDesiredState(), requestBytes, receiptBytes, appliedAt)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO notification_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(space_id) DO UPDATE SET deletion_operation_id=EXCLUDED.deletion_operation_id,generation=EXCLUDED.generation,state=EXCLUDED.state,manifest_id=EXCLUDED.manifest_id,manifest_sha256=EXCLUDED.manifest_sha256,manifest_item_count=EXCLUDED.manifest_item_count,updated_at=EXCLUDED.updated_at`, spaceID, operationID, request.GetGeneration(), stateName, manifest.GetManifestId(), manifest.GetManifestSha256(), manifest.GetItemCount(), appliedAt)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE notification_space_chat_fences SET blocked=$2 WHERE space_id=$1`, spaceID, stateName != "LIVE"); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *SettingsStore) PurgeSpace(ctx context.Context, request *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	if s == nil || s.Pool == nil || request == nil || request.GetManifest() == nil {
		return nil, ErrNotImplemented
	}
	spaceID, err := notificationLifecycleUUID(request.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operationID, err := notificationLifecycleUUID(request.GetDeletionOperationId())
	if err != nil || request.GetProtocolVersion() != 1 || request.GetGeneration() == 0 || request.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION || request.GetPurgeDecidedAt() == nil || request.GetPurgeDecidedAt().CheckValid() != nil {
		return nil, errors.New("invalid notification Space purge")
	}
	wrapper := notificationPurgeWrapper(request)
	requestBytes, requestHash, err := notificationLifecycleWrapperBytes(wrapper)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := notificationLifecycleLock(ctx, tx, spaceID); err != nil {
		return nil, err
	}
	var savedRequest, savedReceipt []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM notification_space_lifecycle_purge_receipts WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3`, spaceID, operationID, request.GetGeneration()).Scan(&savedRequest, &savedReceipt)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) {
			return nil, errors.New("notification Space purge request conflict")
		}
		response := &notificationv1.PurgeSpaceResponse{}
		if err := proto.Unmarshal(savedReceipt, response); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response.GetReceipt(), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	current, exists, err := loadNotificationLifecycleFence(ctx, tx, spaceID)
	if err != nil {
		return nil, err
	}
	if !exists || current.State != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED || current.OperationID != operationID || current.Generation != request.GetGeneration() || !notificationManifestMatches(current, request.GetManifest()) {
		return nil, ErrSpaceLifecycleNotLive
	}
	var completedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completedAt); err != nil {
		return nil, err
	}
	receipt := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: request.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: requestHash, CompletedAt: timestamppb.New(completedAt)}
	response := &notificationv1.PurgeSpaceResponse{Receipt: receipt}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM notification_settings WHERE (scope_type='space' AND scope_id=$1) OR (scope_type IN ('channel','chat') AND scope_id IN (SELECT chat_id FROM notification_space_chat_fences WHERE space_id=$1))`, spaceID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_space_lifecycle_fence_receipts SET retain_until=$2::timestamptz+interval '30 days' WHERE space_id=$1 AND retain_until IS NULL`, spaceID, completedAt); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE notification_space_chat_manifests SET retain_until=$2::timestamptz+interval '30 days' WHERE space_id=$1 AND retain_until IS NULL`, spaceID, completedAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_space_lifecycle_fences SET state='PURGED',deletion_operation_id='00000000-0000-0000-0000-000000000000',manifest_id='',manifest_sha256=decode(repeat('00',32),'hex'),manifest_item_count=0,updated_at=$2 WHERE space_id=$1`, spaceID, completedAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notification_space_lifecycle_purge_receipts(space_id,deletion_operation_id,generation,request_bytes,receipt_bytes,completed_at,retain_until) VALUES($1,$2,$3,$4,$5,$6,$6::timestamptz+interval '30 days')`, spaceID, operationID, request.GetGeneration(), requestBytes, receiptBytes, completedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}

// DeleteExpiredSpaceLifecycleReceipts uses PostgreSQL time as the retention
// authority. The per-Space PURGED fence remains as a minimal permanent tombstone.
func (s *SettingsStore) DeleteExpiredSpaceLifecycleReceipts(ctx context.Context) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, ErrNotImplemented
	}
	var fenceRows, purgeRows int64
	if _, err := s.Pool.Exec(ctx, `DELETE FROM notification_space_chat_manifests WHERE retain_until <= clock_timestamp()`); err != nil {
		return 0, err
	}
	if err := s.Pool.QueryRow(ctx, `WITH deleted AS (DELETE FROM notification_space_lifecycle_fence_receipts WHERE retain_until IS NOT NULL AND retain_until <= clock_timestamp() RETURNING 1) SELECT count(*) FROM deleted`).Scan(&fenceRows); err != nil {
		return 0, err
	}
	if err := s.Pool.QueryRow(ctx, `WITH deleted AS (DELETE FROM notification_space_lifecycle_purge_receipts WHERE retain_until <= clock_timestamp() RETURNING 1) SELECT count(*) FROM deleted`).Scan(&purgeRows); err != nil {
		return fenceRows, err
	}
	return fenceRows + purgeRows, nil
}

func (s *SettingsStore) upsertScopedSettings(ctx context.Context, settings NotificationSettings) error {
	tx, owned, err := s.scopeTransaction(ctx, settings.ScopeType, settings.ScopeID)
	if err != nil {
		return err
	}
	if owned {
		defer func() { _ = tx.Rollback(context.Background()) }()
	}
	suppress, err := json.Marshal(settings.SuppressTypes)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO notification_settings(profile_id,scope_type,scope_id,enabled,mute_until,suppress_types) VALUES($1,$2,$3,$4,$5,$6::jsonb)
ON CONFLICT(profile_id,scope_type,scope_id) WHERE scope_id IS NOT NULL DO UPDATE SET enabled=EXCLUDED.enabled,mute_until=EXCLUDED.mute_until,suppress_types=EXCLUDED.suppress_types`, settings.ProfileID, settings.ScopeType, settings.ScopeID, settings.Enabled, settings.MuteUntil, string(suppress))
	if err != nil {
		return err
	}
	if owned {
		return tx.Commit(ctx)
	}
	return nil
}
