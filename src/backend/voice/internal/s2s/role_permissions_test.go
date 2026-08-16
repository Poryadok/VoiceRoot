package s2s

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/grpcsvc"

	rolev1 "voice.app/voice/role/v1"
)

type recordingRoleClient struct {
	rolev1.RoleServiceClient
	lastReq *rolev1.CheckPermissionRequest
	allowed bool
}

func (r *recordingRoleClient) CheckPermission(_ context.Context, req *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	r.lastReq = req
	return &rolev1.CheckPermissionResponse{Allowed: r.allowed}, nil
}

func TestGRPCRolePermissions_EnsureVoiceJoin_passesVoiceRoomID(t *testing.T) {
	t.Parallel()
	spaceID := uuid.NewString()
	profileID := uuid.NewString()
	voiceRoomID := uuid.NewString()
	cli := &recordingRoleClient{allowed: true}
	g := NewGRPCRolePermissions(cli)

	err := g.EnsureVoiceJoin(context.Background(), spaceID, profileID, voiceRoomID)
	require.NoError(t, err)
	require.NotNil(t, cli.lastReq)
	require.Equal(t, permVoiceJoin, cli.lastReq.GetPermissionName())
	require.Equal(t, spaceID, cli.lastReq.GetSpaceId())
	require.Equal(t, profileID, cli.lastReq.GetProfileId())
	require.Equal(t, voiceRoomID, cli.lastReq.GetVoiceRoomId())
}

func TestGRPCRolePermissions_EnsureVoiceJoin_denied(t *testing.T) {
	t.Parallel()
	cli := &recordingRoleClient{allowed: false}
	g := NewGRPCRolePermissions(cli)

	err := g.EnsureVoiceJoin(context.Background(), uuid.NewString(), uuid.NewString(), uuid.NewString())
	require.ErrorIs(t, err, grpcsvc.ErrVoiceJoinDenied)
}

func TestGRPCRolePermissions_EnsureVoiceJoin_unavailable(t *testing.T) {
	t.Parallel()
	cli := &unavailableRoleClient{}
	g := NewGRPCRolePermissions(cli)

	err := g.EnsureVoiceJoin(context.Background(), uuid.NewString(), uuid.NewString(), uuid.NewString())
	require.Equal(t, codes.Unavailable, status.Code(err))
}

type unavailableRoleClient struct {
	rolev1.RoleServiceClient
}

func (unavailableRoleClient) CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return nil, status.Error(codes.Unavailable, "role service down")
}
