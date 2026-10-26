package s2s

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/messaging/internal/messageviews"
	"voice/backend/pkg/principal"
)

type deletionStateClient struct {
	userv1.UserServiceClient
	call func(context.Context, *userv1.GetProfileDeletionStatesRequest) (*userv1.GetProfileDeletionStatesResponse, error)
}

func (c deletionStateClient) GetProfileDeletionStates(ctx context.Context, req *userv1.GetProfileDeletionStatesRequest, _ ...grpc.CallOption) (*userv1.GetProfileDeletionStatesResponse, error) {
	return c.call(ctx, req)
}
func TestProfileDeletionStatesSignsExactRequestAndValidatesCompleteResponse(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	a, b := uuid.New(), uuid.New()
	seen := map[string]bool{}
	c := &UserGRPCProfileDeletion{Issuer: issuer}
	c.Client = deletionStateClient{call: func(ctx context.Context, req *userv1.GetProfileDeletionStatesRequest) (*userv1.GetProfileDeletionStatesResponse, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		id := md.Get("x-request-id")[0]
		require.False(t, seen[id])
		seen[id] = true
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		_, err = principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: "user", ExpectedRPC: userv1.UserService_GetProfileDeletionStates_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
		require.NoError(t, err)
		require.Equal(t, []string{a.String(), b.String()}, req.ProfileIds)
		return &userv1.GetProfileDeletionStatesResponse{States: []*userv1.ProfileDeletionStateEntry{{ProfileId: a.String(), State: userv1.ProfileDeletionState_PROFILE_DELETION_STATE_SOFT_DELETED}, {ProfileId: b.String(), State: userv1.ProfileDeletionState_PROFILE_DELETION_STATE_UNKNOWN}}}, nil
	}}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "end-user", "x-voice-profile-id", "spoof"))
	for i := 0; i < 2; i++ {
		states, err := c.ProfileDeletionStates(ctx, []uuid.UUID{a, b})
		require.NoError(t, err)
		require.Equal(t, messageviews.SoftDeleted, states[a])
		require.Equal(t, messageviews.Unknown, states[b])
	}
}
func TestProfileDeletionStatesRejectsMalformedResponseBeforeCompaction(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	a, b := uuid.New(), uuid.New()
	for _, entries := range [][]*userv1.ProfileDeletionStateEntry{nil, {{ProfileId: a.String(), State: 2}}, {{ProfileId: a.String(), State: 2}, {ProfileId: a.String(), State: 1}}, {{ProfileId: b.String(), State: 2}, {ProfileId: a.String(), State: 1}}, {{ProfileId: a.String(), State: 0}, {ProfileId: b.String(), State: 1}}, {{ProfileId: a.String(), State: 99}, {ProfileId: b.String(), State: 1}}} {
		c := &UserGRPCProfileDeletion{Issuer: issuer, Client: deletionStateClient{call: func(context.Context, *userv1.GetProfileDeletionStatesRequest) (*userv1.GetProfileDeletionStatesResponse, error) {
			return &userv1.GetProfileDeletionStatesResponse{States: entries}, nil
		}}}
		states, err := c.ProfileDeletionStates(context.Background(), []uuid.UUID{a, b})
		require.Error(t, err)
		require.Nil(t, states)
	}
}
