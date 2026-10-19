package main

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
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
)

type creatorFloorRPC func(context.Context, *authv1.GetSpaceSessionEpochFloorRequest) (*authv1.GetSpaceSessionEpochFloorResponse, error)

func (f creatorFloorRPC) GetSpaceSessionEpochFloor(ctx context.Context, r *authv1.GetSpaceSessionEpochFloorRequest, _ ...grpc.CallOption) (*authv1.GetSpaceSessionEpochFloorResponse, error) {
	return f(ctx, r)
}

type creatorReplayFunc func(context.Context, string, string, time.Time) error

func (f creatorReplayFunc) ClaimCreatorPrincipalReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	return f(ctx, issuer, jti, expiry)
}

func TestCreatorRuntimeIndependentOfRoleReaderAndMapsUnknownFloor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	for _, mode := range []string{"regular", "guest", "revoked", "unknown floor", "missing issuer", "missing Auth", "missing replay"} {
		t.Run(mode, func(t *testing.T) {
			request := &spacev1.CreateSpaceRequest{Name: "Public creator", Visibility: "public"}
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			input := principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)}
			if mode == "guest" {
				input.AccountType = "guest"
			}
			token, err := gateway.IssueDelegatedUser(input)
			require.NoError(t, err)
			floor := creatorFloorRPC(func(ctx context.Context, r *authv1.GetSpaceSessionEpochFloorRequest) (*authv1.GetSpaceSessionEpochFloorResponse, error) {
				require.Equal(t, input.AccountID, r.AccountId)
				md, _ := metadata.FromOutgoingContext(ctx)
				transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(ctx, md))
				require.NoError(t, err)
				hash, err := principal.RequestHash(r)
				require.NoError(t, err)
				_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_GetSpaceSessionEpochFloor_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
				require.NoError(t, err)
				if mode == "unknown floor" {
					return nil, errors.New("offline")
				}
				value := int64(7)
				if mode == "revoked" {
					value = 8
				}
				return &authv1.GetSpaceSessionEpochFloorResponse{SessionEpochFloor: value}, nil
			})
			seen := map[string]bool{}
			replay := creatorReplayFunc(func(_ context.Context, issuer, jti string, _ time.Time) error {
				require.Equal(t, "gateway", issuer)
				if seen[jti] {
					return errors.New("replayed")
				}
				seen[jti] = true
				return nil
			})
			issuer := spaceIssuer
			if mode == "missing issuer" {
				issuer = nil
			}
			var replays creatorReplayStore = replay
			if mode == "missing replay" {
				replays = nil
			}
			var authClient interface {
				GetSpaceSessionEpochFloor(context.Context, *authv1.GetSpaceSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetSpaceSessionEpochFloorResponse, error)
			} = floor
			if mode == "missing Auth" {
				authClient = nil
			}
			dependencies := creatorMembershipDependencies(replays, authClient, issuer)
			require.Nil(t, dependencies.AdmissionEpoch)
			require.Nil(t, dependencies.AdmissionReplay)
			verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, CreatorEpoch: dependencies.CreatorEpoch, CreatorReplay: dependencies.CreatorReplay}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID))
			called := false
			_, err = verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: input.RPC}, func(ctx context.Context, _ any) (any, error) {
				called = true
				proof, err := lifecycleprincipal.RequireCreator(ctx, request)
				if err != nil {
					return nil, err
				}
				_, err = proof.ActorFor(request)
				return nil, err
			})
			switch mode {
			case "regular":
				require.NoError(t, err)
				require.True(t, called)
			case "unknown floor":
				require.Equal(t, codes.Unavailable, status.Code(err))
				require.False(t, called)
			default:
				require.Error(t, err)
				require.False(t, called)
			}
		})
	}
}
