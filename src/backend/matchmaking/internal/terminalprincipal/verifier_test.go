package terminalprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

func terminalFixture(t *testing.T) (Verifier, *principal.Issuer, time.Time) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	return Verifier{Clock: func() time.Time { return now }, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}, issuer, now
}
func terminalToken(t *testing.T, i *principal.Issuer, now time.Time, req proto.Message, method string) (context.Context, principal.DelegatedUserInput) {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	input := principal.DelegatedUserInput{Audience: "matchmaking", RPC: method, RequestID: uuid.NewString(), RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)}
	token, err := i.IssueDelegatedUser(input)
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID)), input
}
func TestNativeTerminalPrincipalExactClosedMethodUnion(t *testing.T) {
	v, i, now := terminalFixture(t)
	for _, tc := range []struct {
		name, method string
		request      proto.Message
	}{
		{"match", matchmakingv1.MatchmakingService_GetMatch_FullMethodName, &matchmakingv1.GetMatchRequest{MatchId: uuid.NewString()}},
		{"status", matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName, &matchmakingv1.GetSearchStatusRequest{SessionId: uuid.NewString()}},
		{"history", matchmakingv1.MatchmakingService_GetMatchHistory_FullMethodName, &matchmakingv1.GetMatchHistoryRequest{}},
		{"cancel", matchmakingv1.MatchmakingService_CancelSearch_FullMethodName, &matchmakingv1.CancelSearchRequest{SessionId: uuid.NewString()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, input := terminalToken(t, i, now, tc.request, tc.method)
			verified, err := v.Verify(ctx, tc.request, tc.method)
			require.NoError(t, err)
			subject, ok := SubjectFromContext(verified, tc.request, tc.method)
			require.True(t, ok)
			require.Equal(t, uuid.MustParse(input.AccountID), subject.AccountID)
			require.Equal(t, uuid.MustParse(input.ProfileID), subject.ProfileID)
			require.EqualValues(t, 7, subject.SessionEpoch)
			_, ok = SubjectFromContext(principal.WithVerified(context.Background(), principal.Principal{AccountID: input.AccountID, ProfileID: input.ProfileID, SessionEpoch: 7}), tc.request, tc.method)
			require.False(t, ok)
			changed := proto.Clone(tc.request)
			changed.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
			_, ok = SubjectFromContext(verified, changed, tc.method)
			require.False(t, ok)
			_, ok = SubjectFromContext(verified, tc.request, matchmakingv1.MatchmakingService_StartSearch_FullMethodName)
			require.False(t, ok)
		})
	}
}
func TestNativeTerminalPrincipalNeverGrantsForwardOrRawAuthority(t *testing.T) {
	for _, kind := range []string{"raw", "forward", "swapped-type", "swapped-body", "unknown", "invalid-id", "nokey", "noreplay", "noepoch", "revoked", "replay", "expired"} {
		t.Run(kind, func(t *testing.T) {
			v, i, now := terminalFixture(t)
			var req proto.Message = &matchmakingv1.CancelSearchRequest{SessionId: uuid.NewString()}
			method := matchmakingv1.MatchmakingService_CancelSearch_FullMethodName
			ctx, _ := terminalToken(t, i, now, req, method)
			switch kind {
			case "raw":
				ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString()))
			case "forward":
				req = &matchmakingv1.StartSearchRequest{OperationId: uuid.NewString()}
				method = matchmakingv1.MatchmakingService_StartSearch_FullMethodName
				ctx, _ = terminalToken(t, i, now, req, method)
			case "swapped-type":
				req = &matchmakingv1.GetSearchStatusRequest{SessionId: req.(*matchmakingv1.CancelSearchRequest).SessionId}
			case "swapped-body":
				req = &matchmakingv1.CancelSearchRequest{SessionId: uuid.NewString()}
			case "unknown":
				req.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
				ctx, _ = terminalToken(t, i, now, req, method)
			case "invalid-id":
				req = &matchmakingv1.CancelSearchRequest{SessionId: "not-an-id"}
				ctx, _ = terminalToken(t, i, now, req, method)
			case "nokey":
				v.KeyResolver = nil
			case "noreplay":
				v.ReplayGuard = nil
			case "noepoch":
				v.SessionEpochChecker = nil
			case "revoked":
				v.SessionEpochChecker = func(context.Context, string, int64) error { return errors.New("revoked") }
			case "replay":
				v.ReplayGuard = func(context.Context, string, string, time.Time) error { return errors.New("replayed") }
			case "expired":
				v.Clock = func() time.Time { return now.Add(2 * time.Minute) }
			}
			verified, err := v.Verify(ctx, req, method)
			require.Error(t, err)
			require.Nil(t, verified)
		})
	}
}
