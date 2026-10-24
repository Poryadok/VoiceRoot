package targetvalidation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/roleoverrideprincipal"
)

type chatFunc func(context.Context, *chatv1.ValidateRoleOverrideTargetRequest) (*chatv1.ValidateRoleOverrideTargetResponse, error)

func (f chatFunc) ValidateRoleOverrideTarget(ctx context.Context, req *chatv1.ValidateRoleOverrideTargetRequest, _ ...grpc.CallOption) (*chatv1.ValidateRoleOverrideTargetResponse, error) {
	return f(ctx, req)
}

type spaceFunc func(context.Context, *spacev1.ValidateRoleOverrideTargetRequest) (*spacev1.ValidateRoleOverrideTargetResponse, error)

func (f spaceFunc) ValidateRoleOverrideTarget(ctx context.Context, req *spacev1.ValidateRoleOverrideTargetRequest, _ ...grpc.CallOption) (*spacev1.ValidateRoleOverrideTargetResponse, error) {
	return f(ctx, req)
}

func TestClientSignsEachExactOwnerRequestWithoutUserMetadata(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	seenIDs, seenJTI := map[string]bool{}, map[string]bool{}
	calls := []string{}
	verify := func(ctx context.Context, target string, req proto.Message) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2, "user bearer/profile metadata must not propagate")
		incoming := metadata.NewIncomingContext(context.Background(), md)
		transport, err := principal.IncomingMetadata(incoming)
		require.NoError(t, err)
		_, err = roleoverrideprincipal.CanonicalID(transport.RequestID)
		require.NoError(t, err)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "role", ExpectedAudience: target, ExpectedRPC: roleoverrideprincipal.Method(target), ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
		require.False(t, seenIDs[p.RequestID])
		seenIDs[p.RequestID] = true
		require.False(t, seenJTI[p.JWTID])
		seenJTI[p.JWTID] = true
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
		calls = append(calls, target)
	}
	expectedSpace, expectedChat, expectedRoom := uuid.NewString(), uuid.NewString(), uuid.NewString()
	roomPresence := []bool{}
	c := &Client{Issuer: issuer,
		Chat: chatFunc(func(ctx context.Context, req *chatv1.ValidateRoleOverrideTargetRequest) (*chatv1.ValidateRoleOverrideTargetResponse, error) {
			verify(ctx, "chat", req)
			require.Equal(t, expectedSpace, req.SpaceId)
			require.Equal(t, expectedChat, req.ChatId)
			return &chatv1.ValidateRoleOverrideTargetResponse{}, nil
		}),
		Space: spaceFunc(func(ctx context.Context, req *spacev1.ValidateRoleOverrideTargetRequest) (*spacev1.ValidateRoleOverrideTargetResponse, error) {
			verify(ctx, "space", req)
			require.Equal(t, expectedSpace, req.SpaceId)
			roomPresence = append(roomPresence, req.VoiceRoomId != nil)
			if req.VoiceRoomId != nil {
				require.Equal(t, expectedRoom, *req.VoiceRoomId)
			}
			return &spacev1.ValidateRoleOverrideTargetResponse{}, nil
		}),
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer user-secret", "x-voice-profile-id", uuid.NewString(), "x-request-id", "inherited"))
	require.NoError(t, c.ValidateChat(ctx, expectedSpace, expectedChat))
	require.NoError(t, c.ValidateChat(ctx, expectedSpace, expectedChat))
	require.NoError(t, c.ValidateVoice(ctx, expectedSpace, expectedRoom))
	require.Equal(t, []string{"space", "chat", "space", "chat", "space"}, calls)
	require.Equal(t, []bool{false, false, true}, roomPresence)
}

func TestClientFailsClosedAndStopsAfterOwnerDenial(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	chatCalls := 0
	c := &Client{Issuer: issuer, Chat: chatFunc(func(context.Context, *chatv1.ValidateRoleOverrideTargetRequest) (*chatv1.ValidateRoleOverrideTargetResponse, error) {
		chatCalls++
		return nil, nil
	}), Space: spaceFunc(func(context.Context, *spacev1.ValidateRoleOverrideTargetRequest) (*spacev1.ValidateRoleOverrideTargetResponse, error) {
		return nil, status.Error(codes.NotFound, "target not found")
	})}
	require.Equal(t, codes.NotFound, status.Code(c.ValidateChat(context.Background(), uuid.NewString(), uuid.NewString())))
	require.Zero(t, chatCalls)
	c.Space = spaceFunc(func(context.Context, *spacev1.ValidateRoleOverrideTargetRequest) (*spacev1.ValidateRoleOverrideTargetResponse, error) {
		return nil, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(c.ValidateVoice(context.Background(), uuid.NewString(), uuid.NewString())))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.Equal(t, codes.Canceled, status.Code(c.ValidateChat(canceled, uuid.NewString(), uuid.NewString())))
	require.Zero(t, chatCalls)
	require.Equal(t, codes.Unavailable, status.Code((*Client)(nil).ValidateVoice(context.Background(), uuid.NewString(), uuid.NewString())))
}
