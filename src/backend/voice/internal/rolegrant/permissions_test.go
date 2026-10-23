package rolegrant

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type permissionClient struct {
	rolev1.RoleServiceClient
	call func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)
}

func (c permissionClient) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return c.call(ctx, req)
}

func TestVoicePermissionDecisionsBindSubjectNodeAndReplaceMetadata(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	space, profile, room := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seen := map[string]bool{}
	want := ""
	client := permissionClient{call: func(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		require.Equal(t, space, req.GetSpaceId())
		require.Equal(t, profile, req.GetProfileId())
		require.Equal(t, room, req.GetVoiceRoomId())
		require.Nil(t, req.Chat)
		require.Equal(t, want, req.PermissionName)
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		id := md.Get("x-request-id")[0]
		require.False(t, seen[id])
		seen[id] = true
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		verified, err := principal.VerifyService(ctx, md.Get("authorization")[0][len("Bearer "):], principal.VerifyConfig{
			ExpectedIssuer: "voice", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName,
			ExpectedRequestID: id, ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
		})
		require.NoError(t, err)
		require.Equal(t, "service:voice", verified.Subject)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), callTimeout)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	checker, err := NewChecker(client, issuer)
	require.NoError(t, err)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer forged", "x-request-id", "forged", "x-voice-profile-id", uuid.NewString()))
	for _, test := range []struct {
		permission string
		call       func(context.Context, string, string, string) error
	}{
		{"VOICE_JOIN", checker.EnsureVoiceJoin}, {"VOICE_SPEAK", checker.EnsureVoiceSpeak},
		{"VOICE_SCREEN_SHARE", checker.EnsureScreenShare}, {"VOICE_MUTE_OTHERS", checker.EnsureMuteOthers},
		{"VOICE_MOVE_OTHERS", checker.EnsureVoiceMoveOthers},
	} {
		want = test.permission
		require.NoError(t, test.call(ctx, space, profile, room))
	}
	require.Len(t, seen, 5)
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	require.Equal(t, codes.Canceled, status.Code(checker.EnsureVoiceJoin(ctx, space, profile, room)))
	require.Equal(t, codes.InvalidArgument, status.Code(checker.EnsureVoiceJoin(context.Background(), space, profile, "")))
	checker.client = permissionClient{call: func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		return nil, nil
	}}
	require.Error(t, checker.EnsureVoiceJoin(context.Background(), space, profile, room))
}
