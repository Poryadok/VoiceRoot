package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/privacy"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// callPrivacyChecker documents Phase 11 VoiceGRPC.Privacy wiring (privacy.md: allow_calls).
type callPrivacyChecker interface {
	AllowCallsAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

type callPrivacyStub struct {
	friendsOnly map[uuid.UUID]bool
}

func (s callPrivacyStub) AllowCallsAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.friendsOnly[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

type callNoFriendsStub struct{}

func (callNoFriendsStub) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (callNoFriendsStub) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// TestStartCall_FriendsOnlyPrivacy_StrangerDenied documents privacy.md: friends-only allow_calls blocks strangers at StartCall.
func TestStartCall_FriendsOnlyPrivacy_StrangerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_ = callPrivacyStub{} // wired when VoiceGRPC gains Privacy + Friends fields
	_ = callNoFriendsStub{}

	callerID := uuid.New().String()
	calleeID := uuid.New().String()
	calleeUUID := uuid.MustParse(calleeID)

	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)
	svc.Privacy = callPrivacyStub{friendsOnly: map[uuid.UUID]bool{calleeUUID: true}}
	svc.Friends = callNoFriendsStub{}

	_, err := svc.StartCall(voiceTestCtx(callerID), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: uuid.NewString()},
		CalleeProfileId: strPtr(calleeID),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, events.incoming)
}
