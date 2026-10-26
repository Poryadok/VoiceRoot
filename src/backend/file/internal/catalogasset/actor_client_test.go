package catalogasset

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	catalogactorv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
)

type actorAuthorityFunc func(context.Context, *catalogactorv1.GetCatalogActorStatesRequest) (*catalogactorv1.GetCatalogActorStatesResponse, error)

func (f actorAuthorityFunc) GetCatalogActorStates(ctx context.Context, r *catalogactorv1.GetCatalogActorStatesRequest, _ ...grpc.CallOption) (*catalogactorv1.GetCatalogActorStatesResponse, error) {
	return f(ctx, r)
}

func TestActorSnapshotClientUsesOwnPureFilePrincipalAndFreshExactRequest(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	actor := ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}
	seen := map[string]bool{}
	calls := 0
	remote := actorAuthorityFunc(func(ctx context.Context, r *catalogactorv1.GetCatalogActorStatesRequest) (*catalogactorv1.GetCatalogActorStatesResponse, error) {
		calls++
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		verified, err := principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "file", ExpectedAudience: "user", ExpectedRPC: catalogactorv1.CatalogActorAuthorityService_GetCatalogActorStates_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seen[jti] {
				return errors.New("replay")
			}
			seen[jti] = true
			return nil
		}})
		require.NoError(t, err)
		require.Equal(t, "service:file", verified.Subject)
		require.Empty(t, verified.AccountID)
		require.Empty(t, verified.ProfileID)
		return &catalogactorv1.GetCatalogActorStatesResponse{ProtocolVersion: 1, States: []*catalogactorv1.CatalogActorStateEntry{{Actor: proto.Clone(r.Actors[0]).(*catalogactorv1.CatalogActor), State: catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED}}}, nil
	})
	client := &ActorAuthorityClient{Client: remote, Issuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", "forged", "authorization", "Bearer incoming"))
	for range 2 {
		states, err := client.States(ctx, []ActorPair{actor})
		require.NoError(t, err)
		require.Equal(t, []ActorState{ActorLive}, states)
	}
	require.Equal(t, 2, calls)
	require.Len(t, seen, 2)
}

func TestActorSnapshotClientRejectsAmbiguityWithoutInventingDeletion(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	actor := ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}
	for _, name := range []string{"unknown", "deleted", "nil", "version", "missing", "duplicate", "wrong-account", "wrong-profile", "unknown-enum", "unknown-fields", "rpc-error"} {
		t.Run(name, func(t *testing.T) {
			client := &ActorAuthorityClient{Issuer: issuer, Client: actorAuthorityFunc(func(_ context.Context, r *catalogactorv1.GetCatalogActorStatesRequest) (*catalogactorv1.GetCatalogActorStatesResponse, error) {
				entry := &catalogactorv1.CatalogActorStateEntry{Actor: proto.Clone(r.Actors[0]).(*catalogactorv1.CatalogActor), State: catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED}
				response := &catalogactorv1.GetCatalogActorStatesResponse{ProtocolVersion: 1, States: []*catalogactorv1.CatalogActorStateEntry{entry}}
				switch name {
				case "unknown":
					entry.State = catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_UNKNOWN
				case "deleted":
					entry.State = catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_SOFT_DELETED
				case "nil":
					return nil, nil
				case "version":
					response.ProtocolVersion = 2
				case "missing":
					response.States = nil
				case "duplicate":
					response.States = append(response.States, entry)
				case "wrong-account":
					entry.Actor.AccountId = uuid.NewString()
				case "wrong-profile":
					entry.Actor.ProfileId = uuid.NewString()
				case "unknown-enum":
					entry.State = 99
				case "unknown-fields":
					entry.Actor.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
				case "rpc-error":
					return nil, errors.New("private provider source/user details")
				}
				return response, nil
			})}
			states, err := client.States(context.Background(), []ActorPair{actor})
			if name == "unknown" {
				require.NoError(t, err)
				require.Equal(t, []ActorState{ActorUnknown}, states)
			} else if name == "deleted" {
				require.NoError(t, err)
				require.Equal(t, []ActorState{ActorSoftDeleted}, states)
			} else {
				require.Error(t, err)
				require.Nil(t, states)
				require.NotContains(t, err.Error(), "private provider")
			}
		})
	}
	client := &ActorAuthorityClient{Issuer: issuer, Client: actorAuthorityFunc(func(context.Context, *catalogactorv1.GetCatalogActorStatesRequest) (*catalogactorv1.GetCatalogActorStatesResponse, error) {
		t.Fatal("invalid batch reached remote")
		return nil, nil
	})}
	for _, actors := range [][]ActorPair{nil, {{}}, {actor, actor}} {
		_, err := client.States(context.Background(), actors)
		require.Error(t, err)
	}
}
