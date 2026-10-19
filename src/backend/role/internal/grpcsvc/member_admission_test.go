package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/internal/store"
)

type failingMemberObserver struct {
	calls   int
	request *spacev1.GetMemberAdmissionAuthorityRequest
	err     error
}

func (f *failingMemberObserver) ObserveMember(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (memberauthority.Observation, error) {
	f.calls++
	f.request = req
	return memberauthority.Observation{}, f.err
}

func protectedAdmissionContext(t *testing.T, request proto.Message, method string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: method, RequestID: uuid.NewString(), RequestHash: hash})
}

func TestAdmissionReceiverAuthenticatesBeforeOwnerOrPersistence(t *testing.T) {
	ensure := &rolev1.EnsureDefaultMemberRoleRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), MembershipGeneration: 1, AdmissionOperationId: uuid.NewString(), AdmissionBindingSha256: make([]byte, 32)}
	observer := &failingMemberObserver{err: status.Error(codes.NotFound, "owner unavailable")}
	// This intentionally unconnected pool must never be reached on a failed
	// owner preflight. Positive database behavior is separately real-PG tested.
	server := &RoleGRPC{Store: &store.RoleStore{Pool: &pgxpool.Pool{}}, MemberAuthority: observer}
	_, err := server.EnsureDefaultMemberRole(context.Background(), ensure)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, observer.calls)
	ctx := protectedAdmissionContext(t, ensure, rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName)
	_, err = server.EnsureDefaultMemberRole(ctx, ensure)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, 1, observer.calls)
	require.Equal(t, ensure.MembershipGeneration, observer.request.GetExpectedGeneration())
	require.Equal(t, ensure.AdmissionOperationId, observer.request.GetExpectedAdmissionOperationId())
	require.Nil(t, observer.request.ChatId)
	p, _ := principal.FromContext(ctx)
	p.Issuer = "gateway"
	p.Subject = "service:gateway"
	_, err = server.EnsureDefaultMemberRole(principal.WithVerified(context.Background(), p), ensure)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 1, observer.calls)
	remove := &rolev1.RemoveMemberRolesRequest{ProtocolVersion: 1, SpaceId: ensure.SpaceId, ProfileId: ensure.ProfileId, RemovalGenerationFloor: 1, RemovalOperationId: uuid.NewString(), RemovalBindingSha256: make([]byte, 32)}
	_, err = server.RemoveMemberRoles(protectedAdmissionContext(t, remove, rolev1.RoleService_RemoveMemberRoles_FullMethodName), remove)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, 2, observer.calls)
	require.Nil(t, observer.request.ExpectedGeneration)
	require.Nil(t, observer.request.ExpectedAdmissionOperationId)
	server.MemberAuthority = nil
	_, err = server.RemoveMemberRoles(protectedAdmissionContext(t, remove, rolev1.RoleService_RemoveMemberRoles_FullMethodName), remove)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
