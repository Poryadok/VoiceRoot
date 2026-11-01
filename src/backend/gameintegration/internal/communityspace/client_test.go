package communityspace

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/principal"
)

type spaceBootstrapService struct {
	spacev1.UnimplementedSpaceServiceServer
	request         *spacev1.CreateCommunityBootstrapRequest
	recoveryRequest *spacev1.RecoverCommunityOwnerRequest
	rosterRequest   *spacev1.ApplyCommunityRosterRequest
	coMemberRequest *spacev1.AreCoMembersRequest
	coMembers       bool
	authorization   []string
}

func (s *spaceBootstrapService) AreCoMembers(ctx context.Context, request *spacev1.AreCoMembersRequest) (*spacev1.AreCoMembersResponse, error) {
	s.coMemberRequest = request
	md, _ := metadata.FromIncomingContext(ctx)
	s.authorization = md.Get("authorization")
	return &spacev1.AreCoMembersResponse{CoMembers: s.coMembers}, nil
}

func (s *spaceBootstrapService) CreateCommunityBootstrap(ctx context.Context, request *spacev1.CreateCommunityBootstrapRequest) (*spacev1.CreateCommunityBootstrapResponse, error) {
	s.request = request
	md, _ := metadata.FromIncomingContext(ctx)
	s.authorization = md.Get("authorization")
	return &spacev1.CreateCommunityBootstrapResponse{Space: &spacev1.Space{Id: uuid.NewString()}}, nil
}

func TestClientRechecksCoMembershipWithinExactSpaceScope(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	service := &spaceBootstrapService{coMembers: true}
	spacev1.RegisterSpaceServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: "gis-test", PrivateKey: key})
	require.NoError(t, err)
	client, err := New(listener.Addr().String(), "gis-test-token")
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	client.SetPrincipalIssuer(issuer)
	profileA, profileB, spaceID := uuid.New(), uuid.New(), uuid.New()
	allowed, err := client.AreCoMembers(context.Background(), profileA, profileB, []uuid.UUID{spaceID})
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, profileA.String(), service.coMemberRequest.GetProfileIdA())
	require.Equal(t, profileB.String(), service.coMemberRequest.GetProfileIdB())
	require.Equal(t, []string{spaceID.String()}, service.coMemberRequest.GetSpaceIds())
	require.Len(t, service.authorization, 1)
	require.True(t, strings.HasPrefix(service.authorization[0], "Bearer "))
	require.NotEqual(t, "Bearer gis-test-token", service.authorization[0])
}

func (s *spaceBootstrapService) RecoverCommunityOwner(ctx context.Context, request *spacev1.RecoverCommunityOwnerRequest) (*spacev1.RecoverCommunityOwnerResponse, error) {
	s.recoveryRequest = request
	md, _ := metadata.FromIncomingContext(ctx)
	s.authorization = md.Get("authorization")
	return &spacev1.RecoverCommunityOwnerResponse{SpaceId: request.GetSpaceId(), OwnerGeneration: request.GetExpectedGeneration() + 1}, nil
}

func (s *spaceBootstrapService) ApplyCommunityRoster(ctx context.Context, request *spacev1.ApplyCommunityRosterRequest) (*spacev1.ApplyCommunityRosterResponse, error) {
	s.rosterRequest = request
	md, _ := metadata.FromIncomingContext(ctx)
	s.authorization = md.Get("authorization")
	return &spacev1.ApplyCommunityRosterResponse{OperationId: request.GetOperationId(), SpaceId: request.GetSpaceId(),
		OwnerGeneration: request.GetOwnerGeneration(), SourceRevision: request.GetSourceRevision(),
		SnapshotSha256: request.GetSnapshotSha256(), ReceiptId: uuid.NewString(), MemberCount: int32(len(request.GetProfileIds())),
		RequestSha256: make([]byte, 32)}, nil
}

func TestClientSendsExactOperationToProtectedSpaceRPC(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	service := &spaceBootstrapService{}
	server := grpc.NewServer()
	spacev1.RegisterSpaceServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	client, err := New(listener.Addr().String(), "gis-test-token")
	require.NoError(t, err)
	defer client.Close()
	op := registry.CommunityBootstrapOperation{
		OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerProfileID: uuid.New(),
		OwnerAccountID: uuid.New(),
		CorporationKey: "corp-7", TemplateID: "guild-default",
	}
	spaceID, err := client.ProvisionCommunityBootstrap(context.Background(), op)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, spaceID)
	require.Equal(t, op.OperationID.String(), service.request.GetOperationId())
	require.Equal(t, op.ApplicationID.String(), service.request.GetApplicationId())
	require.Equal(t, op.EnvironmentID.String(), service.request.GetEnvironmentId())
	require.Equal(t, op.OwnerProfileID.String(), service.request.GetOwnerProfileId())
	require.Equal(t, op.OwnerAccountID.String(), service.request.GetOwnerAccountId())
	require.Equal(t, op.CorporationKey, service.request.GetCorporationKey())
	require.Equal(t, op.TemplateID, service.request.GetTemplateId())
	require.Equal(t, []string{"Bearer gis-test-token"}, service.authorization)
	spaceID = uuid.New()
	recoveryOp := registry.OwnerRecoveryOperation{OperationID: op.OperationID, ApplicationID: op.ApplicationID, EnvironmentID: op.EnvironmentID, CorporationKey: op.CorporationKey, TargetSpaceID: spaceID, PreviousOwnerAccountID: uuid.New(), PreviousOwnerProfileID: uuid.New(), ReplacementAccountID: uuid.New(), ReplacementProfileID: uuid.New(), ExpectedGeneration: 3, ReasonCode: "owner_lost", EvidenceSHA256: make([]byte, 32)}
	receipt, err := client.ProvisionCommunityOwnerRecovery(context.Background(), recoveryOp)
	require.NoError(t, err)
	require.Equal(t, OwnerRecoveryReceipt{SpaceID: spaceID, Generation: 4}, receipt)
	require.Equal(t, recoveryOp.OperationID.String(), service.recoveryRequest.GetOperationId())
	require.Equal(t, recoveryOp.PreviousOwnerAccountID.String(), service.recoveryRequest.GetPreviousOwnerAccountId())
	require.Equal(t, recoveryOp.ReplacementProfileID.String(), service.recoveryRequest.GetReplacementProfileId())
	require.Equal(t, int64(3), service.recoveryRequest.GetExpectedGeneration())
	require.Equal(t, recoveryOp.EvidenceSHA256, service.recoveryRequest.GetEvidenceSha256())
	projection := registry.CommunityRosterProjection{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(),
		CorporationKey: "corp-7", SpaceID: spaceID, OwnerGeneration: 4, SourceRevision: 2, SnapshotSHA256: make([]byte, 32),
		LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{uuid.New(), uuid.New()}}
	rosterReceipt, err := client.ProjectCommunityRoster(context.Background(), projection)
	require.NoError(t, err)
	require.Equal(t, projection.OperationID, rosterReceipt.OperationID)
	require.Equal(t, projection.SpaceID, rosterReceipt.SpaceID)
	require.NotEqual(t, uuid.Nil, rosterReceipt.ReceiptID)
	require.Equal(t, int32(2), rosterReceipt.MemberCount)
	require.Equal(t, projection.OperationID.String(), service.rosterRequest.GetOperationId())
	require.Equal(t, projection.ProfileIDs[1].String(), service.rosterRequest.GetProfileIds()[1])
	require.WithinDuration(t, projection.LeaseExpiresAt, service.rosterRequest.GetLeaseExpiresAt().AsTime(), time.Millisecond)
	require.Equal(t, []string{"Bearer gis-test-token"}, service.authorization)
}
