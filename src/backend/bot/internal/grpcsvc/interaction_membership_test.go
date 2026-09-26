package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
)

type membershipProbe struct {
	chatv1.ChatServiceClient
	err     error
	profile string
}

func (p *membershipProbe) ListMembers(ctx context.Context, _ *chatv1.ListMembersRequest, _ ...grpc.CallOption) (*chatv1.ListMembersResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	p.profile = md.Get("x-voice-profile-id")[0]
	if len(md.Get("x-voice-internal")) != 0 {
		return nil, status.Error(codes.PermissionDenied, "internal bypass forwarded")
	}
	if p.err != nil {
		return nil, p.err
	}
	return &chatv1.ListMembersResponse{}, nil
}

func TestSlashMembershipFailsClosedAndForwardsOnlyInvoker(t *testing.T) {
	chatID, profileID := uuid.New(), uuid.New()
	svc := &BotGRPC{}
	require.Equal(t, codes.Unavailable, status.Code(svc.requireInvokerMembership(context.Background(), chatID, profileID)))
	probe := &membershipProbe{err: status.Error(codes.PermissionDenied, "removed member")}
	svc.Chat = probe
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal", "true"))
	require.Equal(t, codes.PermissionDenied, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	require.Equal(t, profileID.String(), probe.profile)
	probe.err = status.Error(codes.Unavailable, "chat down")
	require.Equal(t, codes.Unavailable, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	probe.err = nil
	require.NoError(t, svc.requireInvokerMembership(ctx, chatID, profileID))
}
