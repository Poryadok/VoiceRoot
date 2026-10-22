package lifecycleprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
)

func TestModerationOrdinaryListenerCannotBypassProtectedIngress(t *testing.T) {
	for _, method := range []string{spacev1.SpaceService_BanMember_FullMethodName, spacev1.SpaceService_UnbanMember_FullMethodName} {
		called := false
		_, err := OrdinaryUnary()(context.Background(), &spacev1.BanMemberRequest{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.False(t, called)
	}
}

func TestModerationIngressBindsExactRequestAndChecksFinalBudget(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, mode := range []string{"valid", "changed body", "wrong rpc", "raw actor", "revoked", "replay", "missing dependency", "owner phase exactly five", "after lock exactly five", "expired after lock", "body changed after verification"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
			require.NoError(t, err)
			request := &spacev1.BanMemberRequest{SpaceId: uuid.NewString(), AccountId: uuid.NewString()}
			// Accept the additive producer field without depending on a mutable
			// generated mirror while the separate intent producer is in review.
			intent := uuid.NewString()
			if field := request.ProtoReflect().Descriptor().Fields().ByName("operation_id"); field != nil {
				request.ProtoReflect().Set(field, protoreflect.ValueOfString(intent))
			}
			transportID := uuid.NewString()
			require.NotEqual(t, intent, transportID)
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			method := spacev1.SpaceService_BanMember_FullMethodName
			if mode == "wrong rpc" {
				method = spacev1.SpaceService_JoinSpace_FullMethodName
			}
			expiry := now.Add(time.Minute)
			if mode == "expired after lock" {
				expiry = now.Add(2 * time.Second)
			}
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: transportID, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: expiry})
			require.NoError(t, err)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", transportID)
			if mode == "changed body" {
				request.AccountId = uuid.NewString()
			}
			if mode == "raw actor" {
				md.Set(authctx.HeaderProfileID, uuid.NewString())
			}
			verifier := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: func() time.Time { return now }, Replay: func(context.Context, string, string, time.Time) error {
				if mode == "replay" {
					return errors.New("replay")
				}
				return nil
			}, Epoch: func(context.Context, string, int64) error {
				if mode == "revoked" {
					return errors.New("revoked")
				}
				if mode == "owner phase exactly five" {
					now = now.Add(5 * time.Second)
				}
				return nil
			}}
			if mode == "missing dependency" {
				verifier.Epoch = nil
			}
			called := false
			_, err = verifier.Unary()(metadata.NewIncomingContext(context.Background(), md), request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_BanMember_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
				called = true
				call, err := RequireModerationBan(ctx, request)
				if err != nil {
					return nil, err
				}
				if mode == "after lock exactly five" {
					now = now.Add(5 * time.Second)
				}
				if mode == "expired after lock" {
					now = now.Add(2 * time.Second)
				}
				if mode == "body changed after verification" {
					request.AccountId = uuid.NewString()
				}
				_, err = call.ActorFor(request)
				return nil, err
			})
			want := codes.Unauthenticated
			if mode == "valid" {
				want = codes.OK
			}
			if mode == "owner phase exactly five" || mode == "after lock exactly five" {
				want = codes.Unavailable
			}
			require.Equal(t, want, status.Code(err))
			if mode == "valid" {
				require.True(t, called)
			}
		})
	}
}
