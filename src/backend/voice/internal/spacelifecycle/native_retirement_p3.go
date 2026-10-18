package spacelifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/retirementproof"
)

// This private branch is reachable only after trusted proof acceptance and fresh
// owner-terminal checks in the SAME caller transaction. The generic EJECT_ONLY
// Service branch is unchanged and cannot call it or import an arbitrary proof.
func completeQualifiedOwnerP3(ctx context.Context, tx pgx.Tx, work retirementproof.OwnerWork, proof retirementproof.ProofRef, now time.Time) ([]byte, error) {
	p := work.ExistingP3
	if work.Purpose != retirementproof.P3FrozenCompletion || p == nil {
		return nil, ErrConflict
	}
	var receiptID uuid.UUID
	var request, hash, oldReceipt []byte
	var sealed bool
	if err := tx.QueryRow(ctx, `SELECT receipt_id,request_bytes,request_sha256,receipt_bytes,coverage_sealed FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=$2 AND applied_state='FROZEN' FOR UPDATE`, work.SpaceID, int64(p.Generation)).Scan(&receiptID, &request, &hash, &oldReceipt, &sealed); err != nil {
		return nil, err
	}
	if !sealed || len(oldReceipt) != 0 || !bytes.Equal(request, p.CanonicalRequestBytes) || !bytes.Equal(hash, p.RequestHash[:]) {
		return nil, ErrConflict
	}
	targets, err := validateNativeOwnerManifest(work)
	if err != nil {
		return nil, err
	}
	expected := map[string]nativeOwnerTarget{}
	for _, target := range targets {
		if target.Kind == "native" || target.Kind == "head" || target.Kind == "legacy" {
			expected[target.ID] = target
		}
	}
	rows, err := tx.Query(ctx, `SELECT target_id,source_kind,target FROM voice_space_lifecycle_targets WHERE space_id=$1 AND generation=$2 AND deletion_operation_id=$3 AND request_sha256=$4 ORDER BY target_id`, work.SpaceID, int64(p.Generation), p.DeletionOperationID, p.RequestHash[:])
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, kind string
		var raw []byte
		if err := rows.Scan(&id, &kind, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		target, ok := expected[id]
		if !ok || target.Kind != kind || !sameJSON(json.RawMessage(raw), target.Data) {
			rows.Close()
			return nil, ErrConflict
		}
		delete(expected, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(expected) != 0 {
		return nil, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_space_lifecycle_targets t SET cleanup_completed_at=COALESCE(t.cleanup_completed_at,$5) WHERE t.space_id=$1 AND t.generation=$2 AND t.deletion_operation_id=$3 AND t.request_sha256=$4 AND EXISTS(SELECT 1 FROM voice_native_owner_targets n WHERE n.work_id=$6 AND n.target_id=t.target_id AND n.source_kind=t.source_kind AND n.cleanup_completed_at IS NOT NULL AND n.effect_kind='REALM_RETIRED' AND n.effect_proof_id=$7)`, work.SpaceID, int64(p.Generation), p.DeletionOperationID, p.RequestHash[:], now.UTC(), work.Ref.WorkID, proof.ProofID); err != nil {
		return nil, err
	}
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_space_lifecycle_targets WHERE space_id=$1 AND generation=$2 AND cleanup_completed_at IS NULL)`, work.SpaceID, int64(p.Generation)).Scan(&pending); err != nil || pending {
		return nil, ErrUnavailable
	}
	receipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: receiptID.String(), SpaceId: work.SpaceID.String(), DeletionOperationId: p.DeletionOperationID.String(), Generation: p.Generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE, AppliedState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, RequestSha256: append([]byte(nil), p.RequestHash[:]...), ManifestSha256: append([]byte(nil), p.ManifestHash[:]...), AppliedAt: timestamppb.New(now.UTC())}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	if err != nil {
		return nil, err
	}
	digest := domainHash("voice.space-lifecycle.receipt.v1\x00", encoded)
	tag, err := tx.Exec(ctx, `UPDATE voice_space_lifecycle_fence_receipts r SET receipt_bytes=$4,receipt_sha256=$5,applied_at=$6,retirement_proven=TRUE,retirement_owner_work_id=$7 WHERE r.space_id=$1 AND r.generation=$2 AND r.receipt_id=$3 AND r.receipt_bytes IS NULL AND r.coverage_sealed AND r.request_sha256=$8 AND EXISTS(SELECT 1 FROM voice_native_owner_acceptances a WHERE a.work_id=$7 AND a.proof_id=$9 AND a.resource_receipt_sha256=$10)`, work.SpaceID, int64(p.Generation), receiptID, encoded, digest, now.UTC(), work.Ref.WorkID, p.RequestHash[:], proof.ProofID, proof.ReceiptHash[:])
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrConflict
	}
	return encoded, nil
}
