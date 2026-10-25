package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
)

func TestCommunicationStateRequiresVerifiedMessagingAndVersion(t *testing.T) {
	svc := &SpaceGRPC{}
	req := &spacev1.ResolveMemberCommunicationStateRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), ProfileId: uuid.NewString()}
	for _, identity := range []authctx.ServiceIdentity{"", authctx.ServiceIdentityVoice, authctx.ServiceIdentityGameIntegration} {
		_, err := svc.ResolveMemberCommunicationState(authctx.WithVerifiedServiceIdentity(context.Background(), identity), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err), "identity %q", identity)
	}
	ctx := authctx.WithVerifiedServiceIdentity(context.Background(), authctx.ServiceIdentity("messaging"))
	for _, version := range []uint32{0, 2} {
		req.ProtocolVersion = version
		_, err := svc.ResolveMemberCommunicationState(ctx, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	req.ProtocolVersion = 1
	_, err := svc.ResolveMemberCommunicationState(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err), "missing authority must not become CLEAR")
}
