package lifecycleprincipal

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
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
)

func TestDelegatedLifecyclePrincipalBindsActorRequestAndSession(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	req := &spacev1.RestoreSpaceRequest{SpaceId: uuid.NewString(), OperationId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	input := principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_RestoreSpace_FullMethodName, RequestID: req.OperationId, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)}
	for _, tc := range []struct {
		name                 string
		change               func(*principal.DelegatedUserInput)
		raw, revoked, replay bool
		code                 codes.Code
	}{
		{"valid", nil, false, false, false, codes.OK},
		{"wrong rpc", func(i *principal.DelegatedUserInput) { i.RPC = spacev1.SpaceService_DeleteSpace_FullMethodName }, false, false, false, codes.Unauthenticated},
		{"wrong hash", func(i *principal.DelegatedUserInput) {
			i.RequestHash, err = principal.RequestHash(&spacev1.RestoreSpaceRequest{SpaceId: req.SpaceId, OperationId: uuid.NewString()})
			require.NoError(t, err)
		}, false, false, false, codes.Unauthenticated},
		{"wrong request", func(i *principal.DelegatedUserInput) { i.RequestID = uuid.NewString() }, false, false, false, codes.Unauthenticated},
		{"wrong audience", func(i *principal.DelegatedUserInput) { i.Audience = "role" }, false, false, false, codes.Unauthenticated},
		{"noncanonical actor", func(i *principal.DelegatedUserInput) { i.ProfileID = "bad" }, false, false, false, codes.Unauthenticated},
		{"raw headers", nil, true, false, false, codes.Unauthenticated},
		{"revoked", nil, false, true, false, codes.Unauthenticated},
		{"replay", nil, false, false, true, codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input
			if tc.change != nil {
				tc.change(&in)
			}
			token, err := issuer.IssueDelegatedUser(in)
			require.NoError(t, err)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId)
			if tc.raw {
				md.Set("x-voice-profile-id", input.ProfileID)
			}
			interceptor := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error {
				if tc.replay {
					return errors.New("replayed")
				}
				return nil
			}, Epoch: func(_ context.Context, account string, epoch int64) error {
				require.Equal(t, input.AccountID, account)
				require.Equal(t, int64(7), epoch)
				if tc.revoked {
					return errors.New("revoked")
				}
				return nil
			}, Clock: func() time.Time { return now }}
			called := false
			_, err = interceptor.Unary()(metadata.NewIncomingContext(context.Background(), md), req, &grpc.UnaryServerInfo{FullMethod: input.RPC}, func(ctx context.Context, _ any) (any, error) {
				called = true
				account, ok := authctx.AccountID(ctx)
				require.True(t, ok)
				require.Equal(t, input.AccountID, account.String())
				profile, ok := authctx.ProfileID(ctx)
				require.True(t, ok)
				require.Equal(t, input.ProfileID, profile.String())
				epoch, ok := authctx.SessionEpoch(ctx)
				require.True(t, ok)
				require.Equal(t, int64(7), epoch)
				return nil, nil
			})
			require.Equal(t, tc.code, status.Code(err))
			require.Equal(t, tc.code == codes.OK, called)
		})
	}
}

func TestLifecycleListenersDenyRawAndUnlistedMethodsBeforeHandler(t *testing.T) {
	for _, method := range []string{spacev1.SpaceService_DeleteSpace_FullMethodName, spacev1.SpaceService_RestoreSpace_FullMethodName, spacev1.SpaceService_GetSpaceDeletionCoordinatorStatus_FullMethodName, spacev1.SpaceService_UpdateMemberNickname_FullMethodName, spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName} {
		_, err := OrdinaryUnary()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
			t.Fatal("protected method reached ordinary handler")
			return nil, nil
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	_, err := (Verifier{}).Unary()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ListMySpaces_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("unlisted method reached handler"); return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCompositeOutcomeRequiresExplicitSignedAccountType(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	req := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: uuid.NewString(), OperationId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	for _, accountType := range []string{"regular", "guest", "", "sdk-account"} {
		t.Run("type="+accountType, func(t *testing.T) {
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName, RequestID: req.OperationId, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: accountType, ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, err)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId))
			v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }, Clock: func() time.Time { return now }}
			called := false
			_, err = v.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
				called = true
				p, ok := principal.FromContext(ctx)
				require.True(t, ok)
				require.Equal(t, accountType, p.AccountType)
				return nil, nil
			})
			allowed := accountType == "regular" || accountType == "guest"
			require.Equal(t, allowed, called)
			if allowed {
				require.NoError(t, err)
			} else {
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
		})
	}
}
