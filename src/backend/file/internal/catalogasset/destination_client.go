package catalogasset

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

var ErrDestinationAuthority = errors.New("catalog destination authority unavailable")

type DestinationRequest struct {
	Actor                                        ActorPair
	Destination                                  *chatv1.ChatRef
	ChatAccountGeneration, ChatProfileGeneration uuid.UUID
}

// DestinationAuthorityClient obtains a current, read-only Chat observation
// using File's pure service principal, never forwarded user identity headers.
// The observation does not replace File's local actor generation barrier.
type DestinationAuthorityClient struct {
	Client chatv1.CatalogAssetServiceClient
	Issuer *principal.Issuer
}

func (c *DestinationAuthorityClient) Authorize(ctx context.Context, input DestinationRequest) (chatv1.ChatType, error) {
	if input.Actor.AccountID == uuid.Nil || input.Actor.ProfileID == uuid.Nil || input.ChatAccountGeneration == uuid.Nil || input.ChatProfileGeneration == uuid.Nil || input.Destination == nil || !canonicalID(input.Destination.Id) || len(input.Destination.ProtoReflect().GetUnknown()) != 0 || (input.Destination.GetType() != 0 && !validChatType(input.Destination.GetType())) {
		return 0, ErrDestinationAuthority
	}
	if c == nil || nilActorClient(c.Client) || c.Issuer == nil {
		return 0, ErrDestinationAuthority
	}
	r := &chatv1.AuthorizeCatalogDestinationRequest{ProtocolVersion: 1, ActorAccountId: input.Actor.AccountID.String(), ActorProfileId: input.Actor.ProfileID.String(), DestinationChat: proto.Clone(input.Destination).(*chatv1.ChatRef)}
	hash, err := principal.RequestHash(r)
	if err != nil {
		return 0, ErrDestinationAuthority
	}
	id := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return 0, ErrDestinationAuthority
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	out, err := c.Client.AuthorizeCatalogDestination(call, r)
	if err != nil {
		return 0, errors.Join(ErrDestinationAuthority, call.Err())
	}
	if out == nil || len(out.ProtoReflect().GetUnknown()) != 0 || out.ActorAccountId != r.ActorAccountId || out.ActorProfileId != r.ActorProfileId || !proto.Equal(out.DestinationChat, r.DestinationChat) || !validChatType(out.ObservedChatType) || (r.DestinationChat.GetType() != 0 && r.DestinationChat.GetType() != out.ObservedChatType) || out.ChatAccountGeneration != input.ChatAccountGeneration.String() || out.ChatProfileGeneration != input.ChatProfileGeneration.String() {
		return 0, ErrDestinationAuthority
	}
	return out.ObservedChatType, nil
}

func validChatType(value chatv1.ChatType) bool {
	return value == chatv1.ChatType_CHAT_TYPE_DM || value == chatv1.ChatType_CHAT_TYPE_GROUP || value == chatv1.ChatType_CHAT_TYPE_CHANNEL
}
