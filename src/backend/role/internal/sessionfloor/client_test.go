package sessionfloor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

type authCall func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error)

func (call authCall) GetRoleSessionEpochFloor(ctx context.Context, req *authv1.GetRoleSessionEpochFloorRequest, opts ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
	return call(ctx, req, opts...)
}

func roleFloorIssuer(t *testing.T) (*principal.Issuer, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return issuer, &key.PublicKey
}

func TestRoleFloorSignsExactAccountRPCAndNeverCachesPositiveAuthority(t *testing.T) {
	issuer, key := roleFloorIssuer(t)
	account := uuid.NewString()
	floor, calls := int64(5), 0
	seen := make(map[string]bool)
	client := New(authCall(func(ctx context.Context, req *authv1.GetRoleSessionEpochFloorRequest, _ ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(ctx, md))
		require.NoError(t, err)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
			ExpectedIssuer: "role", ExpectedAudience: "auth", ExpectedRPC: RPC,
			ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil },
		})
		require.NoError(t, err)
		require.Equal(t, "service:role", verified.Subject)
		require.False(t, seen[verified.JWTID], "each fresh read uses a new principal")
		seen[verified.JWTID] = true
		require.Equal(t, account, req.AccountId)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.Positive(t, time.Until(deadline))
		require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
		calls++
		return &authv1.GetRoleSessionEpochFloorResponse{SessionEpochFloor: floor}, nil
	}), issuer)
	actual, err := client.CurrentFloor(t.Context(), account)
	require.NoError(t, err)
	require.Equal(t, floor, actual)
	floor = 8
	require.Equal(t, codes.Unauthenticated, status.Code(client.RequireCurrent(t.Context(), account, 5)))
	require.NoError(t, client.RequireCurrent(t.Context(), account, 8))
	require.Equal(t, 3, calls)
}

func TestRoleFloorRejectsUnknownInvalidAndUnavailableAuthority(t *testing.T) {
	issuer, _ := roleFloorIssuer(t)
	account := uuid.NewString()
	for name, reply := range map[string]authCall{
		"missing account": func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
			return nil, status.Error(codes.NotFound, "private")
		},
		"dependency": func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
			return nil, status.Error(codes.Unavailable, "private")
		},
		"wrong credentials": func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "private")
		},
		"nil": func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
			return nil, nil
		},
		"zero": func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
			return &authv1.GetRoleSessionEpochFloorResponse{}, nil
		},
		"negative": func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
			return &authv1.GetRoleSessionEpochFloorResponse{SessionEpochFloor: -1}, nil
		},
		"unknown fields": func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
			r := &authv1.GetRoleSessionEpochFloorResponse{SessionEpochFloor: 5}
			r.ProtoReflect().SetUnknown([]byte{0x78, 1})
			return r, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			value, err := New(reply, issuer).CurrentFloor(t.Context(), account)
			want := codes.Unavailable
			if name == "missing account" {
				want = codes.Unauthenticated
			}
			require.Equal(t, want, status.Code(err))
			require.Zero(t, value)
			require.NotContains(t, err.Error(), "private")
		})
	}
	client := New(authCall(func(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
		t.Error("invalid input must not call Auth")
		return nil, nil
	}), issuer)
	for _, invalid := range []string{"", "not-a-uuid", uuid.Nil.String(), "{" + account + "}"} {
		_, err := client.CurrentFloor(t.Context(), invalid)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	require.Equal(t, codes.Unauthenticated, status.Code(client.RequireCurrent(t.Context(), account, 0)))
	for _, client := range []*Client{nil, New(nil, issuer), New(client.service, nil)} {
		_, err := client.CurrentFloor(t.Context(), account)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
}

func TestRoleFloorBoundsOutageAndHonorsEarlierCallerDeadline(t *testing.T) {
	issuer, _ := roleFloorIssuer(t)
	client := New(authCall(func(ctx context.Context, _ *authv1.GetRoleSessionEpochFloorRequest, _ ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}), issuer)
	client.deadline = 25 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
	defer cancel()
	_, err := client.CurrentFloor(ctx, uuid.NewString())
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}
