package store

import (
	"bytes"
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// BotPurgeHandoff contains only original participant evidence already accepted
// by Space. It is not a new participant receipt or aggregate completion.
type BotPurgeHandoff struct {
	Role      *BotPurgeRoleEvidence
	Messaging *BotPurgeMessagingEvidence
}

type BotPurgeRoleEvidence struct {
	Receipt                                                  *rolev1.RetireSpaceReceipt
	RequestBytes, RequestSHA256, ReceiptBytes, ReceiptSHA256 []byte
}

type BotPurgeMessagingEvidence struct {
	Receipt                                                  *commonv1.SpacePurgeReceipt
	RequestBytes, RequestSHA256, ReceiptBytes, ReceiptSHA256 []byte
}

// GetBotPurgeHandoff is a read-only participant-evidence relay. Its protected
// handler authenticates the finite service:bot purpose before entering here.
// It deliberately does not borrow owner status or require Bot/aggregate PURGED.
func (s *SpaceStore) GetBotPurgeHandoff(ctx context.Context, space, operation uuid.UUID, generation uint64, manifest *commonv1.ManifestBinding) (*BotPurgeHandoff, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil || operation == uuid.Nil || generation == 0 || generation > math.MaxInt64 || manifest == nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	manifest = proto.Clone(manifest).(*commonv1.ManifestBinding)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockLifecycleSpace(ctx, tx, space); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	aggregate, err := loadLifecycle(ctx, tx, space, true)
	if err != nil {
		return nil, err
	}
	snapshot := aggregate.Snapshot()
	if snapshot.DeletionOperationID != operation.String() || snapshot.Generation != generation || !proto.Equal(snapshot.Manifest, manifest) {
		return nil, ErrLifecycleConflict
	}
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED && snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING && snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED {
		return nil, ErrLifecycleStateTransition
	}
	result := &BotPurgeHandoff{}
	for _, participant := range []commonv1.ParticipantId{commonv1.ParticipantId_PARTICIPANT_ID_ROLE, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING} {
		kind := "PURGE"
		if participant == commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
			kind = "ROLE_RETIREMENT"
		}
		var progress string
		var requestBytes, requestHash, receiptBytes, receiptHash []byte
		err = tx.QueryRow(ctx, `SELECT progress,request_bytes,request_sha256,receipt_bytes,receipt_sha256
 FROM public.space_lifecycle_participants WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND participant_id=$4 AND request_kind=$5`, space, operation, int64(generation), int16(participant), kind).Scan(&progress, &requestBytes, &requestHash, &receiptBytes, &receiptHash)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		if progress != "COMPLETE" {
			continue
		}
		var typed proto.Message
		if participant == commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
			if snapshot.RoleReceipt == nil {
				return nil, ErrLifecycleEvidenceInvalid
			}
			typed = proto.Clone(snapshot.RoleReceipt)
		} else {
			if snapshot.PurgeReceipts[participant] == nil {
				return nil, ErrLifecycleEvidenceInvalid
			}
			typed = proto.Clone(snapshot.PurgeReceipts[participant])
		}
		canonical, marshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(typed)
		if marshalErr != nil || !bytes.Equal(canonical, receiptBytes) {
			return nil, ErrLifecycleEvidenceInvalid
		}
		if participant == commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
			result.Role = &BotPurgeRoleEvidence{Receipt: typed.(*rolev1.RetireSpaceReceipt), RequestBytes: bytes.Clone(requestBytes), RequestSHA256: bytes.Clone(requestHash), ReceiptBytes: bytes.Clone(receiptBytes), ReceiptSHA256: bytes.Clone(receiptHash)}
		} else {
			result.Messaging = &BotPurgeMessagingEvidence{Receipt: typed.(*commonv1.SpacePurgeReceipt), RequestBytes: bytes.Clone(requestBytes), RequestSHA256: bytes.Clone(requestHash), ReceiptBytes: bytes.Clone(receiptBytes), ReceiptSHA256: bytes.Clone(receiptHash)}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
