package grpcsvc

import (
	"context"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

func TestBotMembershipReceiverRequiresPrivateCapability(t *testing.T) {
	ordinary := &SpaceGRPC{}
	_, ordinaryErr := ordinary.AddBotMember(context.Background(), &spacev1.AddBotMemberRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(ordinaryErr))
	_, ordinaryErr = ordinary.RemoveBotMember(context.Background(), &spacev1.RemoveBotMemberRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(ordinaryErr))
	_, ordinaryErr = ordinary.ApplyBotInstallationRemoval(context.Background(), &spacev1.ApplyBotInstallationRemovalRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(ordinaryErr))
	s := &BotMembershipGRPC{}
	_, err := s.AddBotMember(context.Background(), &spacev1.AddBotMemberRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = s.RemoveBotMember(context.Background(), &spacev1.RemoveBotMemberRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = s.ApplyBotInstallationRemoval(context.Background(), &spacev1.ApplyBotInstallationRemovalRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, codes.FailedPrecondition, status.Code(mapBotMembershipError(store.ErrMemberAdmissionStale)))
	require.Equal(t, codes.Unavailable, status.Code(mapBotMembershipError(store.ErrMemberAuthorityUnavailable)))
	server := grpc.NewServer()
	defer server.Stop()
	RegisterBotMembershipServer(server, s)
	methods := server.GetServiceInfo()["voice.space.v1.SpaceService"].Methods
	require.Len(t, methods, 3)
	require.ElementsMatch(t, []string{"AddBotMember", "RemoveBotMember", "ApplyBotInstallationRemoval"}, []string{methods[0].Name, methods[1].Name, methods[2].Name})
}
