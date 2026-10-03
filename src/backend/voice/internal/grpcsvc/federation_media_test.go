package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/federation/mediaauthority"
	"voice/backend/voice/internal/federationmedia"
)

type routedMediaFixture struct {
	requests []mediaauthority.RouteRequest
	err      error
}

func (f *routedMediaFixture) JoinToken(_ context.Context, request mediaauthority.RouteRequest) (mediaauthority.ExchangeResult, error) {
	f.requests = append(f.requests, request)
	return mediaauthority.ExchangeResult{JWT: "node-token", LivekitURL: "wss://node.test", ExpiresAt: time.Now().Add(20 * time.Second).UnixMilli()}, f.err
}

func TestGetJoinTokenUsesFederatedEdgeAfterCanonicalChecksAndNeverFallsBackOnFailure(t *testing.T) {
	account, profile, space, room := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	svc := newTestVoiceService(time.Now(), &recordingEvents{})
	resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: space, Active: true, Member: true}}
	svc.VoiceRoomAccessResolver = resolver
	svc.Roles = &canonicalRolePermissions{}
	joined, err := svc.JoinVoiceRoom(voiceTestCtx(profile), &callsv1.JoinVoiceRoomRequest{VoiceRoomId: room, Space: &spacev1.SpaceRef{Id: space}})
	require.NoError(t, err)
	edge := &routedMediaFixture{}
	hosted := &canonicalTokenIssuer{}
	svc.FederatedMedia = edge
	svc.Tokens = hosted
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-user-id", account, "x-voice-profile-id", profile, "x-voice-session-epoch", "5"))
	request := &callsv1.GetJoinTokenRequest{RoomId: joined.VoiceSession.RoomId}
	result, err := svc.GetJoinToken(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "wss://node.test", result.LivekitUrl)
	require.Zero(t, hosted.joinCalls)
	require.Len(t, edge.requests, 1)
	call, err := svc.Calls.GetCall(ctx, request.RoomId)
	require.NoError(t, err)
	require.Equal(t, mediaauthority.RouteRequest{AccountID: account, ProfileID: profile, SpaceID: space, ResourceID: room, RoomName: call.LivekitRoomName, SessionEpoch: 5, CanPublish: true}, edge.requests[0])
	for _, failure := range []error{federationmedia.ErrDenied, federationmedia.ErrUnavailable} {
		edge.err = failure
		_, err = svc.GetJoinToken(ctx, request)
		require.Error(t, err)
		require.Zero(t, hosted.joinCalls)
	}
	edge.err = federationmedia.ErrNotHosted
	_, err = svc.GetJoinToken(ctx, request)
	require.NoError(t, err)
	require.Equal(t, 1, hosted.joinCalls)
	edge.err = nil
	resolver.result.Member = false
	before := len(edge.requests)
	_, err = svc.GetJoinToken(ctx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Len(t, edge.requests, before)
	resolver.result.Member = true
	_, err = svc.GetJoinToken(voiceTestCtx(profile), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Len(t, edge.requests, before)
}
