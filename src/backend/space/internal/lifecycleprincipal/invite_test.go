package lifecycleprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestCreateInviteTypedIngressAndOriginalBudget(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, mode := range []string{"valid", "raw", "changed", "ordinary", "late"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
			require.NoError(t, err)
			req := &spacev1.CreateInviteRequest{SpaceId: uuid.NewString()}
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			id := uuid.NewString()
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateInvite_FullMethodName, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), AccountType: "regular", SessionEpoch: 1, ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, err)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
			if mode == "raw" {
				md.Set("x-voice-profile-id", uuid.NewString())
			}
			if mode == "changed" {
				req.SpaceId = uuid.NewString()
			}
			v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }, Clock: func() time.Time { return now }}
			interceptor := v.Unary()
			if mode == "ordinary" {
				interceptor = OrdinaryUnary()
			}
			_, err = interceptor(metadata.NewIncomingContext(context.Background(), md), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateInvite_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
				call, e := RequireInviteCreate(ctx, req)
				if e != nil {
					return nil, e
				}
				if mode == "late" {
					now = now.Add(5 * time.Second)
				}
				_, e = call.ActorFor(req)
				return nil, e
			})
			if mode == "valid" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
