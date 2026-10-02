// Package spacelifecycle implements Matchmaking's durable Space participant.
package spacelifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

var (
	ErrInvalidRequest = errors.New("invalid Matchmaking Space lifecycle request")
	ErrConflict       = errors.New("matchmaking Space lifecycle request conflicts with durable state")
	ErrUnavailable    = errors.New("matchmaking Space lifecycle store unavailable")
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type FenceEffect func(context.Context, uuid.UUID) error
type PurgeEffect func(context.Context, uuid.UUID) error

func (s *Store) ApplyFence(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest, effect FenceEffect) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if s == nil || s.pool == nil {
		return nil, ErrUnavailable
	}
	if err := validateFence(req); err != nil {
		return nil, err
	}
	if fenceNeedsQueueCleanup(req.GetDesiredState()) && effect == nil {
		return nil, ErrUnavailable
	}
	requestBytes, requestHash, err := canonical(&matchmakingv1.ApplySpaceLifecycleFenceRequest{Fence: req}, "voice.matchmaking.v1.ApplySpaceLifecycleFenceRequest\x00")
	if err != nil {
		return nil, err
	}
	spaceID, _ := uuid.Parse(req.GetSpaceId())
	operationID, _ := uuid.Parse(req.GetDeletionOperationId())
	state := fenceState(req.GetDesiredState())
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, req.GetSpaceId()); err != nil {
		return nil, err
	}

	var savedRequest, savedHash, savedReceipt, savedReceiptHash []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_bytes,receipt_sha256
FROM matchmaking_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=$2`, spaceID, int64(req.GetGeneration())).Scan(&savedRequest, &savedHash, &savedReceipt, &savedReceiptHash)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) {
			return nil, ErrConflict
		}
		if !bytes.Equal(savedHash, requestHash) || !validReceipt(savedReceipt, savedReceiptHash, "voice.space-lifecycle.receipt.v1\x00") {
			return nil, ErrUnavailable
		}
		var receipt commonv1.SpaceLifecycleFenceReceipt
		if err := proto.Unmarshal(savedReceipt, &receipt); err != nil {
			return nil, ErrUnavailable
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		if fenceNeedsQueueCleanup(req.GetDesiredState()) && effect != nil {
			if err := effect(ctx, spaceID); err != nil {
				return nil, err
			}
		}
		return &receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var currentGeneration int64
	var currentState string
	var currentOperation uuid.UUID
	var currentManifestID string
	var currentManifestHash []byte
	var currentItemCount int64
	err = tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id,manifest_id,manifest_sha256,manifest_item_count
FROM matchmaking_space_lifecycle_fence_heads WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&currentGeneration, &currentState, &currentOperation, &currentManifestID, &currentManifestHash, &currentItemCount)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	stale := err == nil && req.GetGeneration() < uint64(currentGeneration)
	appliedState := state
	if stale {
		appliedState = currentState
	} else {
		if err := validateTransition(currentGeneration, currentState, currentOperation, currentManifestID, currentManifestHash, currentItemCount, req); err != nil {
			return nil, err
		}
		manifest := req.GetManifest()
		_, err = tx.Exec(ctx, `INSERT INTO matchmaking_space_lifecycle_fence_heads
(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,applied_at)
VALUES($1,$2,$3,$4,$5,$6,$7,clock_timestamp())
ON CONFLICT(space_id) DO UPDATE SET deletion_operation_id=EXCLUDED.deletion_operation_id,
generation=EXCLUDED.generation,state=EXCLUDED.state,manifest_id=EXCLUDED.manifest_id,
manifest_sha256=EXCLUDED.manifest_sha256,manifest_item_count=EXCLUDED.manifest_item_count,
applied_at=EXCLUDED.applied_at`, spaceID, operationID, int64(req.GetGeneration()), state,
			req.GetManifest().GetManifestId(), manifest.GetManifestSha256(), int64(manifest.GetItemCount()))
		if err != nil {
			return nil, err
		}
		if state == "FROZEN" {
			if _, err := tx.Exec(ctx, `UPDATE search_sessions SET status='cancelled',updated_at=clock_timestamp()
WHERE space_id=$1 AND status IN ('searching','pending_accept','matched')`, spaceID); err != nil {
				return nil, err
			}
		}
	}

	var appliedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&appliedAt); err != nil {
		return nil, err
	}
	receipt := &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: req.GetSpaceId(),
		DeletionOperationId: req.GetDeletionOperationId(), Generation: req.GetGeneration(),
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
		AppliedState:  fenceStateEnum(appliedState), RequestSha256: requestHash,
		ManifestSha256: append([]byte(nil), req.GetManifest().GetManifestSha256()...),
		AppliedAt:      timestamppb.New(appliedAt.UTC()),
	}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	if err != nil {
		return nil, err
	}
	receiptHash := domainHash("voice.space-lifecycle.receipt.v1\x00", receiptBytes)
	_, err = tx.Exec(ctx, `INSERT INTO matchmaking_space_lifecycle_fence_receipts
(space_id,generation,request_bytes,request_sha256,receipt_bytes,receipt_sha256,applied_at,retain_until)
VALUES($1,$2,$3,$4,$5,$6,$7,$7::timestamptz + interval '30 days')`, spaceID, int64(req.GetGeneration()), requestBytes, requestHash, receiptBytes, receiptHash, appliedAt.UTC())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if fenceNeedsQueueCleanup(req.GetDesiredState()) && effect != nil {
		if err := effect(ctx, spaceID); err != nil {
			return nil, err
		}
	}
	return receipt, nil
}

func (s *Store) PurgeSpace(ctx context.Context, req *commonv1.SpacePurgeRequest, effect PurgeEffect) (*commonv1.SpacePurgeReceipt, error) {
	if s == nil || s.pool == nil {
		return nil, ErrUnavailable
	}
	if err := validatePurge(req); err != nil {
		return nil, err
	}
	if effect == nil {
		return nil, ErrUnavailable
	}
	requestBytes, requestHash, err := canonical(&matchmakingv1.PurgeSpaceRequest{Purge: req}, "voice.matchmaking.v1.PurgeSpaceRequest\x00")
	if err != nil {
		return nil, err
	}
	spaceID := uuid.MustParse(req.GetSpaceId())
	operationID := uuid.MustParse(req.GetDeletionOperationId())
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, req.GetSpaceId()); err != nil {
		return nil, err
	}

	var savedRequest, savedHash, savedReceipt, savedReceiptHash []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_bytes,receipt_sha256
FROM matchmaking_space_lifecycle_purge_receipts WHERE space_id=$1 AND generation=$2`, spaceID, int64(req.GetGeneration())).Scan(&savedRequest, &savedHash, &savedReceipt, &savedReceiptHash)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) {
			return nil, ErrConflict
		}
		if !bytes.Equal(savedHash, requestHash) || !validReceipt(savedReceipt, savedReceiptHash, "voice.space-purge.receipt.v1\x00") {
			return nil, ErrUnavailable
		}
		var receipt commonv1.SpacePurgeReceipt
		if err := proto.Unmarshal(savedReceipt, &receipt); err != nil {
			return nil, ErrUnavailable
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		if effect != nil {
			if err := effect(ctx, spaceID); err != nil {
				return nil, err
			}
		}
		return &receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var generation int64
	var state string
	var headOperation uuid.UUID
	var manifestID string
	var manifestHash []byte
	var itemCount int64
	err = tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id,manifest_id,manifest_sha256,manifest_item_count
FROM matchmaking_space_lifecycle_fence_heads WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&generation, &state, &headOperation, &manifestID, &manifestHash, &itemCount)
	if err != nil || (state != "PURGE_DECIDED" && state != "PURGED") || generation != int64(req.GetGeneration()) || headOperation != operationID || manifestID != req.GetManifest().GetManifestId() || !bytes.Equal(manifestHash, req.GetManifest().GetManifestSha256()) || itemCount != int64(req.GetManifest().GetItemCount()) {
		return nil, ErrConflict
	}
	var partyIDs []uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(DISTINCT party_id) FILTER (WHERE party_id IS NOT NULL), '{}') FROM search_sessions WHERE space_id=$1`, spaceID).Scan(&partyIDs); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM matches m WHERE EXISTS (
SELECT 1 FROM jsonb_array_elements(m.participants) p
JOIN search_sessions s ON p->>'session_id'=s.id::text WHERE s.space_id=$1)`, spaceID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM match_proposals WHERE search_session_id IN (SELECT id FROM search_sessions WHERE space_id=$1)`, spaceID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM search_sessions WHERE space_id=$1`, spaceID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM parties p WHERE p.id=ANY($1) AND NOT EXISTS (SELECT 1 FROM search_sessions s WHERE s.party_id=p.id)`, partyIDs); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE matchmaking_space_lifecycle_fence_heads SET state='PURGED',purged_at=COALESCE(purged_at,clock_timestamp()),applied_at=clock_timestamp() WHERE space_id=$1`, spaceID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if effect != nil {
		if err := effect(ctx, spaceID); err != nil {
			return nil, err
		}
	}

	var completedAt time.Time
	if err := s.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completedAt); err != nil {
		return nil, err
	}
	receipt := &commonv1.SpacePurgeReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: req.GetSpaceId(),
		DeletionOperationId: req.GetDeletionOperationId(), Generation: req.GetGeneration(),
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
		State:         commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED,
		RequestSha256: requestHash, CompletedAt: timestamppb.New(completedAt.UTC()),
	}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	if err != nil {
		return nil, err
	}
	receiptHash := domainHash("voice.space-purge.receipt.v1\x00", receiptBytes)
	_, err = s.pool.Exec(ctx, `INSERT INTO matchmaking_space_lifecycle_purge_receipts
(space_id,generation,request_bytes,request_sha256,receipt_bytes,receipt_sha256,completed_at,retain_until)
VALUES($1,$2,$3,$4,$5,$6,$7,$7::timestamptz + interval '30 days')`, spaceID, int64(req.GetGeneration()), requestBytes, requestHash, receiptBytes, receiptHash, completedAt.UTC())
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func validateFence(req *commonv1.SpaceLifecycleFenceRequest) error {
	if req == nil || req.GetProtocolVersion() != 1 || !canonicalUUID(req.GetSpaceId()) || !canonicalUUID(req.GetDeletionOperationId()) || req.GetGeneration() == 0 || req.GetGeneration() > uint64(^uint64(0)>>1) || req.GetManifest() == nil || !canonicalUUID(req.GetManifest().GetManifestId()) || len(req.GetManifest().GetManifestSha256()) != 32 || len(req.ProtoReflect().GetUnknown()) != 0 || len(req.GetManifest().ProtoReflect().GetUnknown()) != 0 {
		return ErrInvalidRequest
	}
	switch req.GetDesiredState() {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		return nil
	default:
		return ErrInvalidRequest
	}
}

func validatePurge(req *commonv1.SpacePurgeRequest) error {
	if req == nil || req.GetProtocolVersion() != 1 || !canonicalUUID(req.GetSpaceId()) || !canonicalUUID(req.GetDeletionOperationId()) || req.GetGeneration() == 0 || req.GetGeneration() > uint64(^uint64(0)>>1) || req.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING || req.GetPurgeDecidedAt() == nil || req.GetPurgeDecidedAt().CheckValid() != nil || req.GetManifest() == nil || !canonicalUUID(req.GetManifest().GetManifestId()) || len(req.GetManifest().GetManifestSha256()) != 32 || len(req.ProtoReflect().GetUnknown()) != 0 || len(req.GetManifest().ProtoReflect().GetUnknown()) != 0 {
		return ErrInvalidRequest
	}
	return nil
}

func validateTransition(generation int64, state string, operation uuid.UUID, manifestID string, manifestHash []byte, itemCount int64, req *commonv1.SpaceLifecycleFenceRequest) error {
	newState := fenceState(req.GetDesiredState())
	if generation == 0 && state == "" {
		if newState != "FROZEN" {
			return ErrConflict
		}
		return nil
	}
	if req.GetGeneration() <= uint64(generation) {
		return ErrConflict
	}
	if state == "PURGE_DECIDED" || state == "PURGED" {
		return ErrConflict
	}
	if generation > 0 && req.GetDeletionOperationId() == operation.String() {
		if req.GetManifest().GetManifestId() != manifestID || !bytes.Equal(req.GetManifest().GetManifestSha256(), manifestHash) || int64(req.GetManifest().GetItemCount()) != itemCount {
			return ErrConflict
		}
		if (state == "FROZEN" && (newState == "LIVE" || newState == "PURGE_DECIDED")) || (state == "LIVE" && newState == "FROZEN") {
			return nil
		}
		return ErrConflict
	}
	if newState != "FROZEN" || state != "LIVE" {
		return ErrConflict
	}
	return nil
}

func canonical(message proto.Message, domain string) ([]byte, []byte, error) {
	data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	if err != nil || len(data) == 0 {
		return nil, nil, ErrInvalidRequest
	}
	return data, domainHash(domain, data), nil
}

func domainHash(domain string, data []byte) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write(data)
	return h.Sum(nil)
}

func validReceipt(data, digest []byte, domain string) bool {
	return len(data) != 0 && len(digest) == sha256.Size && bytes.Equal(domainHash(domain, data), digest)
}

func canonicalUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func fenceState(state commonv1.LifecycleFenceState) string {
	switch state {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		return "FROZEN"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
		return "LIVE"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		return "PURGE_DECIDED"
	default:
		return ""
	}
}

func fenceStateEnum(state string) commonv1.LifecycleFenceState {
	switch state {
	case "FROZEN":
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN
	case "LIVE":
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	case "PURGE_DECIDED":
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	default:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_UNSPECIFIED
	}
}

func fenceNeedsQueueCleanup(state commonv1.LifecycleFenceState) bool {
	return state == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN || state == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
}

func (s *Store) CheckSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return ErrUnavailable
	}
	var ready bool
	err := s.pool.QueryRow(ctx, `SELECT to_regclass('matchmaking_space_lifecycle_fence_heads') IS NOT NULL AND to_regclass('matchmaking_space_lifecycle_fence_receipts') IS NOT NULL AND to_regclass('matchmaking_space_lifecycle_purge_receipts') IS NOT NULL`).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("%w: lifecycle schema missing", ErrUnavailable)
	}
	return nil
}
