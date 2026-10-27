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
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type destinationFunc struct {
	chatv1.CatalogAssetServiceClient
	call func(context.Context, *chatv1.AuthorizeCatalogDestinationRequest) (*chatv1.AuthorizeCatalogDestinationResponse, error)
}

func (f destinationFunc) AuthorizeCatalogDestination(ctx context.Context, r *chatv1.AuthorizeCatalogDestinationRequest, _ ...grpc.CallOption) (*chatv1.AuthorizeCatalogDestinationResponse, error) {
	return f.call(ctx, r)
}

func TestDestinationClientUsesPureFileAndRechecksCapturedGenerations(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	input := DestinationRequest{Actor: ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}, Destination: &chatv1.ChatRef{Id: uuid.NewString()}, ChatAccountGeneration: uuid.New(), ChatProfileGeneration: uuid.New()}
	seen := map[string]bool{}
	client := &DestinationAuthorityClient{Issuer: issuer, Client: destinationFunc{call: func(ctx context.Context, r *chatv1.AuthorizeCatalogDestinationRequest) (*chatv1.AuthorizeCatalogDestinationResponse, error) {
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		hash, e := principal.RequestHash(r)
		require.NoError(t, e)
		p, e := principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "file", ExpectedAudience: "chat", ExpectedRPC: chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seen[jti] {
				return errors.New("replay")
			}
			seen[jti] = true
			return nil
		}})
		require.NoError(t, e)
		require.Equal(t, "service:file", p.Subject)
		require.Empty(t, p.AccountID)
		require.Empty(t, p.ProfileID)
		return &chatv1.AuthorizeCatalogDestinationResponse{ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, DestinationChat: proto.Clone(r.DestinationChat).(*chatv1.ChatRef), ObservedChatType: chatv1.ChatType_CHAT_TYPE_GROUP, ChatAccountGeneration: input.ChatAccountGeneration.String(), ChatProfileGeneration: input.ChatProfileGeneration.String()}, nil
	}}}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", "forged", "authorization", "Bearer incoming"))
	for range 2 {
		observed, e := client.Authorize(ctx, input)
		require.NoError(t, e)
		require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, observed)
	}
	require.Len(t, seen, 2)
	input.Destination.Type = chatv1.ChatType_CHAT_TYPE_UNSPECIFIED.Enum()
	_, err = client.Authorize(ctx, input)
	require.NoError(t, err, "explicit optional unspecified type is resolved by authoritative Chat")
}

func TestDestinationClientDeniesStaleOrAmbiguousAuthority(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	issuer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, e)
	input := DestinationRequest{Actor: ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}, Destination: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}, ChatAccountGeneration: uuid.New(), ChatProfileGeneration: uuid.New()}
	for _, name := range []string{"account", "profile", "chat", "type", "unknown-type", "account-generation", "profile-generation", "unknown-fields", "nil", "error"} {
		t.Run(name, func(t *testing.T) {
			c := &DestinationAuthorityClient{Issuer: issuer, Client: destinationFunc{call: func(_ context.Context, r *chatv1.AuthorizeCatalogDestinationRequest) (*chatv1.AuthorizeCatalogDestinationResponse, error) {
				out := &chatv1.AuthorizeCatalogDestinationResponse{ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, DestinationChat: proto.Clone(r.DestinationChat).(*chatv1.ChatRef), ObservedChatType: chatv1.ChatType_CHAT_TYPE_GROUP, ChatAccountGeneration: input.ChatAccountGeneration.String(), ChatProfileGeneration: input.ChatProfileGeneration.String()}
				switch name {
				case "account":
					out.ActorAccountId = uuid.NewString()
				case "profile":
					out.ActorProfileId = uuid.NewString()
				case "chat":
					out.DestinationChat.Id = uuid.NewString()
				case "type":
					out.ObservedChatType = chatv1.ChatType_CHAT_TYPE_DM
				case "unknown-type":
					out.ObservedChatType = 99
				case "account-generation":
					out.ChatAccountGeneration = uuid.NewString()
				case "profile-generation":
					out.ChatProfileGeneration = uuid.NewString()
				case "unknown-fields":
					out.DestinationChat.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
				case "nil":
					return nil, nil
				case "error":
					return nil, errors.New("secret source URL/user detail")
				}
				return out, nil
			}}}
			_, err := c.Authorize(context.Background(), input)
			require.ErrorIs(t, err, ErrDestinationAuthority)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}
