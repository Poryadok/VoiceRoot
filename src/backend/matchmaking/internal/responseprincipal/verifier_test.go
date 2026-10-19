package responseprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

func responseFixture(t *testing.T) (Verifier, *principal.Issuer, time.Time) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	return Verifier{Clock: func() time.Time { return now }, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}, issuer, now
}
func responseInput(t *testing.T, req *matchmakingv1.RespondToMatchRequest, now time.Time) principal.DelegatedUserInput {
	t.Helper()
	h, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName, RequestID: req.OperationId, RequestHash: h, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)}
}
func incomingResponse(token, id string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
}

func TestResponsePrincipalExactDecisionAndFreshRetry(t *testing.T) {
	for _, accept := range []bool{false, true} {
		t.Run(map[bool]string{false: "decline", true: "accept"}[accept], func(t *testing.T) {
			v, issuer, now := responseFixture(t)
			req := &matchmakingv1.RespondToMatchRequest{MatchId: uuid.NewString(), Accept: accept, OperationId: uuid.NewString()}
			input := responseInput(t, req, now)
			seen := map[string]bool{}
			v.ReplayGuard = func(_ context.Context, iss, jti string, _ time.Time) error {
				require.Equal(t, "gateway", iss)
				if seen[jti] {
					return errors.New("used")
				}
				seen[jti] = true
				return nil
			}
			token, err := issuer.IssueDelegatedUser(input)
			require.NoError(t, err)
			ctx, err := v.Verify(incomingResponse(token, input.RequestID), req)
			require.NoError(t, err)
			s, ok := SubjectFromContext(ctx, req)
			require.True(t, ok)
			require.Equal(t, uuid.MustParse(input.AccountID), s.AccountID)
			require.Equal(t, uuid.MustParse(input.ProfileID), s.ProfileID)
			require.Equal(t, uuid.MustParse(req.OperationId), s.OperationID)
			require.EqualValues(t, 7, s.SessionEpoch)
			changed := &matchmakingv1.RespondToMatchRequest{MatchId: req.MatchId, Accept: !accept, OperationId: req.OperationId}
			_, ok = SubjectFromContext(ctx, changed)
			require.False(t, ok)
			_, ok = SubjectFromContext(principal.WithVerified(context.Background(), principal.Principal{AccountID: input.AccountID, ProfileID: input.ProfileID, SessionEpoch: 7}), req)
			require.False(t, ok)
			_, err = v.Verify(incomingResponse(token, input.RequestID), req)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			fresh, err := issuer.IssueDelegatedUser(input)
			require.NoError(t, err)
			require.NotEqual(t, token, fresh)
			_, err = v.Verify(incomingResponse(fresh, input.RequestID), req)
			require.NoError(t, err, "same semantic operation with fresh credential reaches durable replay check")
		})
	}
}

func TestResponsePrincipalRejectsWrongBindingAndDependencies(t *testing.T) {
	for _, kind := range []string{"method", "audience", "requesthash", "operation", "match", "unknown", "account", "profile", "service", "nokey", "noreplay", "noepoch", "revoked", "expired", "duplicate", "rawidentity"} {
		t.Run(kind, func(t *testing.T) {
			v, issuer, now := responseFixture(t)
			req := &matchmakingv1.RespondToMatchRequest{MatchId: uuid.NewString(), Accept: true, OperationId: uuid.NewString()}
			input := responseInput(t, req, now)
			switch kind {
			case "method":
				input.RPC = matchmakingv1.MatchmakingService_StartSearch_FullMethodName
			case "audience":
				input.Audience = "voice"
			case "requesthash":
				input.RequestHash, _ = principal.RequestHash(&matchmakingv1.RespondToMatchRequest{})
			case "operation":
				req.OperationId = uuid.NewString()
			case "match":
				req.MatchId = "invalid"
				input = responseInput(t, req, now)
			case "unknown":
				req.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
				input = responseInput(t, req, now)
			case "account":
				input.AccountID = uuid.Nil.String()
			case "profile":
				input.ProfileID = "20000000000040008000000000000001"
			case "nokey":
				v.KeyResolver = nil
			case "noreplay":
				v.ReplayGuard = nil
			case "noepoch":
				v.SessionEpochChecker = nil
			case "revoked":
				v.SessionEpochChecker = func(context.Context, string, int64) error { return errors.New("revoked") }
			case "expired":
				v.Clock = func() time.Time { return now.Add(2 * time.Minute) }
			}
			token, err := issuer.IssueDelegatedUser(input)
			require.NoError(t, err)
			if kind == "service" {
				token, err = issuer.IssueService(principal.ServiceInput{Audience: input.Audience, RPC: input.RPC, RequestID: input.RequestID, RequestHash: input.RequestHash})
				require.NoError(t, err)
			}
			ctx := incomingResponse(token, input.RequestID)
			if kind == "duplicate" {
				ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID, "x-request-id", input.RequestID))
			}
			if kind == "rawidentity" {
				ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID, "x-profile-id", input.ProfileID))
			}
			verified, err := v.Verify(ctx, req)
			require.Nil(t, verified)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}
