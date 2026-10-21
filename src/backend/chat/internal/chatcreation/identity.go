// Package chatcreation owns Chat's durable Space-create initiating facts.
package chatcreation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

var ErrInitiator = errors.New("verified Chat create initiator required")

type Initiator struct {
	ProfileID   uuid.UUID
	AccountType string
	User        *spacev1.SpaceChatCreationUser
	Bot         *spacev1.SpaceChatCreationBot
}

func CanonicalID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, ErrInitiator
	}
	return id, nil
}

// Creator rechecks the complete immutable request boundary after the network
// verifier. Raw identity can never become a durable producer fact.
func Creator(ctx context.Context, req *chatv1.CreateChatRequest) (Initiator, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || req == nil || HasUnknown(req) || !p.ExpiresAt.After(time.Now()) {
		return Initiator{}, ErrInitiator
	}
	if _, err := CanonicalID(p.RequestID); err != nil {
		return Initiator{}, ErrInitiator
	}
	if req.GetRequestId() != "" && req.GetRequestId() != p.RequestID {
		return Initiator{}, ErrInitiator
	}
	// A verified in-process context may have no transport metadata. An actual
	// incoming transport must still have its one credential and no raw identity.
	if md, exists := metadata.FromIncomingContext(ctx); exists && len(md) != 0 {
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || transport.RequestID != p.RequestID {
			return Initiator{}, ErrInitiator
		}
	}
	if p.Kind == "delegated_user" {
		user, err := principal.RequireDelegatedUser(ctx, "chat", chatv1.ChatService_CreateChat_FullMethodName, req)
		if err != nil || (user.AccountType != "regular" && user.AccountType != "guest") {
			return Initiator{}, ErrInitiator
		}
		return Initiator{ProfileID: user.ProfileID, AccountType: user.AccountType,
			User: &spacev1.SpaceChatCreationUser{AccountId: user.AccountID.String(), SessionEpoch: user.SessionEpoch, AccountType: user.AccountType}}, nil
	}
	hash, err := principal.RequestHash(req)
	if err != nil || p.Kind != "bot_actor" || p.Issuer != "bot" || p.Audience != "chat" ||
		p.RPC != chatv1.ChatService_CreateChat_FullMethodName || p.RequestHash != hash ||
		p.SpaceID == "" || p.SpaceID != req.GetSpaceId() || p.BotScope != "TEXT_CHAT_CREATE_IN_SPACE" ||
		p.AccountID != "" || p.ProfileID != "" || p.AccountType != "" || p.SessionEpoch != 0 {
		return Initiator{}, ErrInitiator
	}
	actor, err := CanonicalID(p.ActorProfileID)
	if err != nil {
		return Initiator{}, ErrInitiator
	}
	bot, err := CanonicalID(p.BotID)
	if err != nil || p.Subject != "bot:"+bot.String() {
		return Initiator{}, ErrInitiator
	}
	if _, err := CanonicalID(p.InstallationID); err != nil {
		return Initiator{}, ErrInitiator
	}
	if _, err := CanonicalID(p.SpaceID); err != nil {
		return Initiator{}, ErrInitiator
	}
	if req.GetType() != chatv1.ChatType_CHAT_TYPE_GROUP && req.GetType() != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return Initiator{}, ErrInitiator
	}
	return Initiator{ProfileID: actor, Bot: &spacev1.SpaceChatCreationBot{BotId: bot.String(), InstallationId: p.InstallationID, BotScope: p.BotScope}}, nil
}

func HasUnknown(message proto.Message) bool {
	if message == nil || !message.ProtoReflect().IsValid() {
		return false
	}
	m := message.ProtoReflect()
	if len(m.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case f.IsMap():
			if f.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, x protoreflect.Value) bool {
					unknown = HasUnknown(x.Message().Interface())
					return !unknown
				})
			}
		case f.IsList() && f.Message() != nil:
			for i := 0; i < v.List().Len(); i++ {
				if HasUnknown(v.List().Get(i).Message().Interface()) {
					unknown = true
					break
				}
			}
		case f.Message() != nil:
			unknown = HasUnknown(v.Message().Interface())
		}
		return !unknown
	})
	return unknown
}

func ValidBinding(b *spacev1.SpaceChatCreationBinding) bool {
	if b == nil || HasUnknown(b) || b.ProtocolVersion != 1 || len(b.CreateRequestSha256) != 32 {
		return false
	}
	for _, raw := range []string{b.OperationId, b.CreatorProfileId, b.SpaceId, b.ChatId, b.NodeId, b.AuditEventId} {
		if _, err := CanonicalID(raw); err != nil {
			return false
		}
	}
	if b.ChatType != chatv1.ChatType_CHAT_TYPE_GROUP && b.ChatType != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return false
	}
	if user := b.GetUser(); user != nil {
		_, err := CanonicalID(user.AccountId)
		return err == nil && user.SessionEpoch > 0 && (user.AccountType == "regular" || user.AccountType == "guest")
	}
	if bot := b.GetBot(); bot != nil {
		_, e1 := CanonicalID(bot.BotId)
		_, e2 := CanonicalID(bot.InstallationId)
		return e1 == nil && e2 == nil && bot.BotScope == "TEXT_CHAT_CREATE_IN_SPACE"
	}
	return false
}
