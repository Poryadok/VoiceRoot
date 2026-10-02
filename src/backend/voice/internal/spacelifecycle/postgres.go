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
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

type FenceEffect func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error
type PurgeEffect func(context.Context, *commonv1.SpacePurgeRequest) error

func (s *PostgresStore) CheckSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return ErrUnavailable
	}
	var ready bool
	err := s.pool.QueryRow(ctx, `SELECT to_regclass('voice_space_lifecycle_fence_heads') IS NOT NULL
AND to_regclass('voice_space_lifecycle_fence_receipts') IS NOT NULL
AND to_regclass('voice_space_lifecycle_purge_receipts') IS NOT NULL`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("check Voice Space lifecycle schema: %w", err)
	}
	if !ready {
		return errors.New("Voice Space lifecycle schema is missing")
	}
	return nil
}

// ApplyFence durably advances the admission fence before invoking effects. A failed
// effect leaves the pending receipt and restrictive head in place for exact retry.
func (s *PostgresStore) ApplyFence(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest, effect FenceEffect) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if s == nil || s.pool == nil {
		return nil, ErrUnavailable
	}
	if err := validateFenceRequest(req); err != nil {
		return nil, err
	}
	requestBytes, requestHash, err := canonicalProto(req)
	if err != nil {
		return nil, err
	}
	spaceID, _ := uuid.Parse(req.GetSpaceId())
	operationID, _ := uuid.Parse(req.GetDeletionOperationId())
	state := fenceStateString(req.GetDesiredState())
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, req.GetSpaceId()); err != nil {
		return nil, err
	}
	var oldRequest, oldRequestHash, oldReceipt, oldReceiptHash []byte
	var oldReceiptID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_bytes,receipt_sha256,receipt_id FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=$2`, spaceID, int64(req.GetGeneration())).Scan(&oldRequest, &oldRequestHash, &oldReceipt, &oldReceiptHash, &oldReceiptID)
	if err == nil {
		if !bytes.Equal(oldRequest, requestBytes) {
			return nil, ErrConflict
		}
		if !bytes.Equal(oldRequestHash, requestHash) {
			return nil, ErrUnavailable
		}
		if len(oldReceipt) != 0 {
			if !validReceiptDigest(oldReceipt, oldReceiptHash, "voice.space-lifecycle.receipt.v1\x00") {
				return nil, ErrUnavailable
			}
			var receipt commonv1.SpaceLifecycleFenceReceipt
			if err := proto.Unmarshal(oldReceipt, &receipt); err != nil {
				return nil, ErrUnavailable
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return &receipt, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else {
		var currentGeneration int64
		var currentState string
		headErr := tx.QueryRow(ctx, `SELECT generation,state FROM voice_space_lifecycle_fence_heads WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&currentGeneration, &currentState)
		if headErr != nil && !errors.Is(headErr, pgx.ErrNoRows) {
			return nil, headErr
		}
		stale := headErr == nil && req.GetGeneration() < uint64(currentGeneration)
		appliedState := state
		if stale {
			appliedState = currentState
		} else {
			if err := validateTransition(currentGeneration, currentState, req.GetGeneration(), state); err != nil {
				return nil, err
			}
			manifest := req.GetManifest()
			_, err = tx.Exec(ctx, `INSERT INTO voice_space_lifecycle_fence_heads(space_id,generation,deletion_operation_id,state,manifest_id,manifest_sha256,manifest_item_count)
VALUES($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT(space_id) DO UPDATE SET generation=EXCLUDED.generation,deletion_operation_id=EXCLUDED.deletion_operation_id,
 state=EXCLUDED.state,manifest_id=EXCLUDED.manifest_id,manifest_sha256=EXCLUDED.manifest_sha256,
 manifest_item_count=EXCLUDED.manifest_item_count,updated_at=clock_timestamp()`, spaceID, int64(req.GetGeneration()), operationID, state, uuid.MustParse(manifest.GetManifestId()), manifest.GetManifestSha256(), manifest.GetItemCount())
			if err != nil {
				return nil, err
			}
		}
		receiptID := uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO voice_space_lifecycle_fence_receipts(space_id,generation,deletion_operation_id,request_bytes,request_sha256,receipt_id,applied_state)
VALUES($1,$2,$3,$4,$5,$6,$7)`, spaceID, int64(req.GetGeneration()), operationID, requestBytes, requestHash, receiptID, appliedState)
		if err != nil {
			return nil, err
		}
		oldReceiptID = receiptID
		if stale {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return s.completeFence(ctx, req, requestHash, oldReceiptID, appliedState)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if effect == nil {
		return nil, ErrUnavailable
	}
	if err := effect(ctx, proto.Clone(req).(*commonv1.SpaceLifecycleFenceRequest)); err != nil {
		return nil, err
	}
	return s.completeFence(ctx, req, requestHash, oldReceiptID, state)
}

func (s *PostgresStore) completeFence(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest, requestHash []byte, receiptID uuid.UUID, state string) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	spaceID, _ := uuid.Parse(req.GetSpaceId())
	var appliedAt time.Time
	if err := s.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&appliedAt); err != nil {
		return nil, err
	}
	receipt := &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), SpaceId: req.GetSpaceId(), DeletionOperationId: req.GetDeletionOperationId(),
		Generation: req.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
		AppliedState: fenceStateEnum(state), RequestSha256: append([]byte(nil), requestHash...),
		ManifestSha256: append([]byte(nil), req.GetManifest().GetManifestSha256()...), AppliedAt: timestamppb.New(appliedAt.UTC()),
	}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	if err != nil {
		return nil, err
	}
	receiptHash := domainHash("voice.space-lifecycle.receipt.v1\x00", receiptBytes)
	tag, err := s.pool.Exec(ctx, `UPDATE voice_space_lifecycle_fence_receipts SET receipt_bytes=$4,receipt_sha256=$5,applied_at=$6
WHERE space_id=$1 AND generation=$2 AND receipt_id=$3 AND receipt_bytes IS NULL`, spaceID, int64(req.GetGeneration()), receiptID, receiptBytes, receiptHash, appliedAt.UTC())
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 1 {
		return receipt, nil
	}
	var saved []byte
	var savedHash []byte
	if err := s.pool.QueryRow(ctx, `SELECT receipt_bytes,receipt_sha256 FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=$2 AND receipt_id=$3`, spaceID, int64(req.GetGeneration()), receiptID).Scan(&saved, &savedHash); err != nil {
		return nil, err
	}
	if !validReceiptDigest(saved, savedHash, "voice.space-lifecycle.receipt.v1\x00") {
		return nil, ErrUnavailable
	}
	var stored commonv1.SpaceLifecycleFenceReceipt
	if err := proto.Unmarshal(saved, &stored); err != nil {
		return nil, ErrUnavailable
	}
	return &stored, nil
}

// PurgeSpace requires the irreversible durable PURGE_DECIDED fence. It stores
// request identity before effects and only persists COMPLETED after all effects
// succeed; retries therefore repeat idempotent cleanup without minting receipts.
func (s *PostgresStore) PurgeSpace(ctx context.Context, req *commonv1.SpacePurgeRequest, effect PurgeEffect) (*commonv1.SpacePurgeReceipt, error) {
	if s == nil || s.pool == nil {
		return nil, ErrUnavailable
	}
	if err := validatePurgeRequest(req); err != nil {
		return nil, err
	}
	requestBytes, requestHash, err := canonicalProto(req)
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
	var generation int64
	var state string
	var headOperation, manifestID uuid.UUID
	var manifestHash []byte
	var manifestCount string
	if err := tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id,manifest_id,manifest_sha256,manifest_item_count::text
FROM voice_space_lifecycle_fence_heads WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&generation, &state, &headOperation, &manifestID, &manifestHash, &manifestCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, err
	}
	manifest := req.GetManifest()
	if state != "PURGE_DECIDED" || generation != int64(req.GetGeneration()) || headOperation != operationID ||
		manifestID != uuid.MustParse(manifest.GetManifestId()) || !bytes.Equal(manifestHash, manifest.GetManifestSha256()) || manifestCount != fmt.Sprint(manifest.GetItemCount()) {
		return nil, ErrConflict
	}
	var oldRequest, oldRequestHash, oldReceipt, oldReceiptHash []byte
	var receiptID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_bytes,receipt_sha256,receipt_id FROM voice_space_lifecycle_purge_receipts WHERE space_id=$1 AND generation=$2`, spaceID, generation).Scan(&oldRequest, &oldRequestHash, &oldReceipt, &oldReceiptHash, &receiptID)
	if err == nil {
		if !bytes.Equal(oldRequest, requestBytes) {
			return nil, ErrConflict
		}
		if !bytes.Equal(oldRequestHash, requestHash) {
			return nil, ErrUnavailable
		}
		if len(oldReceipt) != 0 {
			if !validReceiptDigest(oldReceipt, oldReceiptHash, "voice.space-purge.receipt.v1\x00") {
				return nil, ErrUnavailable
			}
			var receipt commonv1.SpacePurgeReceipt
			if err := proto.Unmarshal(oldReceipt, &receipt); err != nil {
				return nil, ErrUnavailable
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return &receipt, nil
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		receiptID = uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO voice_space_lifecycle_purge_receipts(space_id,generation,deletion_operation_id,request_bytes,request_sha256,receipt_id)
VALUES($1,$2,$3,$4,$5,$6)`, spaceID, generation, operationID, requestBytes, requestHash, receiptID)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if effect == nil {
		return nil, ErrUnavailable
	}
	if err := effect(ctx, proto.Clone(req).(*commonv1.SpacePurgeRequest)); err != nil {
		return nil, err
	}
	return s.completePurge(ctx, req, requestHash, receiptID)
}

func (s *PostgresStore) completePurge(ctx context.Context, req *commonv1.SpacePurgeRequest, requestHash []byte, receiptID uuid.UUID) (*commonv1.SpacePurgeReceipt, error) {
	var completedAt time.Time
	if err := s.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completedAt); err != nil {
		return nil, err
	}
	completedAt = completedAt.UTC()
	receipt := &commonv1.SpacePurgeReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), SpaceId: req.GetSpaceId(),
		DeletionOperationId: req.GetDeletionOperationId(), Generation: req.GetGeneration(),
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
		State:         commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED,
		RequestSha256: append([]byte(nil), requestHash...), CompletedAt: timestamppb.New(completedAt),
	}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	if err != nil {
		return nil, err
	}
	receiptHash := domainHash("voice.space-purge.receipt.v1\x00", receiptBytes)
	tag, err := s.pool.Exec(ctx, `UPDATE voice_space_lifecycle_purge_receipts SET receipt_bytes=$4,receipt_sha256=$5,completed_at=$6
WHERE space_id=$1 AND generation=$2 AND receipt_id=$3 AND receipt_bytes IS NULL`, uuid.MustParse(req.GetSpaceId()), int64(req.GetGeneration()), receiptID, receiptBytes, receiptHash, completedAt)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 1 {
		return receipt, nil
	}
	var saved []byte
	var savedHash []byte
	if err := s.pool.QueryRow(ctx, `SELECT receipt_bytes,receipt_sha256 FROM voice_space_lifecycle_purge_receipts WHERE space_id=$1 AND generation=$2 AND receipt_id=$3`, uuid.MustParse(req.GetSpaceId()), int64(req.GetGeneration()), receiptID).Scan(&saved, &savedHash); err != nil {
		return nil, err
	}
	if !validReceiptDigest(saved, savedHash, "voice.space-purge.receipt.v1\x00") {
		return nil, ErrUnavailable
	}
	var stored commonv1.SpacePurgeReceipt
	if err := proto.Unmarshal(saved, &stored); err != nil {
		return nil, ErrUnavailable
	}
	return &stored, nil
}

// ExpireReceipts removes full request/receipt evidence after its 30-day replay
// period. The compact current fence remains permanently to prevent resurrection.
func (s *PostgresStore) ExpireReceipts(ctx context.Context, before time.Time) error {
	if s == nil || s.pool == nil || before.IsZero() {
		return ErrUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM voice_space_lifecycle_fence_receipts WHERE applied_at < $1`, before.UTC()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM voice_space_lifecycle_purge_receipts WHERE completed_at < $1`, before.UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CheckAdmission(ctx context.Context, spaceID string) error {
	if s == nil || s.pool == nil || !canonicalUUID(spaceID) {
		return ErrUnavailable
	}
	var state string
	err := s.pool.QueryRow(ctx, `SELECT state FROM voice_space_lifecycle_fence_heads WHERE space_id=$1`, uuid.MustParse(spaceID)).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state != "LIVE" {
		return ErrSpaceFrozen
	}
	return nil
}

func validateTransition(currentGeneration int64, currentState string, generation uint64, desired string) error {
	if currentGeneration == 0 {
		if desired != "FROZEN" {
			return ErrConflict
		}
		return nil
	}
	if currentState == "PURGE_DECIDED" || generation != uint64(currentGeneration)+1 {
		return ErrConflict
	}
	switch desired {
	case "LIVE", "PURGE_DECIDED":
		if currentState != "FROZEN" {
			return ErrConflict
		}
	case "FROZEN":
		if currentState != "LIVE" {
			return ErrConflict
		}
	default:
		return ErrInvalidRequest
	}
	return nil
}

func fenceStateString(state commonv1.LifecycleFenceState) string {
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

func domainHash(domain string, bytes []byte) []byte {
	return append(sha256Sum([]byte(domain), bytes), []byte{}...)
}

func validReceiptDigest(receipt, digest []byte, domain string) bool {
	return len(receipt) > 0 && len(digest) == sha256.Size && bytes.Equal(domainHash(domain, receipt), digest)
}

func sha256Sum(parts ...[]byte) []byte {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write(part)
	}
	return hash.Sum(nil)
}
