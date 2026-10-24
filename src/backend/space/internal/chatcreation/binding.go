// Package chatcreation validates the immutable Space/Chat create protocol.
package chatcreation

import (
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

var ErrInvalid = errors.New("invalid Space Chat creation binding or outcome")

func CanonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

// HasUnknown rejects unknown fields recursively, including oneof messages.
func HasUnknown(message proto.Message) bool {
	if message == nil || !message.ProtoReflect().IsValid() {
		return false
	}
	if len(message.ProtoReflect().GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsList() && field.Message() != nil {
			for i := 0; i < value.List().Len(); i++ {
				if HasUnknown(value.List().Get(i).Message().Interface()) {
					unknown = true
					return false
				}
			}
		} else if field.Message() != nil && HasUnknown(value.Message().Interface()) {
			unknown = true
			return false
		}
		return true
	})
	return unknown
}

func ValidateBinding(b *spacev1.SpaceChatCreationBinding) error {
	if b == nil || HasUnknown(b) || b.GetProtocolVersion() != 1 || len(b.GetCreateRequestSha256()) != 32 {
		return ErrInvalid
	}
	for _, id := range []string{b.GetOperationId(), b.GetCreatorProfileId(), b.GetSpaceId(), b.GetChatId(), b.GetNodeId(), b.GetAuditEventId()} {
		if !CanonicalID(id) {
			return ErrInvalid
		}
	}
	if b.GetChatType() != chatv1.ChatType_CHAT_TYPE_GROUP && b.GetChatType() != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return ErrInvalid
	}
	switch initiator := b.GetInitiator().(type) {
	case *spacev1.SpaceChatCreationBinding_User:
		user := initiator.User
		if user == nil || !CanonicalID(user.GetAccountId()) || user.GetSessionEpoch() <= 0 || (user.GetAccountType() != "regular" && user.GetAccountType() != "guest") {
			return ErrInvalid
		}
	case *spacev1.SpaceChatCreationBinding_Bot:
		bot := initiator.Bot
		if bot == nil || !CanonicalID(bot.GetBotId()) || !CanonicalID(bot.GetInstallationId()) || bot.GetBotScope() != "TEXT_CHAT_CREATE_IN_SPACE" {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func TerminalFailure(code codes.Code) bool {
	switch code {
	case codes.PermissionDenied, codes.NotFound, codes.FailedPrecondition, codes.ResourceExhausted:
		return true
	}
	return false
}

func ValidateOutcome(binding *spacev1.SpaceChatCreationBinding, outcome *spacev1.SpaceChatCreationOutcome) error {
	if ValidateBinding(binding) != nil || outcome == nil || HasUnknown(outcome) || !proto.Equal(binding, outcome.GetBinding()) {
		return ErrInvalid
	}
	switch outcome.GetState() {
	case spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED:
		node := outcome.GetNode()
		if outcome.GetFailureCode() != 0 || node == nil || node.GetId() != binding.GetNodeId() || node.GetSpaceId() != binding.GetSpaceId() || node.GetKind() != "text_chat" || node.GetLinkedChat().GetId() != binding.GetChatId() || node.GetVoiceRoomId() != "" {
			return ErrInvalid
		}
		if node.GetLinkedChat().Type != nil && node.GetLinkedChat().GetType() != binding.GetChatType() {
			return ErrInvalid
		}
	case spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED:
		if outcome.GetNode() != nil || !TerminalFailure(codes.Code(outcome.GetFailureCode())) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
