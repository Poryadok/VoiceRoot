package spacecore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

const (
	testSpaceID           = "11111111-1111-4111-8111-111111111111"
	testOtherSpaceID      = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testOperationID       = "22222222-2222-4222-8222-222222222222"
	testManifestID        = "33333333-3333-4333-8333-333333333333"
	testManifestItemCount = 37
)

var (
	testManifestSHA256 = bytes.Repeat([]byte{0x5a}, 32)
	testParticipants   = []commonv1.ParticipantId{
		commonv1.ParticipantId_PARTICIPANT_ID_ROLE,
		commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		commonv1.ParticipantId_PARTICIPANT_ID_FILE,
		commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
		commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
		commonv1.ParticipantId_PARTICIPANT_ID_SEARCH,
		commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
		commonv1.ParticipantId_PARTICIPANT_ID_BOT,
		commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION,
	}
	testParticipantRequestPackages = map[commonv1.ParticipantId]string{
		commonv1.ParticipantId_PARTICIPANT_ID_ROLE:         "voice.role.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_CHAT:         "voice.chat.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING:    "voice.messaging.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_FILE:         "voice.file.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_VOICE:        "voice.calls.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING:  "voice.matchmaking.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_SEARCH:       "voice.search.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION: "voice.subscription.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_BOT:          "voice.bot.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION: "voice.notification.v1",
	}
)

func TestLifecycleScheduleRequiresEveryCanonicalParticipant(t *testing.T) {
	scheduledAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	for _, omitted := range testParticipants {
		omitted := omitted
		t.Run(omitted.String(), func(t *testing.T) {
			aggregate := newLifecycleAggregate(t)
			beginFreeze(t, aggregate, 1)
			for _, participantID := range testParticipants {
				if participantID == omitted {
					continue
				}
				require.NoError(t, aggregate.RecordFenceReceipt(fenceReceipt(participantID, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, 1)))
			}

			_, err := aggregate.CompleteSchedule(scheduledAt)
			require.Error(t, err)
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, aggregate.Phase())
		})
	}
}

func TestLifecycleRejectsZeroAndUnknownParticipantIDs(t *testing.T) {
	for _, participantID := range []commonv1.ParticipantId{
		commonv1.ParticipantId_PARTICIPANT_ID_UNSPECIFIED,
		commonv1.ParticipantId(11),
		commonv1.ParticipantId(65535),
	} {
		participantID := participantID
		t.Run(fmt.Sprintf("participant_%d", participantID), func(t *testing.T) {
			aggregate := newLifecycleAggregate(t)
			beginFreeze(t, aggregate, 1)

			err := aggregate.RecordFenceReceipt(fenceReceipt(participantID, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, 1))
			require.Error(t, err)
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, aggregate.Phase())
		})
	}
}

func TestLifecycleFenceReceiptsBindFullManifestAndExactParticipantRequest(t *testing.T) {
	seenHashes := make(map[string]string, len(testParticipants)*3)
	for _, state := range []commonv1.LifecycleFenceState{
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE,
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED,
	} {
		for _, participantID := range testParticipants {
			receipt := fenceReceiptForManifest(testSpaceID, participantID, state, 2, testManifestBinding())
			hashHex := hex.EncodeToString(receipt.GetRequestSha256())
			require.Len(t, receipt.GetRequestSha256(), sha256.Size)
			require.NotContains(t, seenHashes, hashHex, "request hash must bind participant wrapper FQN and phase")
			seenHashes[hashHex] = fmt.Sprintf("%s/%s", participantID, state)
		}
	}

	tests := []struct {
		name   string
		mutate func(*commonv1.SpaceLifecycleFenceReceipt)
	}{
		{
			name: "wrong request hash",
			mutate: func(receipt *commonv1.SpaceLifecycleFenceReceipt) {
				receipt.RequestSha256[0] ^= 0xff
			},
		},
		{
			name: "wrong manifest ID",
			mutate: func(receipt *commonv1.SpaceLifecycleFenceReceipt) {
				wrong := testManifestBinding()
				wrong.ManifestId = "44444444-4444-4444-8444-444444444444"
				receipt.RequestSha256 = fenceRequestSHA256(testSpaceID, receipt.GetParticipantId(), receipt.GetAppliedState(), receipt.GetGeneration(), wrong)
			},
		},
		{
			name: "wrong item count",
			mutate: func(receipt *commonv1.SpaceLifecycleFenceReceipt) {
				wrong := testManifestBinding()
				wrong.ItemCount++
				receipt.RequestSha256 = fenceRequestSHA256(testSpaceID, receipt.GetParticipantId(), receipt.GetAppliedState(), receipt.GetGeneration(), wrong)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aggregate := newLifecycleAggregate(t)
			beginFreeze(t, aggregate, 1)
			receipt := fenceReceipt(commonv1.ParticipantId_PARTICIPANT_ID_CHAT, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, 1)
			test.mutate(receipt)

			require.Error(t, aggregate.RecordFenceReceipt(receipt))
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, aggregate.Phase())
		})
	}
}

func TestLifecycleRestoreNeedsAllLiveReceiptsAndReturnsToLive(t *testing.T) {
	for _, omitted := range testParticipants {
		omitted := omitted
		t.Run(omitted.String(), func(t *testing.T) {
			aggregate := newLifecycleAggregate(t)
			scheduleEvent := completeSchedule(t, aggregate)
			restoreDecidedAt := aggregate.PurgeAfter().Add(-time.Nanosecond)

			phase, err := aggregate.DecideRecovery(restoreDecidedAt, 2)
			require.NoError(t, err)
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, phase)
			require.Equal(t, uint64(2), aggregate.Generation())
			for _, generation := range []uint64{0, 1} {
				require.Error(t, aggregate.RecordFenceReceipt(fenceReceipt(commonv1.ParticipantId_PARTICIPANT_ID_ROLE, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, generation)))
			}

			for _, participantID := range testParticipants {
				if participantID == omitted {
					continue
				}
				require.NoError(t, aggregate.RecordFenceReceipt(fenceReceipt(participantID, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, 2)))
			}
			_, err = aggregate.CompleteRestore(restoreDecidedAt.Add(time.Minute))
			require.Error(t, err)
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, aggregate.Phase())

			require.NoError(t, aggregate.RecordFenceReceipt(fenceReceipt(omitted, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, 2)))
			restoredAt := restoreDecidedAt.Add(time.Minute)
			first, err := aggregate.CompleteRestore(restoredAt)
			require.NoError(t, err)
			replay, err := aggregate.CompleteRestore(restoredAt.Add(time.Hour))
			require.NoError(t, err)
			require.Equal(t, first, replay)
			require.Equal(t, "space.restored", first.EventType)
			require.NotEmpty(t, first.EventID)
			require.Equal(t, restoredAt, first.OccurredAt)
			require.NotEqual(t, scheduleEvent.EventID, first.EventID)
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, aggregate.Phase())
		})
	}
}

func TestLifecycleGenerationIsPositiveAndMonotonic(t *testing.T) {
	aggregate := newLifecycleAggregate(t)
	require.Error(t, aggregate.BeginSchedule(0))
	require.Zero(t, aggregate.Generation())
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, aggregate.Phase())

	completeSchedule(t, aggregate)
	beforeDeadline := aggregate.PurgeAfter().Add(-time.Second)
	for _, generation := range []uint64{0, 1} {
		_, err := aggregate.DecideRecovery(beforeDeadline, generation)
		require.Error(t, err)
		require.Equal(t, uint64(1), aggregate.Generation())
		require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED, aggregate.Phase())
	}

	phase, err := aggregate.DecideRecovery(beforeDeadline, 2)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, phase)
	require.Equal(t, uint64(2), aggregate.Generation())

	_, err = aggregate.DecideRecovery(beforeDeadline, 1)
	require.Error(t, err)
	require.Equal(t, uint64(2), aggregate.Generation())
}

func TestLifecyclePurgeRequiresPurgingAndTypedRoleRetirement(t *testing.T) {
	for _, participantID := range testParticipants[1:] {
		participantID := participantID
		t.Run("before_role_"+participantID.String(), func(t *testing.T) {
			aggregate := newPurgingLifecycleAggregate(t)
			require.Error(t, aggregate.RecordPurgeReceipt(purgeReceipt(participantID, 2)), "typed Role retirement must precede every non-Role purge receipt")
		})
	}

	aggregate := newPurgingLifecycleAggregate(t)
	require.Error(t, aggregate.RecordPurgeReceipt(purgeReceipt(commonv1.ParticipantId_PARTICIPANT_ID_ROLE, 2)), "Role completion has authority only through role.v1.RetireSpaceReceipt")
	wrongRoleReceipt := roleRetirementReceipt(2)
	wrongRoleReceipt.RequestSha256[0] ^= 0xff
	require.Error(t, aggregate.RecordRoleRetirementReceipt(wrongRoleReceipt), "typed Role receipt must bind the exact deterministic retirement request")
	require.NoError(t, aggregate.RecordRoleRetirementReceipt(roleRetirementReceipt(2)))
	require.Error(t, aggregate.RecordLocalPurgeCompleted(), "local purge must wait for all ten validated remote receipts")

	for index, participantID := range testParticipants[1:] {
		wrongReceipt := purgeReceipt(participantID, 2)
		wrongReceipt.RequestSha256[0] ^= 0xff
		require.Error(t, aggregate.RecordPurgeReceipt(wrongReceipt), "participant purge receipt must bind the exact deterministic request")
		require.NoError(t, aggregate.RecordPurgeReceipt(purgeReceipt(participantID, 2)))
		if index != len(testParticipants[1:])-1 {
			require.Error(t, aggregate.RecordLocalPurgeCompleted(), "local purge must wait for every non-Role receipt")
		}
	}
	require.NoError(t, aggregate.RecordLocalPurgeCompleted())
	purgedAt := aggregate.PurgeAfter().Add(2 * time.Hour)
	first, err := aggregate.CompletePurge(purgedAt)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, aggregate.Phase())
	replay, err := aggregate.CompletePurge(purgedAt.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, first, replay)
	require.Equal(t, "space.deleted", first.EventType)
	require.NotEmpty(t, first.EventID)
	require.Equal(t, purgedAt, first.OccurredAt)

	for _, omitted := range testParticipants[1:] {
		omitted := omitted
		t.Run("missing_"+omitted.String(), func(t *testing.T) {
			aggregate := newPurgingLifecycleAggregate(t)
			require.NoError(t, aggregate.RecordRoleRetirementReceipt(roleRetirementReceipt(2)))
			for _, participantID := range testParticipants[1:] {
				if participantID == omitted {
					continue
				}
				require.NoError(t, aggregate.RecordPurgeReceipt(purgeReceipt(participantID, 2)))
			}
			require.Error(t, aggregate.RecordLocalPurgeCompleted(), "local purge must wait for every remote receipt")

			_, err := aggregate.CompletePurge(aggregate.PurgeAfter().Add(time.Hour))
			require.Error(t, err, "every non-Role participant receipt is mandatory for purge completion")
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING, aggregate.Phase())
		})
	}
}

func TestLifecycleScheduleOutboxIdentityAndTimeAreStableOnReplay(t *testing.T) {
	aggregate := newLifecycleAggregateForIDs(t, testSpaceID, testOperationID)
	beginFreeze(t, aggregate, 1)
	for _, participantID := range testParticipants {
		require.NoError(t, aggregate.RecordFenceReceipt(fenceReceipt(participantID, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, 1)))
	}

	scheduledAt := time.Date(2026, time.September, 2, 9, 30, 0, 0, time.UTC)
	first, err := aggregate.CompleteSchedule(scheduledAt)
	require.NoError(t, err)
	replay, err := aggregate.CompleteSchedule(scheduledAt.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, first, replay)
	require.Equal(t, "space.deletion_scheduled", first.EventType)
	require.NotEmpty(t, first.EventID)
	require.Equal(t, scheduledAt, first.OccurredAt)
	require.Equal(t, scheduledAt.Add(7*24*time.Hour), aggregate.PurgeAfter())

	otherAggregate := newLifecycleAggregateForIDs(t, testOtherSpaceID, testOperationID)
	beginFreeze(t, otherAggregate, 1)
	for _, participantID := range testParticipants {
		require.NoError(t, otherAggregate.RecordFenceReceipt(fenceReceiptForManifest(testOtherSpaceID, participantID, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, 1, testManifestBinding())))
	}
	other, err := otherAggregate.CompleteSchedule(scheduledAt)
	require.NoError(t, err)
	otherReplay, err := otherAggregate.CompleteSchedule(scheduledAt.Add(2 * time.Hour))
	require.NoError(t, err)
	require.Equal(t, other, otherReplay)
	require.NotEqual(t, first.EventID, other.EventID, "event identity must bind space_id even when operation_id, type and generation match")
}

func TestCanonicalLifecycleRetentionUsesOnlyTerminalPurgedAndDeliveredAnchors(t *testing.T) {
	policy := CanonicalLifecycleRetention()
	anchor := time.Date(2026, time.September, 3, 8, 0, 0, 0, time.UTC)
	boundary := anchor.Add(30 * 24 * time.Hour)

	tests := []struct {
		name string
		keep func(time.Time, *time.Time) bool
	}{
		{name: "terminal operation completed_at", keep: policy.KeepOperationEvidence},
		{name: "coordinator participant bytes purged_at", keep: policy.KeepCoordinatorParticipantEvidence},
		{name: "delivered outbox delivered_at", keep: policy.KeepDeliveredOutboxEvidence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.True(t, test.keep(boundary.Add(365*24*time.Hour), nil), "nonterminal or undelivered evidence has no 30-day clock")
			require.True(t, test.keep(boundary.Add(-time.Nanosecond), &anchor))
			require.False(t, test.keep(boundary, &anchor), "database-time equality expires bounded full evidence")
		})
	}
}

func TestCanonicalLifecycleRetentionKeepsTombstoneFor365DaysFromPurgedAt(t *testing.T) {
	policy := CanonicalLifecycleRetention()
	purgedAt := time.Date(2026, time.September, 3, 8, 0, 0, 0, time.UTC)
	boundary := purgedAt.Add(365 * 24 * time.Hour)

	require.False(t, policy.KeepTombstone(boundary.Add(-time.Nanosecond), spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING, &purgedAt), "a timestamp without terminal PURGED state is not a tombstone anchor")
	require.False(t, policy.KeepTombstone(boundary.Add(-time.Nanosecond), spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, nil), "PURGED without a persisted tombstone timestamp has no retention anchor")
	require.True(t, policy.KeepTombstone(boundary.Add(-time.Nanosecond), spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, &purgedAt))
	require.False(t, policy.KeepTombstone(boundary, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, &purgedAt), "database-time equality expires the no-legal-hold tombstone")
}

func TestLifecycleCoreExposesNoLegalHoldAuthority(t *testing.T) {
	aggregate := newLifecycleAggregate(t)
	assertNoLegalHoldSurface(t, reflect.TypeOf(aggregate))
	assertNoLegalHoldSurface(t, reflect.TypeOf(CanonicalLifecycleRetention()))
}

func newLifecycleAggregate(t *testing.T) *LifecycleAggregate {
	t.Helper()
	return newLifecycleAggregateForIDs(t, testSpaceID, testOperationID)
}

func newLifecycleAggregateForIDs(t *testing.T, spaceID, operationID string) *LifecycleAggregate {
	t.Helper()
	aggregate, err := NewLifecycleAggregate(spaceID, operationID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, aggregate.Phase())
	require.Zero(t, aggregate.Generation())
	return aggregate
}

func beginFreeze(t *testing.T, aggregate *LifecycleAggregate, generation uint64) {
	t.Helper()
	require.NoError(t, aggregate.BeginSchedule(generation))
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, aggregate.Phase())
	require.NoError(t, aggregate.BeginFreeze(testManifestBinding()))
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, aggregate.Phase())
}

func completeSchedule(t *testing.T, aggregate *LifecycleAggregate) LifecycleOutboxRecord {
	t.Helper()
	beginFreeze(t, aggregate, 1)
	for _, participantID := range testParticipants {
		require.NoError(t, aggregate.RecordFenceReceipt(fenceReceipt(participantID, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, 1)))
	}
	scheduledAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	event, err := aggregate.CompleteSchedule(scheduledAt)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED, aggregate.Phase())
	return event
}

func newPurgingLifecycleAggregate(t *testing.T) *LifecycleAggregate {
	t.Helper()
	aggregate := newLifecycleAggregate(t)
	completeSchedule(t, aggregate)
	phase, err := aggregate.DecideRecovery(aggregate.PurgeAfter(), 2)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED, phase)
	for _, participantID := range testParticipants {
		require.NoError(t, aggregate.RecordFenceReceipt(fenceReceipt(participantID, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, 2)))
	}
	require.NoError(t, aggregate.BeginPurging())
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING, aggregate.Phase())
	return aggregate
}

func fenceReceipt(participantID commonv1.ParticipantId, state commonv1.LifecycleFenceState, generation uint64) *commonv1.SpaceLifecycleFenceReceipt {
	return fenceReceiptForManifest(testSpaceID, participantID, state, generation, testManifestBinding())
}

func fenceReceiptForManifest(spaceID string, participantID commonv1.ParticipantId, state commonv1.LifecycleFenceState, generation uint64, manifest *commonv1.ManifestBinding) *commonv1.SpaceLifecycleFenceReceipt {
	return &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion:     1,
		ReceiptId:           fmt.Sprintf("fence-%d-%d-%d", participantID, state, generation),
		SpaceId:             spaceID,
		DeletionOperationId: testOperationID,
		Generation:          generation,
		ParticipantId:       participantID,
		AppliedState:        state,
		RequestSha256:       fenceRequestSHA256(spaceID, participantID, state, generation, manifest),
		ManifestSha256:      append([]byte(nil), manifest.GetManifestSha256()...),
		AppliedAt:           timestamppb.New(time.Date(2026, time.September, 1, 11, 0, 0, int(participantID), time.UTC)),
	}
}

func purgeReceipt(participantID commonv1.ParticipantId, generation uint64) *commonv1.SpacePurgeReceipt {
	return &commonv1.SpacePurgeReceipt{
		ProtocolVersion:     1,
		ReceiptId:           fmt.Sprintf("purge-%d-%d", participantID, generation),
		SpaceId:             testSpaceID,
		DeletionOperationId: testOperationID,
		Generation:          generation,
		ParticipantId:       participantID,
		State:               commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED,
		RequestSha256:       purgeRequestSHA256(participantID, generation),
		CompletedAt:         timestamppb.New(time.Date(2026, time.September, 9, 12, 0, 0, int(participantID), time.UTC)),
	}
}

func roleRetirementReceipt(generation uint64) *rolev1.RetireSpaceReceipt {
	return &rolev1.RetireSpaceReceipt{
		ProtocolVersion:     1,
		ReceiptId:           fmt.Sprintf("role-retirement-%d", generation),
		SpaceId:             testSpaceID,
		DeletionOperationId: testOperationID,
		Generation:          generation,
		State:               rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED,
		RequestSha256:       roleRetirementRequestSHA256(generation),
		ManifestSha256:      append([]byte(nil), testManifestSHA256...),
		RetiredAt:           timestamppb.New(time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)),
	}
}

func testManifestBinding() *commonv1.ManifestBinding {
	return &commonv1.ManifestBinding{
		ManifestId:     testManifestID,
		ManifestSha256: append([]byte(nil), testManifestSHA256...),
		ItemCount:      testManifestItemCount,
	}
}

func fenceRequestSHA256(spaceID string, participantID commonv1.ParticipantId, state commonv1.LifecycleFenceState, generation uint64, manifest *commonv1.ManifestBinding) []byte {
	request := &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID,
		DeletionOperationId: testOperationID,
		Generation:          generation,
		DesiredState:        state,
		Manifest:            proto.Clone(manifest).(*commonv1.ManifestBinding),
	}
	return wrappedRequestSHA256(testParticipantRequestPackages[participantID]+".ApplySpaceLifecycleFenceRequest", request)
}

func purgeRequestSHA256(participantID commonv1.ParticipantId, generation uint64) []byte {
	request := &commonv1.SpacePurgeRequest{
		ProtocolVersion:     1,
		SpaceId:             testSpaceID,
		DeletionOperationId: testOperationID,
		Generation:          generation,
		PurgeDecidedAt:      timestamppb.New(testPurgeDecidedAt()),
		ParticipantId:       participantID,
		Manifest:            testManifestBinding(),
	}
	return wrappedRequestSHA256(testParticipantRequestPackages[participantID]+".PurgeSpaceRequest", request)
}

func roleRetirementRequestSHA256(generation uint64) []byte {
	request := &rolev1.RetireSpaceRequest{
		ProtocolVersion:     1,
		SpaceId:             testSpaceID,
		DeletionOperationId: testOperationID,
		Generation:          generation,
		PurgeDecidedAt:      timestamppb.New(testPurgeDecidedAt()),
		Manifest:            testManifestBinding(),
	}
	return deterministicMessageSHA256(request)
}

func wrappedRequestSHA256(wrapperFQN string, request proto.Message) []byte {
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		panic(err)
	}
	wrapperBytes := protowire.AppendTag(nil, 1, protowire.BytesType)
	wrapperBytes = protowire.AppendBytes(wrapperBytes, requestBytes)
	return domainSeparatedSHA256(wrapperFQN, wrapperBytes)
}

func deterministicMessageSHA256(message proto.Message) []byte {
	messageBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		panic(err)
	}
	return domainSeparatedSHA256(string(message.ProtoReflect().Descriptor().FullName()), messageBytes)
}

func domainSeparatedSHA256(fqn string, deterministicWire []byte) []byte {
	payload := make([]byte, 0, len(fqn)+1+len(deterministicWire))
	payload = append(payload, fqn...)
	payload = append(payload, 0)
	payload = append(payload, deterministicWire...)
	hash := sha256.Sum256(payload)
	return append([]byte(nil), hash[:]...)
}

func testPurgeDecidedAt() time.Time {
	return time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
}

func assertNoLegalHoldSurface(t *testing.T, typ reflect.Type) {
	t.Helper()
	require.NotNil(t, typ)
	for methodIndex := 0; methodIndex < typ.NumMethod(); methodIndex++ {
		name := strings.ToLower(typ.Method(methodIndex).Name)
		require.NotContains(t, name, "legal")
		require.NotContains(t, name, "hold")
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		name := strings.ToLower(typ.Field(fieldIndex).Name)
		require.NotContains(t, name, "legal")
		require.NotContains(t, name, "hold")
	}
}
