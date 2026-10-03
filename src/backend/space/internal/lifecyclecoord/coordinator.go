// Package lifecyclecoord advances durable Space lifecycle barriers by calling
// every contract participant and persisting each exact receipt immediately.
package lifecyclecoord

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

const participantRPCDeadline = 10 * time.Second

var ErrCoordinatorNotConfigured = errors.New("space lifecycle coordinator is not configured")

// Store is the durable barrier boundary. Each receipt is committed before the
// next participant is contacted, so process loss resumes from saved progress.
type Store interface {
	LoadLifecycle(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, error)
	RecordLifecycleFenceReceipt(context.Context, *commonv1.SpaceLifecycleFenceReceipt) (*spacecore.LifecycleAggregate, error)
	CompleteLifecycleSchedule(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error)
	CompleteLifecycleRestore(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error)
	RecordLifecycleRoleRetirementReceipt(context.Context, *rolev1.RetireSpaceReceipt) (*spacecore.LifecycleAggregate, error)
	RecordLifecyclePurgeReceipt(context.Context, *commonv1.SpacePurgeReceipt) (*spacecore.LifecycleAggregate, error)
	PersistLifecycle(context.Context, *spacecore.LifecycleAggregate) error
}

type FenceParticipant interface {
	ApplySpaceLifecycleFence(context.Context, *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error)
}

type RoleRetirementParticipant interface {
	RetireSpace(context.Context, *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceReceipt, error)
}

type PurgeParticipant interface {
	PurgeSpace(context.Context, *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error)
}
type SpaceFileProducer interface {
	ReleaseSpaceFileProducer(context.Context, spacecore.LifecycleSnapshot) error
}

type Dependencies struct {
	Store             Store
	Participants      map[commonv1.ParticipantId]FenceParticipant
	RoleRetirement    RoleRetirementParticipant
	PurgeParticipants map[commonv1.ParticipantId]PurgeParticipant
	Manifests         ManifestOwners
	FileProducer      SpaceFileProducer
	RPCDeadline       time.Duration
}

type Coordinator struct{ dependencies Dependencies }

func New(dependencies Dependencies) *Coordinator { return &Coordinator{dependencies: dependencies} }

// ScheduleDeletion resumes the complete schedule-side lifecycle attempt. A
// retry repeats immutable participant requests and relies on owner receipts
// plus the Space ledger to recover any ambiguous boundary.
func (c *Coordinator) ScheduleDeletion(ctx context.Context, spaceID uuid.UUID) error {
	if c == nil {
		return ErrCoordinatorNotConfigured
	}
	if _, err := c.PrepareFreeze(ctx, spaceID); err != nil {
		return err
	}
	_, _, err := c.ApplyFenceBarrier(ctx, spaceID)
	return err
}

// RestoreSpace resumes only a durable restore decision. The complete participant
// barrier remains the authority that makes the Space live again.
func (c *Coordinator) RestoreSpace(ctx context.Context, spaceID uuid.UUID) error {
	if c == nil || c.dependencies.Store == nil || spaceID == uuid.Nil {
		return ErrCoordinatorNotConfigured
	}
	aggregate, err := c.dependencies.Store.LoadLifecycle(ctx, spaceID)
	if err != nil {
		return err
	}
	if aggregate.Phase() != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED {
		return fmt.Errorf("restore barrier is not pending in phase %s", aggregate.Phase())
	}
	_, _, err = c.ApplyFenceBarrier(ctx, spaceID)
	return err
}

// ApplyFenceBarrier contacts participants in canonical order. Each RPC gets a
// fresh bounded context, and exact durable receipts are skipped on restart.
// A participant error leaves the phase pending and is returned for the retry
// worker; no absent participant is interpreted as an empty successful result.
func (c *Coordinator) ApplyFenceBarrier(ctx context.Context, spaceID uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error) {
	if c == nil || c.dependencies.Store == nil || spaceID == uuid.Nil {
		return nil, spacecore.LifecycleOutboxRecord{}, ErrCoordinatorNotConfigured
	}
	for _, participantID := range spacecore.CanonicalLifecycleParticipants() {
		if c.dependencies.Participants[participantID] == nil {
			return nil, spacecore.LifecycleOutboxRecord{}, fmt.Errorf("%w: participant %s is unavailable", ErrCoordinatorNotConfigured, participantID)
		}
	}
	aggregate, err := c.dependencies.Store.LoadLifecycle(ctx, spaceID)
	if err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	snapshot := aggregate.Snapshot()
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING &&
		snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED &&
		snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED {
		return nil, spacecore.LifecycleOutboxRecord{}, fmt.Errorf("fence barrier is not pending in phase %s", snapshot.Phase)
	}
	deadline := c.dependencies.RPCDeadline
	if deadline <= 0 {
		deadline = participantRPCDeadline
	}

	for _, participantID := range spacecore.CanonicalLifecycleParticipants() {
		if snapshot.FenceReceipts[participantID] != nil {
			continue
		}
		request, err := aggregate.FenceRequest(participantID)
		if err != nil {
			return nil, spacecore.LifecycleOutboxRecord{}, err
		}
		callCtx, cancel := context.WithTimeout(ctx, deadline)
		receipt, callErr := c.dependencies.Participants[participantID].ApplySpaceLifecycleFence(callCtx, request)
		cancel()
		if callErr != nil {
			return nil, spacecore.LifecycleOutboxRecord{}, fmt.Errorf("participant %s lifecycle fence: %w", participantID, callErr)
		}
		if receipt == nil || receipt.GetParticipantId() != participantID {
			return nil, spacecore.LifecycleOutboxRecord{}, fmt.Errorf("participant %s returned a mismatched lifecycle fence receipt", participantID)
		}
		aggregate, err = c.dependencies.Store.RecordLifecycleFenceReceipt(ctx, receipt)
		if err != nil {
			return nil, spacecore.LifecycleOutboxRecord{}, fmt.Errorf("persist participant %s lifecycle fence receipt: %w", participantID, err)
		}
	}

	switch aggregate.Phase() {
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING:
		return c.dependencies.Store.CompleteLifecycleSchedule(ctx, spaceID)
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED:
		return c.dependencies.Store.CompleteLifecycleRestore(ctx, spaceID)
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED:
		if err := aggregate.BeginPurging(); err != nil {
			return nil, spacecore.LifecycleOutboxRecord{}, err
		}
		if err := c.dependencies.Store.PersistLifecycle(ctx, aggregate); err != nil {
			return nil, spacecore.LifecycleOutboxRecord{}, err
		}
		return aggregate, spacecore.LifecycleOutboxRecord{}, nil
	default:
		return nil, spacecore.LifecycleOutboxRecord{}, fmt.Errorf("fence barrier phase advanced unexpectedly to %s", aggregate.Phase())
	}
}

// ApplyPurgeBarrier retires Role before purging dependent owners, then persists
// every exact receipt as it arrives. The dependency order ensures Messaging and
// Chat releases its producer references after Messaging completion, then File
// verifies all three producer releases before its participant completion.
// Saved receipts are skipped on restart, so ambiguous RPC outcomes retry only
// the still-unrecorded immutable request.
func (c *Coordinator) ApplyPurgeBarrier(ctx context.Context, spaceID uuid.UUID) (*spacecore.LifecycleAggregate, error) {
	if c == nil || c.dependencies.Store == nil || spaceID == uuid.Nil || c.dependencies.RoleRetirement == nil || c.dependencies.FileProducer == nil {
		return nil, ErrCoordinatorNotConfigured
	}
	purgeOrder := []commonv1.ParticipantId{
		commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		commonv1.ParticipantId_PARTICIPANT_ID_FILE,
		commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
		commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
		commonv1.ParticipantId_PARTICIPANT_ID_SEARCH,
		commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
		commonv1.ParticipantId_PARTICIPANT_ID_BOT,
		commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION,
	}
	for _, participantID := range purgeOrder {
		if c.dependencies.PurgeParticipants[participantID] == nil {
			return nil, fmt.Errorf("%w: purge participant %s is unavailable", ErrCoordinatorNotConfigured, participantID)
		}
	}
	aggregate, err := c.dependencies.Store.LoadLifecycle(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	snapshot := aggregate.Snapshot()
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING {
		return nil, fmt.Errorf("purge barrier is not pending in phase %s", snapshot.Phase)
	}
	deadline := c.dependencies.RPCDeadline
	if deadline <= 0 {
		deadline = participantRPCDeadline
	}
	if snapshot.RoleReceipt == nil {
		request, err := aggregate.RoleRetirementRequest()
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(ctx, deadline)
		receipt, callErr := c.dependencies.RoleRetirement.RetireSpace(callCtx, request)
		cancel()
		if callErr != nil {
			return nil, fmt.Errorf("role space retirement: %w", callErr)
		}
		if receipt == nil {
			return nil, errors.New("role returned an empty retirement receipt")
		}
		aggregate, err = c.dependencies.Store.RecordLifecycleRoleRetirementReceipt(ctx, receipt)
		if err != nil {
			return nil, fmt.Errorf("persist Role retirement receipt: %w", err)
		}
		snapshot = aggregate.Snapshot()
	}
	producerCtx, producerCancel := context.WithTimeout(ctx, deadline)
	producerErr := c.dependencies.FileProducer.ReleaseSpaceFileProducer(producerCtx, snapshot)
	producerCancel()
	if producerErr != nil {
		return nil, fmt.Errorf("space File producer release: %w", producerErr)
	}
	for _, participantID := range purgeOrder {
		if snapshot.PurgeReceipts[participantID] != nil {
			continue
		}
		request, err := aggregate.PurgeRequest(participantID)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(ctx, deadline)
		receipt, callErr := c.dependencies.PurgeParticipants[participantID].PurgeSpace(callCtx, request)
		cancel()
		if callErr != nil {
			return nil, fmt.Errorf("participant %s space purge: %w", participantID, callErr)
		}
		if receipt == nil || receipt.GetParticipantId() != participantID {
			return nil, fmt.Errorf("participant %s returned a mismatched purge receipt", participantID)
		}
		aggregate, err = c.dependencies.Store.RecordLifecyclePurgeReceipt(ctx, receipt)
		if err != nil {
			return nil, fmt.Errorf("persist participant %s purge receipt: %w", participantID, err)
		}
		snapshot = aggregate.Snapshot()
	}
	return aggregate, nil
}
