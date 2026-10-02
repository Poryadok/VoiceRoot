package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
)

var ErrDeletionFenceInvalid = errors.New("invalid Role deletion fence")
var ErrDeletionFenceConflict = errors.New("Role deletion fence conflict")

func deletionFenceState(state commonv1.LifecycleFenceState) string {
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

func (s *RoleStore) ApplySpaceDeletionFence(ctx context.Context, req *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.ProtocolVersion != 1 || req.Generation == 0 || req.Generation > math.MaxInt64 || req.Manifest == nil || len(req.Manifest.ProtoReflect().GetUnknown()) != 0 || len(req.Manifest.ManifestSha256) != 32 || deletionFenceState(req.DesiredState) == "" {
		return nil, ErrDeletionFenceInvalid
	}
	parse := func(raw string) (uuid.UUID, error) {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return uuid.Nil, ErrDeletionFenceInvalid
		}
		return id, nil
	}
	space, err := parse(req.SpaceId)
	if err != nil {
		return nil, err
	}
	op, err := parse(req.DeletionOperationId)
	if err != nil {
		return nil, err
	}
	manifest, err := parse(req.Manifest.ManifestId)
	if err != nil {
		return nil, err
	}
	wrapped := &rolev1.ApplySpaceLifecycleFenceRequest{Fence: req}
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(wrapped)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append(append([]byte(wrapped.ProtoReflect().Descriptor().FullName()), 0), wire...))
	var result *commonv1.SpaceLifecycleFenceReceipt
	err = s.withLockedTransaction(ctx, op, []uuid.UUID{space}, func(scoped *RoleStore) error {
		db := scoped.db()
		var oldRequest, oldDigest, oldReceipt []byte
		var savedOp, savedManifest, savedReceiptID uuid.UUID
		var savedState string
		var savedManifestHash []byte
		var savedCount uint64
		var savedTime time.Time
		err := db.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_bytes,deletion_operation_id,state,manifest_id,manifest_sha256,manifest_item_count,receipt_id,applied_at FROM role_space_deletion_fence_receipts WHERE space_id=$1 AND generation=$2`, space, int64(req.Generation)).Scan(&oldRequest, &oldDigest, &oldReceipt, &savedOp, &savedState, &savedManifest, &savedManifestHash, &savedCount, &savedReceiptID, &savedTime)
		if err == nil {
			if (oldRequest != nil && !bytes.Equal(oldRequest, wire)) || !bytes.Equal(oldDigest, digest[:]) || savedOp != op || savedState != deletionFenceState(req.DesiredState) || savedManifest != manifest || !bytes.Equal(savedManifestHash, req.Manifest.ManifestSha256) || savedCount != req.Manifest.ItemCount {
				return ErrDeletionFenceConflict
			}
			result = &commonv1.SpaceLifecycleFenceReceipt{}
			if oldReceipt == nil {
				result = &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: savedReceiptID.String(), SpaceId: req.SpaceId, DeletionOperationId: req.DeletionOperationId, Generation: req.Generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_ROLE, AppliedState: req.DesiredState, RequestSha256: digest[:], ManifestSha256: bytes.Clone(savedManifestHash), AppliedAt: timestamppb.New(savedTime.UTC())}
			} else if err := proto.Unmarshal(oldReceipt, result); err != nil {
				return err
			}
			if result.ProtocolVersion != 1 || result.SpaceId != req.SpaceId || result.DeletionOperationId != req.DeletionOperationId || result.Generation != req.Generation || result.AppliedState != req.DesiredState || !bytes.Equal(result.RequestSha256, digest[:]) || !bytes.Equal(result.ManifestSha256, req.Manifest.ManifestSha256) || result.ParticipantId != commonv1.ParticipantId_PARTICIPANT_ID_ROLE || result.ReceiptId != savedReceiptID.String() || result.AppliedAt == nil || result.AppliedAt.CheckValid() != nil || !result.AppliedAt.AsTime().Equal(savedTime) {
				return ErrDeletionFenceConflict
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var retired, prepared bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL),EXISTS(SELECT 1 FROM ownership_transfer_v2 WHERE space_id=$1 AND state='prepared')`, space).Scan(&retired, &prepared); err != nil {
			return err
		}
		if retired {
			return ErrSpaceRetired
		}
		if prepared {
			return ErrSpaceFrozen
		}
		var generation int64
		var oldState string
		var oldOp, oldManifest uuid.UUID
		var oldManifestHash []byte
		var oldCount uint64
		err = db.QueryRow(ctx, `SELECT generation,state,deletion_operation_id,manifest_id,manifest_sha256,manifest_item_count FROM role_space_deletion_fences WHERE space_id=$1`, space).Scan(&generation, &oldState, &oldOp, &oldManifest, &oldManifestHash, &oldCount)
		state := deletionFenceState(req.DesiredState)
		if errors.Is(err, pgx.ErrNoRows) {
			if req.Generation != 1 || state != "FROZEN" {
				return ErrDeletionFenceConflict
			}
		} else {
			if err != nil {
				return err
			}
			if generation == math.MaxInt64 || req.Generation != uint64(generation)+1 || oldState == "PURGE_DECIDED" {
				return ErrDeletionFenceConflict
			}
			if oldState == "LIVE" {
				if state != "FROZEN" || oldOp == op {
					return ErrDeletionFenceConflict
				}
			} else if oldState != "FROZEN" || (state != "LIVE" && state != "PURGE_DECIDED") || oldOp != op || oldManifest != manifest || !bytes.Equal(oldManifestHash, req.Manifest.ManifestSha256) || oldCount != req.Manifest.ItemCount {
				return ErrDeletionFenceConflict
			}
		}
		var applied time.Time
		if err := db.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&applied); err != nil {
			return err
		}
		result = &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: req.SpaceId, DeletionOperationId: req.DeletionOperationId, Generation: req.Generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_ROLE, AppliedState: req.DesiredState, RequestSha256: digest[:], ManifestSha256: bytes.Clone(req.Manifest.ManifestSha256), AppliedAt: timestamppb.New(applied.UTC())}
		receiptWire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(result)
		if err != nil {
			return err
		}
		_, err = db.Exec(ctx, `INSERT INTO role_space_deletion_fences(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(space_id) DO UPDATE SET deletion_operation_id=EXCLUDED.deletion_operation_id,generation=EXCLUDED.generation,state=EXCLUDED.state,manifest_id=EXCLUDED.manifest_id,manifest_sha256=EXCLUDED.manifest_sha256,manifest_item_count=EXCLUDED.manifest_item_count`, space, op, int64(req.Generation), state, manifest, req.Manifest.ManifestSha256, req.Manifest.ItemCount)
		if err != nil {
			return err
		}
		_, err = db.Exec(ctx, `INSERT INTO role_space_deletion_fence_receipts(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,receipt_id,applied_at,request_sha256,request_bytes,receipt_bytes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, space, op, int64(req.Generation), state, manifest, req.Manifest.ManifestSha256, req.Manifest.ItemCount, uuid.MustParse(result.ReceiptId), applied, digest[:], wire, receiptWire)
		return err
	})
	return result, err
}

func checkRoleDeletionFence(ctx context.Context, db scopeExecutor, spaces []uuid.UUID) error {
	var frozen bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_space_deletion_fences WHERE space_id=ANY($1) AND state<>'LIVE')`, spaces).Scan(&frozen); err != nil {
		return fmt.Errorf("%w: deletion fence lookup: %w", ErrScopeUnavailable, err)
	}
	if frozen {
		return ErrSpaceFrozen
	}
	return nil
}

// Compact only full bytes after permanent retirement's database retention window.
// Semantic tuple, stable receipt and current head remain permanent.
func (s *RoleStore) CleanupSpaceDeletionFenceEvidence(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return ErrScopeUnavailable
	}
	_, err := s.Pool.Exec(ctx, `UPDATE role_space_deletion_fence_receipts r SET request_bytes=NULL,receipt_bytes=NULL FROM role_space_lifecycle l WHERE l.space_id=r.space_id AND l.retired_at IS NOT NULL AND clock_timestamp()>=l.retired_at+interval '30 days' AND r.request_bytes IS NOT NULL`)
	return err
}
