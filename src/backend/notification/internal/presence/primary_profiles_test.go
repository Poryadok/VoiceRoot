package presence

import (
	"context"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

type primaryClientStub struct {
	userv1.UserServiceClient
	response *userv1.ResolvePrimaryProfileIDsResponse
	err      error
	contexts []context.Context
	request  *userv1.ResolvePrimaryProfileIDsRequest
	wait     bool
}

func (f *primaryClientStub) ResolvePrimaryProfileIDs(ctx context.Context, req *userv1.ResolvePrimaryProfileIDsRequest, _ ...grpc.CallOption) (*userv1.ResolvePrimaryProfileIDsResponse, error) {
	f.contexts = append(f.contexts, ctx)
	f.request = req
	if f.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.response, f.err
}

func TestPrimaryProfilesHonorsCallerDeadlineAndRejectsPartialConfig(t *testing.T) {
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "notification", KeyID: "current", PrivateKey: testSigningKey(t)})
	require.NoError(t, err)
	reader := &PrimaryProfiles{client: &primaryClientStub{wait: true}, issuer: issuer}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = reader.PrimaryProfileForAccount(ctx, uuid.New())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	checker := &GRPCChecker{issuer: issuer}
	reader, err = checker.PrimaryProfilesFromEnv(func(string) (string, bool) { return "", false })
	require.NoError(t, err)
	require.Nil(t, reader)
	reader, err = checker.PrimaryProfilesFromEnv(func(name string) (string, bool) {
		if name == "USER_NOTIFICATION_PRIMARY_GRPC_ADDR" {
			return "user:9098", true
		}
		return "", false
	})
	require.Error(t, err)
	require.Nil(t, reader)
}

func TestPrimaryProfilesUsesFreshBoundNotificationIdentity(t *testing.T) {
	key := testSigningKey(t)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "notification", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, profile := uuid.New(), uuid.New()
	stub := &primaryClientStub{response: &userv1.ResolvePrimaryProfileIDsResponse{PrimaryProfileIds: map[string]string{account.String(): profile.String()}}}
	reader := &PrimaryProfiles{client: stub, issuer: issuer}
	ambient := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "auth", "x-voice-user-id", account.String()))
	for range 2 {
		got, err := reader.PrimaryProfileForAccount(ambient, account)
		require.NoError(t, err)
		require.Equal(t, profile, got)
	}
	ids := map[string]bool{}
	for _, ctx := range stub.contexts {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-voice-internal-caller"))
		require.Empty(t, md.Get("x-voice-user-id"))
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		id := md.Get("x-request-id")[0]
		require.False(t, ids[id])
		ids[id] = true
		hash, err := principal.RequestHash(stub.request)
		require.NoError(t, err)
		p, err := principal.VerifyService(context.Background(), strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "notification", ExpectedAudience: "user", ExpectedRPC: userv1.UserService_ResolvePrimaryProfileIDs_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
		require.Empty(t, p.AccountID)
		require.Empty(t, p.ProfileID)
		require.Equal(t, "service:notification", p.Subject)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 500*time.Millisecond)
	}
}

func TestPrimaryProfilesFailsClosedOnUnavailableOrWrongRecipient(t *testing.T) {
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "notification", KeyID: "current", PrivateKey: testSigningKey(t)})
	require.NoError(t, err)
	account := uuid.New()
	for _, tc := range []struct {
		name     string
		response *userv1.ResolvePrimaryProfileIDsResponse
		err      error
	}{
		{name: "nil"}, {name: "missing primary", response: &userv1.ResolvePrimaryProfileIDsResponse{}},
		{name: "malformed profile", response: &userv1.ResolvePrimaryProfileIDsResponse{PrimaryProfileIds: map[string]string{account.String(): "bad"}}},
		{name: "extra account", response: &userv1.ResolvePrimaryProfileIDsResponse{PrimaryProfileIds: map[string]string{account.String(): uuid.NewString(), uuid.NewString(): uuid.NewString()}}},
		{name: "wrong account", response: &userv1.ResolvePrimaryProfileIDsResponse{PrimaryProfileIds: map[string]string{uuid.NewString(): uuid.NewString()}}},
		{name: "RPC unavailable", err: errors.New("unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &PrimaryProfiles{client: &primaryClientStub{response: tc.response, err: tc.err}, issuer: issuer}
			got, err := r.PrimaryProfileForAccount(context.Background(), account)
			require.Error(t, err)
			require.Equal(t, uuid.Nil, got)
		})
	}
	_, err = (&PrimaryProfiles{}).PrimaryProfileForAccount(context.Background(), account)
	require.Error(t, err)
}
