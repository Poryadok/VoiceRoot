package spacecore

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// LifecycleSnapshot is a typed, detached persistence view of the pure
// aggregate. Every protobuf value returned by Snapshot is cloned.
type LifecycleSnapshot struct {
	SpaceID             string
	DeletionOperationID string
	Phase               spacev1.SpaceDeletionPhase
	Generation          uint64
	Manifest            *commonv1.ManifestBinding
	ScheduledAt         time.Time
	PurgeAfter          time.Time
	PurgeDecidedAt      time.Time
	FenceReceipts       map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt
	PurgeReceipts       map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt
	RoleReceipt         *rolev1.RetireSpaceReceipt
	LocalPurgeCompleted bool
	ScheduleEvent       *LifecycleOutboxRecord
	RestoreEvent        *LifecycleOutboxRecord
	DeletedEvent        *LifecycleOutboxRecord
}

func (a *LifecycleAggregate) Snapshot() LifecycleSnapshot {
	if a == nil {
		return LifecycleSnapshot{}
	}
	snapshot := LifecycleSnapshot{
		SpaceID: a.spaceID, DeletionOperationID: a.deletionOperationID,
		Phase: a.phase, Generation: a.generation,
		ScheduledAt: a.scheduledAt, PurgeAfter: a.purgeAfter, PurgeDecidedAt: a.purgeDecidedAt,
		LocalPurgeCompleted: a.localPurgeCompleted,
	}
	if a.manifest != nil {
		snapshot.Manifest = proto.Clone(a.manifest).(*commonv1.ManifestBinding)
	}
	if a.roleReceipt != nil {
		snapshot.RoleReceipt = proto.Clone(a.roleReceipt).(*rolev1.RetireSpaceReceipt)
	}
	snapshot.FenceReceipts = cloneFenceReceipts(a.fenceReceipts)
	snapshot.PurgeReceipts = clonePurgeReceipts(a.purgeReceipts)
	snapshot.ScheduleEvent = cloneLifecycleEvent(a.scheduleEvent)
	snapshot.RestoreEvent = cloneLifecycleEvent(a.restoreEvent)
	snapshot.DeletedEvent = cloneLifecycleEvent(a.deletedEvent)
	return snapshot
}

// RestoreLifecycleAggregate hydrates a persisted snapshot and revalidates all
// deterministic request/receipt bindings before returning domain state.
func RestoreLifecycleAggregate(snapshot LifecycleSnapshot) (*LifecycleAggregate, error) {
	a, err := NewLifecycleAggregate(snapshot.SpaceID, snapshot.DeletionOperationID)
	if err != nil {
		return nil, err
	}
	if snapshot.Generation == 0 {
		return nil, errors.New("lifecycle snapshot generation must be positive")
	}
	if err := validatePersistedPhase(snapshot.Phase); err != nil {
		return nil, err
	}
	if snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING {
		if snapshot.Manifest != nil {
			return nil, errors.New("schedule-pending lifecycle snapshot must not have a manifest")
		}
	} else if err := validateManifest(snapshot.Manifest); err != nil {
		return nil, err
	}
	if snapshot.ScheduledAt.IsZero() != snapshot.PurgeAfter.IsZero() {
		return nil, errors.New("lifecycle schedule timestamps must be present together")
	}
	a.phase = snapshot.Phase
	a.generation = snapshot.Generation
	if snapshot.Manifest != nil {
		a.manifest = proto.Clone(snapshot.Manifest).(*commonv1.ManifestBinding)
	}
	a.scheduledAt = snapshot.ScheduledAt
	a.purgeAfter = snapshot.PurgeAfter
	a.purgeDecidedAt = snapshot.PurgeDecidedAt
	a.localPurgeCompleted = snapshot.LocalPurgeCompleted
	a.scheduleEvent = cloneLifecycleEvent(snapshot.ScheduleEvent)
	a.restoreEvent = cloneLifecycleEvent(snapshot.RestoreEvent)
	a.deletedEvent = cloneLifecycleEvent(snapshot.DeletedEvent)

	if expectedState, acceptsFence := a.expectedFenceState(); acceptsFence {
		a.expectedFenceHashes, err = a.fenceRequestHashes(a.generation, expectedState, a.manifest)
		if err != nil {
			return nil, err
		}
		a.fenceReceipts = make(map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt, len(snapshot.FenceReceipts))
		for participantID, receipt := range snapshot.FenceReceipts {
			if receipt == nil || receipt.GetParticipantId() != participantID {
				return nil, errors.New("persisted fence receipt participant key does not match")
			}
			if err := a.validateFenceReceipt(receipt, expectedState); err != nil {
				return nil, err
			}
			a.fenceReceipts[participantID] = proto.Clone(receipt).(*commonv1.SpaceLifecycleFenceReceipt)
		}
	}

	if snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING ||
		snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED {
		a.expectedPurgeHashes, a.expectedRoleHash, err = a.purgeRequestHashes()
		if err != nil {
			return nil, err
		}
		a.purgeReceipts = make(map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt, len(snapshot.PurgeReceipts))
		if snapshot.RoleReceipt != nil {
			if err := a.validateRoleRetirementReceipt(snapshot.RoleReceipt); err != nil {
				return nil, err
			}
			a.roleReceipt = proto.Clone(snapshot.RoleReceipt).(*rolev1.RetireSpaceReceipt)
		}
		for participantID, receipt := range snapshot.PurgeReceipts {
			if receipt == nil || receipt.GetParticipantId() != participantID {
				return nil, errors.New("persisted purge receipt participant key does not match")
			}
			if err := a.validatePurgeReceipt(receipt); err != nil {
				return nil, err
			}
			a.purgeReceipts[participantID] = proto.Clone(receipt).(*commonv1.SpacePurgeReceipt)
		}
	}

	for eventType, event := range map[string]*LifecycleOutboxRecord{
		"space.deletion_scheduled": a.scheduleEvent,
		"space.restored":           a.restoreEvent,
		"space.deleted":            a.deletedEvent,
	} {
		if event != nil {
			if err := a.validatePersistedEvent(eventType, *event); err != nil {
				return nil, err
			}
		}
	}
	return a, nil
}

// ProjectedOutboxRecord returns the deterministic identity of an event before
// its terminal barrier supplies occurred_at.
func (a *LifecycleAggregate) ProjectedOutboxRecord(eventType string) (LifecycleOutboxRecord, error) {
	if a == nil || a.generation == 0 {
		return LifecycleOutboxRecord{}, errors.New("lifecycle aggregate is not initialized")
	}
	switch eventType {
	case "space.deletion_scheduled", "space.restored", "space.deleted":
		return a.newOutboxRecord(eventType, time.Time{}), nil
	default:
		return LifecycleOutboxRecord{}, fmt.Errorf("unsupported lifecycle event type %q", eventType)
	}
}

func (a *LifecycleAggregate) validatePersistedEvent(eventType string, event LifecycleOutboxRecord) error {
	expected := LifecycleOutboxRecord{
		EventID:   uuid.NewSHA1(uuid.MustParse(a.deletionOperationID), []byte(fmt.Sprintf("%s\x00%s\x00%d", a.spaceID, eventType, event.Generation))).String(),
		EventType: eventType, SpaceID: a.spaceID, DeletionOperationID: a.deletionOperationID,
		Generation: event.Generation, OccurredAt: event.OccurredAt,
	}
	if event.EventID != expected.EventID || event.EventType != expected.EventType ||
		event.SpaceID != expected.SpaceID || event.DeletionOperationID != expected.DeletionOperationID ||
		event.Generation != expected.Generation || event.OccurredAt.IsZero() {
		return fmt.Errorf("persisted %s event binding is invalid", eventType)
	}
	return nil
}

func validatePersistedPhase(phase spacev1.SpaceDeletionPhase) error {
	switch phase {
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE:
		return nil
	default:
		return fmt.Errorf("invalid persisted lifecycle phase %s", phase)
	}
}

func cloneFenceReceipts(source map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt) map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt {
	cloned := make(map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt, len(source))
	for participantID, receipt := range source {
		cloned[participantID] = proto.Clone(receipt).(*commonv1.SpaceLifecycleFenceReceipt)
	}
	return cloned
}

func clonePurgeReceipts(source map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt) map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt {
	cloned := make(map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt, len(source))
	for participantID, receipt := range source {
		cloned[participantID] = proto.Clone(receipt).(*commonv1.SpacePurgeReceipt)
	}
	return cloned
}

func cloneLifecycleEvent(event *LifecycleOutboxRecord) *LifecycleOutboxRecord {
	if event == nil {
		return nil
	}
	cloned := *event
	return &cloned
}
