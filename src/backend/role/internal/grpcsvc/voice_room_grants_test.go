package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

type recordingVoiceRoomGrantStore struct {
	decision                   store.VoiceRoomGrantDecision
	spaceID, roomID, profileID uuid.UUID
	calls                      int
}

func (s *recordingVoiceRoomGrantStore) ResolveVoiceRoomGrants(_ context.Context, spaceID, roomID, profileID uuid.UUID) (store.VoiceRoomGrantDecision, error) {
	s.calls++
	s.spaceID, s.roomID, s.profileID = spaceID, roomID, profileID
	return s.decision, nil
}

func TestResolveVoiceRoomGrantsRequiresExactVoicePrincipalAndUsesFencedDecision(t *testing.T) {
	request := &rolev1.ResolveVoiceRoomGrantsRequest{SpaceId: uuid.NewString(), VoiceRoomId: uuid.NewString(), ProfileId: uuid.NewString()}
	requestID := uuid.NewString()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	verified := principal.Principal{
		Kind: "service", Issuer: "voice", Subject: "service:voice", Audience: "role",
		RPC: rolev1.RoleService_ResolveVoiceRoomGrants_FullMethodName, RequestID: requestID, RequestHash: hash,
	}
	grantStore := &recordingVoiceRoomGrantStore{decision: store.VoiceRoomGrantDecision{
		CanJoin: true, CanPublishAudio: true, CanSubscribe: true, PolicyEpoch: 19,
	}}
	svc := &RoleGRPC{voiceRoomGrantStore: grantStore}
	ctx := principal.WithVerified(context.Background(), verified)
	response, err := svc.ResolveVoiceRoomGrants(ctx, request)
	require.NoError(t, err)
	require.Equal(t, uint64(19), response.GetPolicyEpoch())
	require.True(t, response.GetGrants().GetCanJoin())
	require.True(t, response.GetGrants().GetCanPublishAudio())
	require.Equal(t, uuid.MustParse(request.SpaceId), grantStore.spaceID)
	require.Equal(t, uuid.MustParse(request.VoiceRoomId), grantStore.roomID)
	require.Equal(t, uuid.MustParse(request.ProfileId), grantStore.profileID)
	require.Equal(t, 1, grantStore.calls)

	for _, mutate := range []func(*principal.Principal){
		func(p *principal.Principal) { p.Kind = "user" },
		func(p *principal.Principal) { p.Issuer = "gameintegration" },
		func(p *principal.Principal) { p.Subject = "service:gameintegration" },
		func(p *principal.Principal) { p.Audience = "voice" },
		func(p *principal.Principal) { p.RPC = rolev1.RoleService_CheckGameSessionGrant_FullMethodName },
		func(p *principal.Principal) { p.RequestID = "not-a-canonical-uuid" },
		func(p *principal.Principal) { p.RequestHash = "sha256:wrong" },
		func(p *principal.Principal) { p.ProfileID = uuid.NewString() },
		func(p *principal.Principal) { p.SessionEpoch = 1 },
	} {
		bad := verified
		mutate(&bad)
		_, err := svc.ResolveVoiceRoomGrants(principal.WithVerified(context.Background(), bad), request)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	require.Equal(t, 1, grantStore.calls, "invalid principal must not reach the fenced store")
}

func TestResolveVoiceRoomGrantsRejectsNonCanonicalIDsAndZeroEpochDecision(t *testing.T) {
	request := &rolev1.ResolveVoiceRoomGrantsRequest{SpaceId: uuid.NewString(), VoiceRoomId: uuid.NewString(), ProfileId: uuid.NewString()}
	grantStore := &recordingVoiceRoomGrantStore{}
	svc := &RoleGRPC{voiceRoomGrantStore: grantStore}
	principalFor := func(req *rolev1.ResolveVoiceRoomGrantsRequest, requestID string) context.Context {
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		return principal.WithVerified(context.Background(), principal.Principal{
			Kind: "service", Issuer: "voice", Subject: "service:voice", Audience: "role",
			RPC: rolev1.RoleService_ResolveVoiceRoomGrants_FullMethodName, RequestID: requestID, RequestHash: hash,
		})
	}
	bad := proto.Clone(request).(*rolev1.ResolveVoiceRoomGrantsRequest)
	bad.SpaceId = "ABCDEFAB-CDEF-ABCD-EFAB-CDEFABCDEFAB"
	_, err := svc.ResolveVoiceRoomGrants(principalFor(bad, uuid.NewString()), bad)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Zero(t, grantStore.calls)

	_, err = svc.ResolveVoiceRoomGrants(principalFor(request, uuid.NewString()), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "zero policy epoch must fail closed")
	require.Equal(t, 1, grantStore.calls)

	grantStore.decision.PolicyEpoch = 19
	response, err := svc.ResolveVoiceRoomGrants(principalFor(request, uuid.NewString()), request)
	require.NoError(t, err, "a complete all-false decision is a valid denial")
	require.Equal(t, uint64(19), response.GetPolicyEpoch())
	require.NotNil(t, response.GetGrants())
	require.False(t, response.GetGrants().GetCanJoin())
	require.False(t, response.GetGrants().GetCanPublishAudio())
	require.False(t, response.GetGrants().GetCanSubscribe())
	require.Equal(t, 2, grantStore.calls)
}
