package intakeprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/pkg/principal"
)

func TestReportDelegationExactBinding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	request := &moderationv1.CreateReportRequest{TargetType: "story", TargetId: uuid.NewString(), Category: "spam"}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	input := principal.DelegatedUserInput{Audience: "moderation", RPC: moderationv1.ModerationService_CreateReport_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)}
	for _, tc := range []struct {
		name                                        string
		change                                      func(*principal.DelegatedUserInput)
		raw, duplicate, replay, revoked, dependency bool
		code                                        codes.Code
	}{
		{name: "valid", code: codes.OK},
		{name: "guest", change: func(i *principal.DelegatedUserInput) { i.AccountType = "guest" }, code: codes.OK},
		{name: "wrong_audience", change: func(i *principal.DelegatedUserInput) { i.Audience = "user" }, code: codes.Unauthenticated},
		{name: "wrong_RPC", change: func(i *principal.DelegatedUserInput) {
			i.RPC = moderationv1.ModerationService_SubmitAppeal_FullMethodName
		}, code: codes.Unauthenticated},
		{name: "wrong_body", change: func(i *principal.DelegatedUserInput) {
			i.RequestHash = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}, code: codes.Unauthenticated},
		{name: "wrong_request", change: func(i *principal.DelegatedUserInput) { i.RequestID = uuid.NewString() }, code: codes.Unauthenticated},
		{name: "raw_identity", raw: true, code: codes.Unauthenticated},
		{name: "duplicate_auth", duplicate: true, code: codes.Unauthenticated},
		{name: "replay", replay: true, code: codes.Unauthenticated},
		{name: "revoked_epoch", revoked: true, code: codes.Unauthenticated},
		{name: "epoch_outage", dependency: true, code: codes.Unavailable},
		{name: "noncanonical_profile", change: func(i *principal.DelegatedUserInput) { i.ProfileID = "bad" }, code: codes.Unauthenticated},
		{name: "missing_account_type", change: func(i *principal.DelegatedUserInput) { i.AccountType = "" }, code: codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input
			if tc.change != nil {
				tc.change(&in)
			}
			token, err := issuer.IssueDelegatedUser(in)
			require.NoError(t, err)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID)
			if tc.raw {
				md.Set("x-voice-profile-id", input.ProfileID)
			}
			if tc.duplicate {
				md.Append("authorization", "Bearer "+token)
			}
			verifier := Verifier{Keys: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
				require.Equal(t, "gateway", issuer)
				require.Equal(t, "current", kid)
				return &key.PublicKey, nil
			}, Replay: func(context.Context, string, string, time.Time) error {
				if tc.replay {
					return errors.New("replay")
				}
				return nil
			}, Epoch: func(_ context.Context, account string, epoch int64) error {
				require.Equal(t, input.AccountID, account)
				require.Equal(t, int64(7), epoch)
				if tc.dependency {
					return ErrDependencyUnavailable
				}
				if tc.revoked {
					return errors.New("revoked")
				}
				return nil
			}, Clock: func() time.Time { return now }}
			called := false
			_, err = verifier.Unary()(metadata.NewIncomingContext(context.Background(), md), request, &grpc.UnaryServerInfo{FullMethod: input.RPC}, func(ctx context.Context, _ any) (any, error) {
				called = true
				p, ok := principal.FromContext(ctx)
				require.True(t, ok)
				require.Equal(t, input.AccountID, p.AccountID)
				require.Equal(t, input.ProfileID, p.ProfileID)
				return nil, nil
			})
			require.Equal(t, tc.code, status.Code(err))
			require.Equal(t, tc.code == codes.OK, called)
		})
	}
}

func TestReportListenersAreDisjoint(t *testing.T) {
	handler := func(context.Context, any) (any, error) { t.Fatal("denied method reached handler"); return nil, nil }
	_, err := OrdinaryUnary()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: moderationv1.ModerationService_CreateReport_FullMethodName}, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = (Verifier{}).Unary()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: moderationv1.ModerationService_ListReports_FullMethodName}, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
