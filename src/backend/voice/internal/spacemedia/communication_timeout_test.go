package spacemedia

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/store"
)

func TestTimeoutInvalidationEjectsListenOnlyAndUnknownRemainsUnacked(t *testing.T) {
	for _, resolved := range []bool{true, false} {
		t.Run(map[bool]string{true: "explicit timeout", false: "unknown capability"}[resolved], func(t *testing.T) {
			ctx := t.Context()
			space, room, voiceRoom, profile, generation := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
			calls := newRoomStore(t, ctx, space, room, voiceRoom)
			identity := "profile:" + profile + ":media:" + generation
			grant := store.SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 1, PolicyEpoch: 1, CanJoin: true, CanSubscribe: true, CanPublishAudio: false}
			_, err := calls.AdmitSpaceMediaParticipant(ctx, room, store.SpaceMediaParticipant{AccountID: uuid.NewString(), AdmissionOperationID: uuid.NewString(), ProfileID: profile, Identity: identity, Generation: generation, Issued: grant, Reconciled: grant}, store.MaxSpaceProVoiceParticipants)
			require.NoError(t, err)
			media := &fakeMedia{}
			coordinator := &Coordinator{Store: calls, Admissions: fakeRecoveryForCommittedCall(t, ctx, calls, room), SessionEpochChecker: currentEpochChecker{}, Access: fakeAccess{byProfile: map[string]grpcsvc.CanonicalVoiceRoomAccess{profile: {SpaceID: space, Member: true, Active: true, AccessEpoch: 4, CommunicationStateResolved: resolved, TimedOut: resolved}}}, Grants: fakeGrants{byProfile: map[string]grpcsvc.CanonicalVoiceRoomGrants{profile: {PolicyEpoch: 7, CanJoin: true, CanSubscribe: true}}}, Media: media}
			envelope := &eventsv1.ChatStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.ChatStreamEvent_VoiceRoomAccessInvalidated{VoiceRoomAccessInvalidated: &eventsv1.VoiceRoomAccessInvalidated{SpaceId: space, AccessEpoch: 4}}}
			data, err := proto.Marshal(envelope)
			require.NoError(t, err)
			message := &invalidationAckMessage{}
			err = handleSpaceAccessInvalidation(ctx, spaceAccessSubject, data, message, coordinator)
			if !resolved {
				require.Error(t, err)
				require.Zero(t, message.acked)
				require.Equal(t, 1, message.nacked)
				require.Empty(t, media.removed)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, message.acked)
			require.Equal(t, []string{identity}, media.removed)
			current, err := calls.GetCall(ctx, room)
			require.NoError(t, err)
			require.Empty(t, current.SpaceMedia, "timeout must eject listeners rather than downgrade grants")
			require.NoError(t, handleSpaceAccessInvalidation(ctx, spaceAccessSubject, data, &invalidationAckMessage{}, coordinator))
			require.Equal(t, []string{identity}, media.removed, "duplicate hint cannot rejoin or eject a replacement by profile alone")
		})
	}
}
