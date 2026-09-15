package searchprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

func TestVerifier_BindsVerifiedSubjectToExactSearch(t *testing.T) {
	v, issuer, now := fixture(t)
	req := &matchmakingv1.StartSearchRequest{GameId: uuid.NewString(), Mode: "duo"}
	input := credentialInput(t, req, now)
	token, err := issuer.IssueDelegatedUser(input)
	require.NoError(t, err)
	ctx, err := v.Verify(incoming(token, "request-1"), req)
	require.NoError(t, err)
	subject, ok := SubjectFromContext(ctx, req)
	require.True(t, ok)
	require.Equal(t, uuid.MustParse(input.AccountID), subject.AccountID)
	require.Equal(t, uuid.MustParse(input.ProfileID), subject.ProfileID)
	require.Equal(t, int64(7), subject.SessionEpoch)
	_, ok = SubjectFromContext(ctx, &matchmakingv1.StartSearchRequest{GameId: req.GameId, Mode: "squad"})
	require.False(t, ok, "a verified subject must not be reused for altered request bytes")
	_, ok = SubjectFromContext(ctx, nil)
	require.False(t, ok)
	_, ok = SubjectFromContext(principal.WithVerified(context.Background(), principal.Principal{AccountID: input.AccountID, ProfileID: input.ProfileID, SessionEpoch: 7}), req)
	require.False(t, ok, "generic context values cannot bypass the search verifier")
}

func TestVerifier_RejectsCredentialBindingAndIdentity(t *testing.T) {
	v, issuer, now := fixture(t)
	req := &matchmakingv1.StartSearchRequest{GameId: uuid.NewString()}
	for _, tc := range []struct {
		name   string
		change func(*principal.DelegatedUserInput)
	}{
		{"audience", func(i *principal.DelegatedUserInput) { i.Audience = "voice" }},
		{"rpc", func(i *principal.DelegatedUserInput) {
			i.RPC = matchmakingv1.MatchmakingService_CancelSearch_FullMethodName
		}},
		{"request id", func(i *principal.DelegatedUserInput) { i.RequestID = "other" }},
		{"request body", func(i *principal.DelegatedUserInput) {
			i.RequestHash, _ = principal.RequestHash(&matchmakingv1.StartSearchRequest{Mode: "other"})
		}},
		{"account malformed", func(i *principal.DelegatedUserInput) { i.AccountID = "invalid" }},
		{"profile malformed", func(i *principal.DelegatedUserInput) { i.ProfileID = "invalid" }},
		{"account zero", func(i *principal.DelegatedUserInput) { i.AccountID = uuid.Nil.String() }},
		{"profile zero", func(i *principal.DelegatedUserInput) { i.ProfileID = uuid.Nil.String() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := credentialInput(t, req, now)
			tc.change(&input)
			token, err := issuer.IssueDelegatedUser(input)
			require.NoError(t, err)
			ctx, err := v.Verify(incoming(token, "request-1"), req)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Nil(t, ctx)
		})
	}
}

func TestVerifier_RejectsMetadataBeforeDependencyCalls(t *testing.T) {
	v, issuer, now := fixture(t)
	req := &matchmakingv1.StartSearchRequest{}
	token, err := issuer.IssueDelegatedUser(credentialInput(t, req, now))
	require.NoError(t, err)
	v.KeyResolver = func(context.Context, string, string) (*rsa.PublicKey, error) {
		t.Fatal("metadata rejection must precede key lookup")
		return nil, nil
	}
	for _, pairs := range [][]string{
		{}, {"authorization", "Bearer " + token},
		{"authorization", "Bearer " + token, "authorization", "Bearer " + token, "x-request-id", "request-1"},
		{"authorization", "Bearer " + token, "x-request-id", "request-1", "x-request-id", "request-1"},
		{"authorization", "Bearer " + token, "x-request-id", "request-1", "x-voice-profile-id", uuid.NewString()},
		{"authorization", "Bearer " + token, "x-request-id", "request-1", "x-profile-id", uuid.NewString()},
		{"authorization", "Basic " + token, "x-request-id", "request-1"},
	} {
		ctx, err := v.Verify(metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...)), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Nil(t, ctx)
	}
}

func TestVerifier_RequiresLiveEpochAndReplayDependencies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Verifier)
	}{
		{"no key resolver", func(v *Verifier) { v.KeyResolver = nil }},
		{"no epoch checker", func(v *Verifier) { v.SessionEpochChecker = nil }},
		{"no replay guard", func(v *Verifier) { v.ReplayGuard = nil }},
		{"revoked epoch", func(v *Verifier) {
			v.SessionEpochChecker = func(context.Context, string, int64) error { return errors.New("revoked") }
		}},
		{"epoch outage", func(v *Verifier) {
			v.SessionEpochChecker = func(context.Context, string, int64) error { return context.DeadlineExceeded }
		}},
		{"replay", func(v *Verifier) {
			v.ReplayGuard = func(context.Context, string, string, time.Time) error { return errors.New("replayed") }
		}},
		{"expired", func(v *Verifier) { clock := v.Clock; v.Clock = func() time.Time { return clock().Add(time.Minute) } }},
		{"unknown key", func(v *Verifier) {
			v.KeyResolver = func(context.Context, string, string) (*rsa.PublicKey, error) { return nil, errors.New("unknown kid") }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, issuer, now := fixture(t)
			req := &matchmakingv1.StartSearchRequest{}
			token, err := issuer.IssueDelegatedUser(credentialInput(t, req, now))
			require.NoError(t, err)
			tc.change(&v)
			ctx, err := v.Verify(incoming(token, "request-1"), req)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Nil(t, ctx)
		})
	}
}

func TestVerifier_RejectsServiceCredentialAndNilRequest(t *testing.T) {
	v, issuer, now := fixture(t)
	req := &matchmakingv1.StartSearchRequest{}
	input := credentialInput(t, req, now)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: input.Audience, RPC: input.RPC, RequestID: input.RequestID, RequestHash: input.RequestHash})
	require.NoError(t, err)
	ctx, err := v.Verify(incoming(token, input.RequestID), req)
	require.Nil(t, ctx)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	ctx, err = v.Verify(incoming(token, input.RequestID), nil)
	require.Nil(t, ctx)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestVerifier_ReplayedCredentialCannotCreateAnotherSubject(t *testing.T) {
	v, issuer, now := fixture(t)
	req := &matchmakingv1.StartSearchRequest{}
	input := credentialInput(t, req, now)
	token, err := issuer.IssueDelegatedUser(input)
	require.NoError(t, err)
	seen := make(map[string]bool)
	v.ReplayGuard = func(_ context.Context, issuer, id string, expires time.Time) error {
		require.Equal(t, "gateway", issuer)
		require.True(t, expires.After(now))
		if seen[id] {
			return errors.New("already used")
		}
		seen[id] = true
		return nil
	}
	v.SessionEpochChecker = func(_ context.Context, accountID string, epoch int64) error {
		require.Equal(t, input.AccountID, accountID)
		require.Equal(t, input.SessionEpoch, epoch)
		return nil
	}
	_, err = v.Verify(incoming(token, input.RequestID), req)
	require.NoError(t, err)
	ctx, err := v.Verify(incoming(token, input.RequestID), req)
	require.Nil(t, ctx)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func fixture(t *testing.T) (Verifier, *principal.Issuer, time.Time) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	return Verifier{
		Clock: func() time.Time { return now },
		KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			require.Equal(t, "gateway", issuer)
			require.Equal(t, "current", kid)
			return &key.PublicKey, nil
		},
		ReplayGuard: func(context.Context, string, string, time.Time) error { return nil },
		SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
			require.NotEmpty(t, account)
			require.Equal(t, int64(7), epoch)
			return nil
		},
	}, issuer, now
}

func credentialInput(t *testing.T, req *matchmakingv1.StartSearchRequest, now time.Time) principal.DelegatedUserInput {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_StartSearch_FullMethodName, RequestID: "request-1", RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)}
}

func incoming(token, requestID string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
}
