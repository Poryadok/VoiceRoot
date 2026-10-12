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
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	socialv1 "voice.app/voice/social/v1"
)

type recordingSocialProfileBlock struct {
	socialv1.UnimplementedSocialServiceServer
	viewer  string
	other   string
	others  []string
	blocked bool
	err     error
}

func (s *recordingSocialProfileBlock) IsProfilePairsBlocked(_ context.Context, req *socialv1.IsProfilePairsBlockedRequest) (*socialv1.IsProfilePairsBlockedResponse, error) {
	s.viewer = req.GetViewerProfileId()
	s.others = append([]string(nil), req.GetOtherProfileIds()...)
	if s.err != nil {
		return nil, s.err
	}
	results := make([]*socialv1.ProfilePairBlockResult, len(s.others))
	for i, profileID := range s.others {
		results[i] = &socialv1.ProfilePairBlockResult{OtherProfileId: profileID, Blocked: profileID == s.other}
	}
	return &socialv1.IsProfilePairsBlockedResponse{Results: results}, nil
}

func (s *recordingSocialProfileBlock) IsProfilePairBlocked(_ context.Context, req *socialv1.IsProfilePairBlockedRequest) (*socialv1.IsProfilePairBlockedResponse, error) {
	s.viewer = req.GetViewerProfileId()
	s.other = req.GetOtherProfileId()
	if s.err != nil {
		return nil, s.err
	}
	return &socialv1.IsProfilePairBlockedResponse{Blocked: s.blocked}, nil
}

func startProfileBlockBufconn(t *testing.T, impl socialv1.SocialServiceServer) grpc.ClientConnInterface {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestSocialGRPCProfileBlocks_ProfilePairBlockedIsDirectional(t *testing.T) {
	viewer, other := uuid.New(), uuid.New()
	server := &recordingSocialProfileBlock{blocked: true}
	checker := NewSocialGRPCProfileBlocks(socialv1.NewSocialServiceClient(startProfileBlockBufconn(t, server)))
	blocked, err := checker.ProfilePairBlocked(context.Background(), viewer, other)
	require.NoError(t, err)
	require.True(t, blocked)
	require.Equal(t, viewer.String(), server.viewer)
	require.Equal(t, other.String(), server.other)
}

func TestSocialGRPCProfileBlocks_FailsClosedWithoutSocial(t *testing.T) {
	viewer, other := uuid.New(), uuid.New()
	for _, checker := range []*SocialGRPCProfileBlocks{nil, {}} {
		blocked, err := checker.ProfilePairBlocked(context.Background(), viewer, other)
		require.False(t, blocked)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
}

func TestSocialGRPCProfileBlocks_BatchIsBoundedAndPreservesResults(t *testing.T) {
	viewer, blocked, neutral := uuid.New(), uuid.New(), uuid.New()
	server := &recordingSocialProfileBlock{other: blocked.String()}
	checker := NewSocialGRPCProfileBlocks(socialv1.NewSocialServiceClient(startProfileBlockBufconn(t, server)))
	results, err := checker.ProfilePairsBlocked(context.Background(), viewer, []uuid.UUID{blocked, neutral})
	require.NoError(t, err)
	require.Equal(t, []string{blocked.String(), neutral.String()}, server.others)
	require.Equal(t, map[uuid.UUID]bool{blocked: true, neutral: false}, results)
	_, err = checker.ProfilePairsBlocked(context.Background(), viewer, make([]uuid.UUID, 501))
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
