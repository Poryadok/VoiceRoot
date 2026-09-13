package s2s

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/privacy"

	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

type stubUserPrivacy struct {
	userv1.UnimplementedUserServiceServer
	lastMD   metadata.MD
	lastReq  *userv1.GetPrivacySettingsRequest
	response *userv1.PrivacySettings
	err      error
}

func (s *stubUserPrivacy) GetPrivacySettings(ctx context.Context, req *userv1.GetPrivacySettingsRequest) (*userv1.GetPrivacySettingsResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.lastMD = md.Copy()
	}
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	settings := s.response
	if settings == nil {
		settings = &userv1.PrivacySettings{}
	}
	return &userv1.GetPrivacySettingsResponse{
		PrivacySettings: settings,
	}, nil
}

type stubSpaceCoMembership struct {
	spacev1.UnimplementedSpaceServiceServer
	lastMD   metadata.MD
	lastReq  *spacev1.AreCoMembersRequest
	response bool
	err      error
}

func (s *stubSpaceCoMembership) AreCoMembers(ctx context.Context, req *spacev1.AreCoMembersRequest) (*spacev1.AreCoMembersResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.lastMD = md.Copy()
	}
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	return &spacev1.AreCoMembersResponse{CoMembers: s.response}, nil
}

func startBufconnUser(t *testing.T, impl userv1.UserServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func startBufconnSpace(t *testing.T, impl spacev1.SpaceServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	spacev1.RegisterSpaceServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func TestGRPCUserPrivacy_AudienceMappingAndInternalCaller(t *testing.T) {
	t.Parallel()
	target := uuid.New()
	friendAudience := privacy.Audience{Friends: true, SpaceMembers: true, SpaceIDs: []string{uuid.New().String()}}
	phoneAudience := privacy.Audience{FriendsOfFriends: true, IncludeGuests: true}
	stub := &stubUserPrivacy{response: &userv1.PrivacySettings{
		AllowFriendRequests: privacy.ToProto(friendAudience),
		AllowPhoneSearch:    privacy.ToProto(phoneAudience),
	}}
	conn, cleanup := startBufconnUser(t, stub)
	t.Cleanup(cleanup)

	client := &GRPCUserPrivacy{Client: userv1.NewUserServiceClient(conn)}

	// Incoming end-user MD must not be forwarded (would trip ownership checks).
	inCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-user-id", uuid.New().String(),
		"x-voice-profile-id", uuid.New().String(),
	))

	gotFriends, err := client.AllowFriendRequestsAudience(inCtx, target)
	require.NoError(t, err)
	require.Equal(t, friendAudience, gotFriends)
	require.Equal(t, target.String(), stub.lastReq.GetProfileId())
	require.Equal(t, []string{"social"}, stub.lastMD.Get("x-voice-internal-caller"))
	require.Empty(t, stub.lastMD.Get("x-voice-user-id"))
	require.Empty(t, stub.lastMD.Get("x-voice-profile-id"))

	gotPhone, err := client.AllowPhoneSearchAudience(context.Background(), target)
	require.NoError(t, err)
	require.Equal(t, phoneAudience, gotPhone)
	require.Equal(t, target.String(), stub.lastReq.GetProfileId())
}

func TestGRPCUserPrivacy_PropagatesGetPrivacySettingsError(t *testing.T) {
	t.Parallel()
	wantErr := status.Error(codes.Unavailable, "user unavailable")
	conn, cleanup := startBufconnUser(t, &stubUserPrivacy{err: wantErr})
	t.Cleanup(cleanup)

	got, err := (&GRPCUserPrivacy{Client: userv1.NewUserServiceClient(conn)}).AllowFriendRequestsAudience(context.Background(), uuid.New())
	require.Equal(t, privacy.Nobody(), got)
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, status.Convert(wantErr).Message(), status.Convert(err).Message())
}

func TestGRPCUserPrivacy_NilClientDegradesToEveryone(t *testing.T) {
	t.Parallel()
	target := uuid.New()

	for _, client := range []*GRPCUserPrivacy{nil, {}} {
		friendAudience, err := client.AllowFriendRequestsAudience(context.Background(), target)
		require.NoError(t, err)
		require.Equal(t, privacy.EveryoneWithGuests(), friendAudience)

		phoneAudience, err := client.AllowPhoneSearchAudience(context.Background(), target)
		require.NoError(t, err)
		require.Equal(t, privacy.EveryoneWithGuests(), phoneAudience)
	}
}

func TestGRPCSpaceCoMembership_RequestMetadataAndResponse(t *testing.T) {
	t.Parallel()
	profileA := uuid.New()
	profileB := uuid.New()
	spaceIDs := []string{uuid.New().String(), uuid.New().String()}
	stub := &stubSpaceCoMembership{response: true}
	conn, cleanup := startBufconnSpace(t, stub)
	t.Cleanup(cleanup)

	inCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-user-id", uuid.New().String(),
		"x-voice-profile-id", uuid.New().String(),
	))
	got, err := NewGRPCSpaceCoMembership(conn).AreCoMembers(inCtx, profileA, profileB, spaceIDs)
	require.NoError(t, err)
	require.True(t, got)
	require.Equal(t, profileA.String(), stub.lastReq.GetProfileIdA())
	require.Equal(t, profileB.String(), stub.lastReq.GetProfileIdB())
	require.Equal(t, spaceIDs, stub.lastReq.GetSpaceIds())
	require.Equal(t, []string{"social"}, stub.lastMD.Get("x-voice-internal-caller"))
	require.Empty(t, stub.lastMD.Get("x-voice-user-id"))
	require.Empty(t, stub.lastMD.Get("x-voice-profile-id"))
}

func TestGRPCSpaceCoMembership_PropagatesError(t *testing.T) {
	t.Parallel()
	wantErr := status.Error(codes.Unavailable, "space unavailable")
	conn, cleanup := startBufconnSpace(t, &stubSpaceCoMembership{err: wantErr})
	t.Cleanup(cleanup)

	got, err := NewGRPCSpaceCoMembership(conn).AreCoMembers(context.Background(), uuid.New(), uuid.New(), nil)
	require.False(t, got)
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, status.Convert(wantErr).Message(), status.Convert(err).Message())
}

func TestGRPCSpaceCoMembership_NilClientDegradesToNoCoMembership(t *testing.T) {
	t.Parallel()

	for _, client := range []*GRPCSpaceCoMembership{nil, {}} {
		got, err := client.AreCoMembers(context.Background(), uuid.New(), uuid.New(), []string{uuid.New().String()})
		require.NoError(t, err)
		require.False(t, got)
	}
	require.Nil(t, NewGRPCSpaceCoMembership(nil))
}
