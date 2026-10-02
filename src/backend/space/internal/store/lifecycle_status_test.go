package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

func TestLifecycleCoordinatorStatus_ProjectsPurgeReceiptsAndTerminalTime(t *testing.T) {
	spaceID, operationID := uuid.New(), uuid.New()
	purgeDecidedAt := time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC)
	purgedAt := purgeDecidedAt.Add(time.Minute)
	snapshot := spacecore.LifecycleSnapshot{
		SpaceID: spaceID.String(), DeletionOperationID: operationID.String(),
		Phase: spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, Generation: 2,
		Manifest: lifecycleManifestFixture(), ScheduledAt: purgeDecidedAt.Add(-8 * 24 * time.Hour),
		PurgeAfter: purgeDecidedAt.Add(-24 * time.Hour), PurgeDecidedAt: purgeDecidedAt,
		RoleReceipt:   lifecycleRoleRetirementReceipt(t, spaceID, operationID, 2, purgeDecidedAt),
		PurgeReceipts: make(map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt),
		DeletedEvent: &spacecore.LifecycleOutboxRecord{
			EventType: "space.deleted", SpaceID: spaceID.String(), DeletionOperationID: operationID.String(),
			Generation: 2, OccurredAt: purgedAt,
		},
	}
	for _, participantID := range lifecycleTestParticipants[1:] {
		snapshot.PurgeReceipts[participantID] = lifecyclePurgeReceipt(t, spaceID, operationID, participantID, 2, purgeDecidedAt)
	}

	status, err := lifecycleStatusFromSnapshot(snapshot)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, status.GetPhase())
	require.Equal(t, purgedAt, status.GetPurgedAt().AsTime())
	require.Len(t, status.GetParticipants(), len(lifecycleTestParticipants))
	for i, participant := range status.GetParticipants() {
		require.Equal(t, lifecycleTestParticipants[i], participant.GetParticipantId())
		require.Equal(t, spacev1.ParticipantOperationState_PARTICIPANT_OPERATION_STATE_COMPLETE, participant.GetState())
		require.Len(t, participant.GetRequestSha256(), 32)
		require.Len(t, participant.GetReceiptSha256(), 32)
		require.NotNil(t, participant.GetUpdatedAt())
	}
}
