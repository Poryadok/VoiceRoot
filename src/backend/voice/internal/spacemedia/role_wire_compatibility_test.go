package spacemedia

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/s2s"
	"voice/backend/voice/internal/spacegatetest"
)

func TestRolePolicyWireOrdinaryArmsCannotAdvanceEpoch(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profileID)
	before, err := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, err)
	coordinator := &Coordinator{ProjectionGates: spacegatetest.New(), Store: calls}
	for _, subject := range []string{"role.created", "role.updated", "role.deleted", "role.assigned", "role.revoked", "role.chat_override_set", "role.chat_override_removed", "role.voice_override_set", "role.voice_override_removed"} {
		t.Run(subject, func(t *testing.T) {
			event := &eventsv1.RoleStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now()}
			mutation := &eventsv1.RoleMutation{SpaceId: spaceID, RoleId: uuid.NewString(), ProfileId: profileID, ChatId: uuid.NewString(), VoiceRoomId: voiceRoomID}
			field := event.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(strings.ReplaceAll(subject, ".", "_")))
			require.NotNil(t, field)
			event.ProtoReflect().Set(field, protoreflect.ValueOfMessage(mutation.ProtoReflect()))
			data, marshalErr := proto.Marshal(event)
			require.NoError(t, marshalErr)
			message := &invalidationAckMessage{}
			require.Error(t, handleRolePolicyInvalidation(ctx, rolePolicySubject, data, message, coordinator))
			require.Equal(t, 1, message.termed)
			require.Zero(t, message.acked)
			require.Zero(t, message.nacked)
			after, readErr := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
			require.NoError(t, readErr)
			require.Equal(t, before, after)
		})
	}
}

func TestRolePolicyWireRetryKeepsFullReconciliationAckBoundary(t *testing.T) {
	ctx := context.Background()
	spaceID, roomID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := newRoomStore(t, ctx, spaceID, roomID, voiceRoomID, profileID)
	media := &fakeMedia{err: errors.New("remove failed")}
	coordinator := &Coordinator{
		ProjectionGates: spacegatetest.New(), Store: calls, SessionEpochChecker: currentEpochChecker{},
		Admissions: fakeRecoveryForCommittedCall(t, ctx, calls, roomID),
		Access:     fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{profileID: {SpaceID: spaceID, Member: false, Active: true, AccessEpoch: 4, CommunicationStateResolved: true}}},
		Grants:     fakeGrants{byProfile: map[string]s2s.VoiceRoomGrants{}}, Media: media,
	}
	event := &eventsv1.RoleStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.RoleStreamEvent_VoiceRoomPolicyInvalidated{VoiceRoomPolicyInvalidated: &eventsv1.VoiceRoomPolicyInvalidated{SpaceId: spaceID, PolicyEpoch: 7}}}
	data, err := proto.Marshal(event)
	require.NoError(t, err)
	first := &invalidationAckMessage{}
	require.Error(t, handleRolePolicyInvalidation(ctx, rolePolicySubject, data, first, coordinator))
	require.Zero(t, first.acked)
	require.Equal(t, 1, first.nacked)
	require.Zero(t, first.termed)
	progress, err := calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, err)
	require.EqualValues(t, 7, progress.Observed.PolicyEpoch)
	require.Zero(t, progress.Reconciled.PolicyEpoch)
	retained, err := calls.GetCall(ctx, roomID)
	require.NoError(t, err)
	require.True(t, retained.SpaceMedia[profileID].Revoking)
	media.err = nil
	retry := &invalidationAckMessage{}
	require.NoError(t, handleRolePolicyInvalidation(ctx, rolePolicySubject, data, retry, coordinator))
	require.Equal(t, 1, retry.acked)
	require.Zero(t, retry.nacked)
	require.Zero(t, retry.termed)
	progress, err = calls.GetSpaceMediaEpochProgress(ctx, spaceID)
	require.NoError(t, err)
	require.EqualValues(t, 7, progress.Reconciled.PolicyEpoch)
	require.Equal(t, progress.Observed, progress.Reconciled)
}
