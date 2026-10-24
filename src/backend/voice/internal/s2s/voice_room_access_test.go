package s2s_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/s2s"
)

type communicationClient struct {
	spacev1.SpaceServiceClient
	response *spacev1.ResolveVoiceRoomAccessResponse
	err      error
	contexts []context.Context
	requests []*spacev1.ResolveVoiceRoomAccessRequest
}

func (c *communicationClient) ResolveVoiceRoomAccess(ctx context.Context, req *spacev1.ResolveVoiceRoomAccessRequest, _ ...grpc.CallOption) (*spacev1.ResolveVoiceRoomAccessResponse, error) {
	c.contexts = append(c.contexts, ctx)
	c.requests = append(c.requests, req)
	return c.response, c.err
}
func communicationIssuer(t *testing.T) (*principal.Issuer, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return issuer, &key.PublicKey
}
func validVoiceAccess(space, profile string) *spacev1.ResolveVoiceRoomAccessResponse {
	return &spacev1.ResolveVoiceRoomAccessResponse{SpaceId: space, ProfileId: profile, Member: true, Active: true, AccessEpoch: 1, ProtocolVersion: 1, EvaluatedAt: timestamppb.Now(), CommunicationState: spacev1.MemberCommunicationState_MEMBER_COMMUNICATION_STATE_CLEAR}
}

func TestGRPCVoiceRoomAccessResolverSignsFreshExactRequests(t *testing.T) {
	space, room, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	issuer, key := communicationIssuer(t)
	client := &communicationClient{response: validVoiceAccess(space, profile)}
	resolver := s2s.NewGRPCVoiceRoomAccessResolver(client, issuer)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer attacker", "x-voice-profile-id", uuid.NewString(), "x-voice-internal-caller", "voice"))
	var previous string
	for i := 0; i < 2; i++ {
		access, err := resolver.ResolveVoiceRoomAccess(ctx, space, room, profile)
		require.NoError(t, err)
		require.True(t, access.CommunicationStateResolved)
		md, _ := metadata.FromOutgoingContext(client.contexts[i])
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-internal-caller"))
		require.Len(t, md.Get("authorization"), 1)
		hash, err := principal.RequestHash(client.requests[i])
		require.NoError(t, err)
		verified, err := principal.VerifyService(t.Context(), strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "voice", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_ResolveVoiceRoomAccess_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
		require.NoError(t, err)
		require.NotEqual(t, previous, verified.JWTID)
		previous = verified.JWTID
		require.Equal(t, space, client.requests[i].GetSpace().GetId())
		require.Equal(t, room, client.requests[i].VoiceRoomId)
		require.Equal(t, profile, client.requests[i].ProfileId)
	}
}

func TestGRPCVoiceRoomAccessResolverRejectsLegacyAndMismatchedSnapshots(t *testing.T) {
	space, profile := uuid.NewString(), uuid.NewString()
	issuer, _ := communicationIssuer(t)
	for name, mutate := range map[string]func(*spacev1.ResolveVoiceRoomAccessResponse){"legacy": func(r *spacev1.ResolveVoiceRoomAccessResponse) { r.ProtocolVersion = 0 }, "space": func(r *spacev1.ResolveVoiceRoomAccessResponse) { r.SpaceId = uuid.NewString() }, "profile": func(r *spacev1.ResolveVoiceRoomAccessResponse) { r.ProfileId = uuid.NewString() }, "epoch": func(r *spacev1.ResolveVoiceRoomAccessResponse) { r.AccessEpoch = 0 }, "enum": func(r *spacev1.ResolveVoiceRoomAccessResponse) { r.CommunicationState = 0 }} {
		t.Run(name, func(t *testing.T) {
			response := validVoiceAccess(space, profile)
			mutate(response)
			_, err := s2s.NewGRPCVoiceRoomAccessResolver(&communicationClient{response: response}, issuer).ResolveVoiceRoomAccess(t.Context(), space, uuid.NewString(), profile)
			require.Equal(t, codes.Unavailable, status.Code(err))
		})
	}
	response := validVoiceAccess(space, profile)
	response.CommunicationState = spacev1.MemberCommunicationState_MEMBER_COMMUNICATION_STATE_TIMED_OUT
	response.TimedOutUntil = timestamppb.New(response.EvaluatedAt.AsTime().Add(time.Hour))
	access, err := s2s.NewGRPCVoiceRoomAccessResolver(&communicationClient{response: response}, issuer).ResolveVoiceRoomAccess(t.Context(), space, uuid.NewString(), profile)
	require.NoError(t, err)
	require.True(t, access.TimedOut)
}

func TestGRPCVoiceRoomAccessResolverMissingSignerAndDependencyFailClosed(t *testing.T) {
	issuer, _ := communicationIssuer(t)
	client := &communicationClient{err: status.Error(codes.Unavailable, "Space unavailable")}
	for _, signer := range []*principal.Issuer{nil, issuer} {
		_, err := s2s.NewGRPCVoiceRoomAccessResolver(client, signer).ResolveVoiceRoomAccess(t.Context(), uuid.NewString(), uuid.NewString(), uuid.NewString())
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	require.Len(t, client.contexts, 1, "missing signer fails before RPC")
}
