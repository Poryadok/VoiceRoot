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
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/creatorauthority"
)

func TestCreatorCapabilityRequiresExplicitSignedActorAndFinalFreshSession(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, name := range []string{"valid", "empty type", "guest", "sdk", "wrong hash", "wrong rpc", "wrong audience", "raw actor", "revoked initially", "revoked before write", "unknown before write", "expired before write"} {
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			clock := func() time.Time { return now }
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
			require.NoError(t, err)
			request := &spacev1.CreateSpaceRequest{Name: "Verified creator", Visibility: "private"}
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			input := principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)}
			switch name {
			case "empty type":
				input.AccountType = ""
			case "guest":
				input.AccountType = "guest"
			case "sdk":
				input.AccountType = "sdk-account"
			case "wrong hash":
				input.RequestHash, _ = principal.RequestHash(&spacev1.CreateSpaceRequest{Name: "Other"})
			case "wrong rpc":
				input.RPC = spacev1.SpaceService_DeleteSpace_FullMethodName
			case "wrong audience":
				input.Audience = "role"
			}
			token, err := issuer.IssueDelegatedUser(input)
			require.NoError(t, err)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID)
			if name == "raw actor" {
				md.Set("x-voice-profile-id", input.ProfileID)
			}
			ctx := metadata.NewIncomingContext(context.Background(), md)
			floorCalls := 0
			floor := int64(7)
			unavailable := false
			if name == "revoked initially" {
				floor = 8
			}
			seen := map[string]bool{}
			v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock,
				Replay: func(context.Context, string, string, time.Time) error {
					t.Fatal("creator must not use legacy Redis replay")
					return nil
				},
				Epoch: func(context.Context, string, int64) error {
					t.Fatal("creator must not use legacy Redis-only floor")
					return nil
				},
				CreatorReplay: func(_ context.Context, issuer, jti string, _ time.Time) error {
					require.Equal(t, "gateway", issuer)
					if seen[jti] {
						return errors.New("durable replay")
					}
					seen[jti] = true
					return nil
				},
				CreatorEpoch: func(_ context.Context, account string, epoch int64) error {
					floorCalls++
					require.Equal(t, input.AccountID, account)
					require.Equal(t, int64(7), epoch)
					if unavailable {
						return creatorauthority.ErrUnavailable
					}
					if epoch < floor {
						return creatorauthority.ErrSessionRevoked
					}
					return nil
				},
			}
			var saved context.Context
			wrote := false
			_, err = v.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateSpace_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
				saved = ctx
				switch name {
				case "revoked before write":
					floor = 8
				case "unknown before write":
					unavailable = true
				case "expired before write":
					now = now.Add(40 * time.Second)
				}
				creator, err := RequireCreator(ctx, request)
				if err != nil {
					return nil, err
				}
				actor, err := creator.ActorFor(request)
				require.NoError(t, err)
				require.Equal(t, input.AccountID, actor.AccountID)
				require.Equal(t, input.ProfileID, actor.ProfileID)
				changed := proto.Clone(request).(*spacev1.CreateSpaceRequest)
				changed.Name = "Changed"
				_, err = creator.ActorFor(changed)
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				_, err = RequireCreator(principal.WithVerified(context.Background(), actor), request)
				require.Equal(t, codes.Unauthenticated, status.Code(err), "generic principal is not a creator capability")
				wrote = true
				return nil, nil
			})
			expected := codes.Unauthenticated
			if name == "valid" {
				expected = codes.OK
			}
			if name == "unknown before write" {
				expected = codes.Unavailable
			}
			require.Equal(t, expected, status.Code(err))
			require.Equal(t, name == "valid", wrote)
			if name == "valid" {
				require.Equal(t, 2, floorCalls, "ingress and final pre-write are separate fresh observations")
				// A restarted verifier using the same durable guard still rejects the JTI.
				restarted := v
				_, err = restarted.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: input.RPC}, func(context.Context, any) (any, error) { t.Fatal("replay reached writer"); return nil, nil })
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				current, _ := principal.FromContext(saved)
				current.ProfileID = uuid.NewString()
				_, err = RequireCreator(principal.WithVerified(saved, current), request)
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
		})
	}
	request := &spacev1.CreateSpaceRequest{Name: "Raw"}
	_, err = RequireCreator(context.Background(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = OrdinaryUnary()(context.Background(), request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateSpace_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("ordinary creator reached handler"); return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
