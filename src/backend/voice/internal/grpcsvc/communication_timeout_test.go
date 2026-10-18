package grpcsvc

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
	voicestore "voice/backend/voice/internal/store"
)

func TestSpaceTimeoutAndUnknownCapabilityDenyBeforeRoleRosterAndToken(t *testing.T) {
	for _, tc := range []struct {
		name               string
		resolved, timedOut bool
		want               codes.Code
	}{{"timeout", true, true, codes.PermissionDenied}, {"legacy capability", false, false, codes.Unavailable}} {
		t.Run(tc.name, func(t *testing.T) {
			space, room, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
			events := &recordingEvents{}
			svc := newTestVoiceService(time.Now(), events)
			resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: space, Member: true, Active: true, AccessEpoch: 11, CommunicationStateResolved: tc.resolved, TimedOut: tc.timedOut}}
			configureReadySpaceMediaFixture(svc, resolver)
			roles := &canonicalRolePermissions{}
			svc.Roles = roles
			response, err := joinSpaceVoiceUser(t, svc, profile, &callsv1.JoinVoiceRoomRequest{VoiceRoomId: room, Space: &spacev1.SpaceRef{Id: space}})
			require.Nil(t, response)
			require.Equal(t, tc.want, status.Code(err))
			require.Empty(t, roles.joinChecks)
			require.Empty(t, events.startedCall)
			_, err = svc.Calls.GetCallByVoiceRoomID(t.Context(), room)
			require.ErrorIs(t, err, voicestore.ErrNotFound)
		})
	}
}
