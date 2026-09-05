package s2s

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/correlation"

	userv1 "voice.app/voice/user/v1"
)

type recordingProfileOwnerUser struct {
	userv1.UnimplementedUserServiceServer
	accountID uuid.UUID
	err       error
	block     bool
	started   chan struct{}

	mu       sync.Mutex
	profile  string
	metadata metadata.MD
	deadline time.Time
}

func (s *recordingProfileOwnerUser) ResolveAccountIDForProfile(ctx context.Context, req *userv1.ResolveAccountIDForProfileRequest) (*userv1.ResolveAccountIDForProfileResponse, error) {
	s.mu.Lock()
	s.profile = req.GetProfileId()
	s.metadata, _ = metadata.FromIncomingContext(ctx)
	s.deadline, _ = ctx.Deadline()
	s.mu.Unlock()
	if s.started != nil {
		close(s.started)
	}
	if s.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if s.err != nil {
		return nil, s.err
	}
	return &userv1.ResolveAccountIDForProfileResponse{AccountId: s.accountID.String()}, nil
}

func (s *recordingProfileOwnerUser) observed() (string, metadata.MD, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.profile, s.metadata.Copy(), s.deadline
}

// TestUserGRPCProfiles_UsesDedicatedOwnerRPC proves that Messaging does not use
// public GetProfile visibility data for delete/block guards.  It also captures
// the actual metadata and context arriving at User through bufconn.
func TestUserGRPCProfiles_UsesDedicatedOwnerRPC(t *testing.T) {
	profileID, accountID := uuid.New(), uuid.New()
	stub := &recordingProfileOwnerUser{accountID: accountID}
	conn, cleanup := startBufconnUser(t, stub)
	t.Cleanup(cleanup)

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		"x-voice-user-id", "poisoned-account",
		"x-voice-profile-id", "poisoned-profile",
		"x-voice-internal-caller", "poisoned-caller",
		correlation.GRPCMetadataKey, "poisoned-request",
	))
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
		"x-voice-user-id", uuid.NewString(),
		"x-voice-profile-id", uuid.NewString(),
		correlation.GRPCMetadataKey, "request-owner-lookup",
	))
	before := time.Now()
	got, err := (&UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}).AccountIDByProfileID(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, accountID, got)

	requestProfile, md, deadline := stub.observed()
	require.Equal(t, profileID.String(), requestProfile)
	require.Equal(t, []string{"messaging"}, md.Get("x-voice-internal-caller"))
	require.Empty(t, md.Get("x-voice-user-id"))
	require.Empty(t, md.Get("x-voice-profile-id"))
	require.Equal(t, []string{"request-owner-lookup"}, md.Get(correlation.GRPCMetadataKey))
	require.True(t, deadline.After(before))
	require.Less(t, deadline.Sub(before), 5*time.Second)
}

func TestUserGRPCProfiles_OwnerRPCPreservesCancellationAndUnavailable(t *testing.T) {
	profileID := uuid.New()
	t.Run("cancellation", func(t *testing.T) {
		stub := &recordingProfileOwnerUser{block: true, started: make(chan struct{})}
		conn, cleanup := startBufconnUser(t, stub)
		t.Cleanup(cleanup)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := (&UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}).AccountIDByProfileID(ctx, profileID)
			done <- err
		}()
		select {
		case <-stub.started:
		case <-time.After(time.Second):
			t.Fatal("owner RPC did not reach User")
		}
		cancel()
		select {
		case err := <-done:
			require.Equal(t, codes.Canceled, status.Code(err))
		case <-time.After(time.Second):
			t.Fatal("owner RPC did not return after cancellation")
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		stub := &recordingProfileOwnerUser{err: status.Error(codes.Unavailable, "user unavailable")}
		conn, cleanup := startBufconnUser(t, stub)
		t.Cleanup(cleanup)
		_, err := (&UserGRPCProfiles{Client: userv1.NewUserServiceClient(conn)}).AccountIDByProfileID(context.Background(), profileID)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})
}
