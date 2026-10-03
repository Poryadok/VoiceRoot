package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

// GetLifecycleCoordinatorStatus returns the durable participant receipts only
// to the account and profile that admitted the original deletion operation.
// The operation ledger remains the owner proof after the Space row is purged.
func (s *SpaceStore) GetLifecycleCoordinatorStatus(
	ctx context.Context,
	accountID, actorProfileID, spaceID, deletionOperationID uuid.UUID,
) (*spacev1.SpaceDeletionCoordinatorStatus, error) {
	if s == nil || s.Pool == nil || s.tx != nil || accountID == uuid.Nil || actorProfileID == uuid.Nil ||
		spaceID == uuid.Nil || deletionOperationID == uuid.Nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)

	var savedAccountID, savedActorProfileID, savedSpaceID uuid.UUID
	var method string
	err = tx.QueryRow(ctx, `SELECT account_id,actor_profile_id,space_id,method
		FROM space_lifecycle_operations WHERE operation_id=$1`, deletionOperationID).
		Scan(&savedAccountID, &savedActorProfileID, &savedSpaceID, &method)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil &&
		(savedAccountID != accountID || savedActorProfileID != actorProfileID || method != "DELETE")) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	if savedSpaceID != spaceID {
		return nil, ErrLifecycleConflict
	}
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return nil, err
	}
	aggregate, err := loadLifecycle(ctx, tx, spaceID, true)
	if err != nil {
		return nil, err
	}
	snapshot := aggregate.Snapshot()
	if snapshot.DeletionOperationID != deletionOperationID.String() {
		return nil, ErrLifecycleConflict
	}
	result, err := lifecycleStatusFromSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func lifecycleStatusFromSnapshot(snapshot spacecore.LifecycleSnapshot) (*spacev1.SpaceDeletionCoordinatorStatus, error) {
	generation := snapshot.Generation
	result := &spacev1.SpaceDeletionCoordinatorStatus{
		ProtocolVersion:     1,
		SpaceId:             snapshot.SpaceID,
		DeletionOperationId: snapshot.DeletionOperationID,
		Generation:          &generation,
		Phase:               snapshot.Phase,
	}
	if snapshot.Manifest != nil {
		result.Manifest = proto.Clone(snapshot.Manifest).(*commonv1.ManifestBinding)
	}
	if !snapshot.ScheduledAt.IsZero() {
		result.ScheduledAt = timestamppb.New(snapshot.ScheduledAt.UTC())
	}
	if !snapshot.PurgeAfter.IsZero() {
		result.PurgeAfter = timestamppb.New(snapshot.PurgeAfter.UTC())
	}
	if snapshot.DeletedEvent != nil {
		result.PurgedAt = timestamppb.New(snapshot.DeletedEvent.OccurredAt.UTC())
	}
	if (result.ScheduledAt != nil && result.ScheduledAt.CheckValid() != nil) ||
		(result.PurgeAfter != nil && result.PurgeAfter.CheckValid() != nil) ||
		(result.PurgedAt != nil && result.PurgedAt.CheckValid() != nil) {
		return nil, ErrLifecycleEvidenceInvalid
	}

	purgeProgress := snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING ||
		snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED
	result.Participants = make([]*spacev1.ParticipantCoordinatorStatus, 0, len(lifecycleParticipants))
	for _, participantID := range spacecore.CanonicalLifecycleParticipants() {
		participant := &spacev1.ParticipantCoordinatorStatus{
			ParticipantId: participantID,
			State:         spacev1.ParticipantOperationState_PARTICIPANT_OPERATION_STATE_NOT_STARTED,
		}
		var hashErr error
		if purgeProgress {
			if participantID == commonv1.ParticipantId_PARTICIPANT_ID_ROLE && snapshot.RoleReceipt != nil {
				participant.State = spacev1.ParticipantOperationState_PARTICIPANT_OPERATION_STATE_COMPLETE
				participant.RequestSha256 = append([]byte(nil), snapshot.RoleReceipt.GetRequestSha256()...)
				participant.ReceiptSha256, hashErr = lifecycleStatusReceiptHash("voice.role.v1.RetireSpaceReceipt", snapshot.RoleReceipt)
				if hashErr != nil {
					return nil, hashErr
				}
				participant.UpdatedAt = cloneStatusTimestamp(snapshot.RoleReceipt.GetRetiredAt())
			} else if receipt := snapshot.PurgeReceipts[participantID]; receipt != nil {
				participant.State = spacev1.ParticipantOperationState_PARTICIPANT_OPERATION_STATE_COMPLETE
				participant.RequestSha256 = append([]byte(nil), receipt.GetRequestSha256()...)
				participant.ReceiptSha256, hashErr = lifecycleStatusReceiptHash("voice.common.v1.SpacePurgeReceipt", receipt)
				if hashErr != nil {
					return nil, hashErr
				}
				participant.UpdatedAt = cloneStatusTimestamp(receipt.GetCompletedAt())
			}
		} else if receipt := snapshot.FenceReceipts[participantID]; receipt != nil {
			participant.State = spacev1.ParticipantOperationState_PARTICIPANT_OPERATION_STATE_COMPLETE
			participant.RequestSha256 = append([]byte(nil), receipt.GetRequestSha256()...)
			participant.ReceiptSha256, hashErr = lifecycleStatusReceiptHash("voice.common.v1.SpaceLifecycleFenceReceipt", receipt)
			if hashErr != nil {
				return nil, hashErr
			}
			participant.UpdatedAt = cloneStatusTimestamp(receipt.GetAppliedAt())
		}
		if participant.State == spacev1.ParticipantOperationState_PARTICIPANT_OPERATION_STATE_COMPLETE &&
			(len(participant.RequestSha256) != 32 || len(participant.ReceiptSha256) != 32 || participant.UpdatedAt == nil || participant.UpdatedAt.CheckValid() != nil) {
			return nil, ErrLifecycleEvidenceInvalid
		}
		result.Participants = append(result.Participants, participant)
	}
	return result, nil
}

func lifecycleStatusReceiptHash(fqn string, receipt proto.Message) ([]byte, error) {
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	if err != nil {
		return nil, err
	}
	digest := lifecycleHash(fqn, wire)
	return append([]byte(nil), digest[:]...), nil
}

func cloneStatusTimestamp(value *timestamppb.Timestamp) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return proto.Clone(value).(*timestamppb.Timestamp)
}
