package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

type inviteDecisionFunc struct {
	rolev1.RoleServiceClient
	check func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)
}

func (c inviteDecisionFunc) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return c.check(ctx, r)
}

func TestCreateInviteDecisionPrecedesGateAndCannotRenewCallBudget(t *testing.T) {
	f := newInviteIngressFixture(t)
	now := time.Now()
	f.verifier.Clock = func() time.Time { return now }
	space, actor := uuid.New(), uuid.New()
	req := &spacev1.CreateInviteRequest{SpaceId: space.String()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := f.gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateInvite_FullMethodName, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: actor.String(), AccountType: "regular", SessionEpoch: 1, ClientExpiresAt: now.Add(time.Minute)})
	require.NoError(t, err)
	svc := &SpaceGRPC{Store: &store.SpaceStore{}, PrincipalIssuer: f.space}
	called := false
	svc.OwnershipRoles = inviteDecisionFunc{check: func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		called = true
		require.Equal(t, actor.String(), r.ProfileId)
		require.Equal(t, space.String(), r.SpaceId)
		require.Equal(t, "SPACE_MANAGE_INVITES", r.PermissionName)
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		release, err := svc.lockSpaceMutation(bounded, space)
		require.NoError(t, err, "Role must run before the Space gate")
		if err == nil {
			release()
		}
		now = now.Add(5 * time.Second)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	_, err = f.verifier.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateInvite_FullMethodName}, func(ctx context.Context, r any) (any, error) {
		return svc.CreateInvite(ctx, r.(*spacev1.CreateInviteRequest))
	})
	require.True(t, called)
	require.Equal(t, codes.Unavailable, status.Code(err), "expired call must fail before persistence")
}
