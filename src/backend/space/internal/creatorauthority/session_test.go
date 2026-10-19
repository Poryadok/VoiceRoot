package creatorauthority

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

type clientFunc func(context.Context, *authv1.GetSpaceSessionEpochFloorRequest) (*authv1.GetSpaceSessionEpochFloorResponse, error)

func (f clientFunc) GetSpaceSessionEpochFloor(ctx context.Context, r *authv1.GetSpaceSessionEpochFloorRequest, _ ...grpc.CallOption) (*authv1.GetSpaceSessionEpochFloorResponse, error) {
	return f(ctx, r)
}

func TestCreatorEpochObservationIsFreshExactSignedAndHasNoFallback(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account := uuid.NewString()
	floor := int64(2)
	unknown := false
	remoteErr := false
	seenIDs, seenJWTs := map[string]bool{}, map[string]bool{}
	calls := 0
	client := clientFunc(func(ctx context.Context, r *authv1.GetSpaceSessionEpochFloorRequest) (*authv1.GetSpaceSessionEpochFloorResponse, error) {
		calls++
		require.Equal(t, account, r.AccountId)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.Positive(t, time.Until(deadline))
		require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		id := md.Get("x-request-id")[0]
		require.False(t, seenIDs[id])
		seenIDs[id] = true
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_GetSpaceSessionEpochFloor_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seenJWTs[jti] {
				return errors.New("replay")
			}
			seenJWTs[jti] = true
			return nil
		}})
		require.NoError(t, err)
		require.Equal(t, "service:space", p.Subject)
		if remoteErr {
			return nil, context.DeadlineExceeded
		}
		response := &authv1.GetSpaceSessionEpochFloorResponse{SessionEpochFloor: floor}
		if unknown {
			response.ProtoReflect().SetUnknown([]byte{0x78, 1})
		}
		return response, nil
	})
	checker := &Checker{Client: client, Issuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "raw-client", "x-voice-profile-id", uuid.NewString()))
	require.NoError(t, checker.CheckEpoch(ctx, account, 2))
	floor = 3
	require.ErrorIs(t, checker.CheckEpoch(ctx, account, 2), ErrSessionRevoked, "a previously accepted session cannot bypass a fresh durable floor")
	floor = 2
	remoteErr = true
	require.ErrorIs(t, checker.CheckEpoch(ctx, account, 2), ErrUnavailable)
	remoteErr = false
	unknown = true
	require.ErrorIs(t, checker.CheckEpoch(ctx, account, 2), ErrUnavailable)
	unknown = false
	floor = 0
	require.ErrorIs(t, checker.CheckEpoch(ctx, account, 2), ErrUnavailable)
	require.Equal(t, 5, calls)
	for _, bad := range []string{"", uuid.Nil.String(), "{" + account + "}"} {
		require.ErrorIs(t, checker.CheckEpoch(ctx, bad, 2), ErrInvalidActor)
	}
	require.ErrorIs(t, checker.CheckEpoch(ctx, account, 0), ErrInvalidActor)
	require.Equal(t, 5, calls, "invalid actor must not dispatch")
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, checker.CheckEpoch(cancelled, account, 2), ErrUnavailable)
	require.Equal(t, 5, calls)
	require.ErrorIs(t, (*Checker)(nil).CheckEpoch(ctx, account, 2), ErrUnavailable)
	require.ErrorIs(t, (&Checker{Client: client}).CheckEpoch(ctx, account, 2), ErrUnavailable)
	require.ErrorIs(t, (&Checker{Issuer: issuer}).CheckEpoch(ctx, account, 2), ErrUnavailable)
}
