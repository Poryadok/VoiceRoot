package main

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/guestguard"
)

const boundaryAccount = "11111111-1111-1111-1111-111111111111"
const boundaryProfile = "22222222-2222-2222-2222-222222222222"
const boundaryTarget = "33333333-3333-3333-3333-333333333333"
const boundaryBlockedAccount = "44444444-4444-4444-4444-444444444444"

type boundaryPresenceServer struct {
	userv1.UnimplementedUserServiceServer
	metadata chan metadata.MD
}

func (s *boundaryPresenceServer) UpdatePresence(ctx context.Context, _ *userv1.UpdatePresenceRequest) (*userv1.UpdatePresenceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.metadata <- md.Copy()
	return &userv1.UpdatePresenceResponse{}, nil
}

func (s *boundaryPresenceServer) GetPresence(ctx context.Context, request *userv1.GetPresenceRequest) (*userv1.GetPresenceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.metadata <- md.Copy()
	accounts := md.Get(grpcMDVoiceUserID)
	if len(accounts) != 1 || (accounts[0] != boundaryAccount && accounts[0] != boundaryBlockedAccount) {
		return nil, status.Error(codes.PermissionDenied, "viewer account unavailable")
	}
	// User owns the Social-block result. Realtime must propagate this sparse
	// response instead of synthesizing the raw event's online status.
	presence := &userv1.PresenceStatus{ProfileId: request.GetProfileId()}
	if accounts[0] != boundaryBlockedAccount {
		presence.Status = "online"
	}
	return &userv1.GetPresenceResponse{PresenceStatus: presence}, nil
}

func boundaryPresenceClient(t *testing.T) (*grpc.ClientConn, *boundaryPresenceServer) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	fixture := &boundaryPresenceServer{metadata: make(chan metadata.MD, 8)}
	userv1.RegisterUserServiceServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///presence-boundary", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn, fixture
}

func boundaryAmbientContext() context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		grpcMDVoiceUserID, boundaryBlockedAccount,
		grpcMDVoiceProfileID, boundaryTarget,
		guestguard.HeaderAccountType, "regular",
		"authorization", "Bearer unrelated",
		grpcMDVoiceInternalCaller, "subscription",
		"x-voice-subscription-tier", "premium",
		"x-request-id", "trace-preserved",
	))
}

func requireBoundaryMetadata(t *testing.T, md metadata.MD, account, accountType string) {
	t.Helper()
	require.Equal(t, []string{account}, md.Get(grpcMDVoiceUserID))
	require.Equal(t, []string{boundaryProfile}, md.Get(grpcMDVoiceProfileID))
	for _, key := range []string{"authorization", grpcMDVoiceInternalCaller, "x-voice-subscription-tier"} {
		require.Empty(t, md.Get(key), key)
	}
	if accountType == "" {
		require.Empty(t, md.Get(guestguard.HeaderAccountType))
	} else {
		require.Equal(t, []string{accountType}, md.Get(guestguard.HeaderAccountType))
	}
	require.Equal(t, []string{"trace-preserved"}, md.Get("x-request-id"))
}

func TestPresenceUpdaterUsesExplicitIdentityWithoutAmbientPrivileges(t *testing.T) {
	conn, fixture := boundaryPresenceClient(t)
	ctx := boundaryAmbientContext()
	err := newGRPCPresenceUpdater(conn).UpdatePresence(ctx, boundaryAccount, boundaryProfile, "dnd", "focus")
	require.NoError(t, err)
	requireBoundaryMetadata(t, <-fixture.metadata, boundaryAccount, "")
	// Copy-on-write must leave the caller's context unchanged.
	md, _ := metadata.FromOutgoingContext(ctx)
	require.Equal(t, []string{boundaryBlockedAccount}, md.Get(grpcMDVoiceUserID))
	require.Equal(t, []string{"premium"}, md.Get("x-voice-subscription-tier"))
}

func TestPresenceViewerCarriesConnectionAccountAndHonorsUserBlockResult(t *testing.T) {
	conn, fixture := boundaryPresenceClient(t)
	for _, account := range []string{boundaryAccount, boundaryBlockedAccount} {
		t.Run(account, func(t *testing.T) {
			reg := &connReg{accountID: account, profileID: boundaryProfile, accountType: "guest"}
			payload, err := presenceFanoutPayload(boundaryAmbientContext(), newGRPCPresenceViewer(conn), boundaryTarget, "online", reg, "")
			require.NoError(t, err)
			requireBoundaryMetadata(t, <-fixture.metadata, account, "guest")
			var body map[string]any
			require.NoError(t, json.Unmarshal(payload, &body))
			require.Equal(t, boundaryTarget, body["profile_id"])
			if account == boundaryBlockedAccount {
				require.NotContains(t, body, "status")
				require.NotContains(t, body, "custom_status")
				require.NotContains(t, body, "last_seen")
			} else {
				require.Equal(t, "online", body["status"])
			}
		})
	}
}

func TestPresenceAdaptersRejectUnknownExplicitIdentity(t *testing.T) {
	conn, fixture := boundaryPresenceClient(t)
	err := newGRPCPresenceUpdater(conn).UpdatePresence(boundaryAmbientContext(), "", boundaryProfile, "online", "")
	require.Error(t, err)
	reg := &connReg{profileID: boundaryProfile, accountType: "guest"}
	_, err = presenceFanoutPayload(boundaryAmbientContext(), newGRPCPresenceViewer(conn), boundaryTarget, "online", reg, "")
	require.Error(t, err)
	select {
	case <-fixture.metadata:
		t.Fatal("unknown explicit identity must fail before contacting User")
	default:
	}
}
