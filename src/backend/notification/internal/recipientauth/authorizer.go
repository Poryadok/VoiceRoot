// Package recipientauth revalidates the current Chat and Social audience for
// Notification-owned in-app routing; a member snapshot alone grants nothing.
package recipientauth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/notification/internal/s2s"
)

type ChatReader interface {
	GetChat(context.Context, *chatv1.GetChatRequest, ...grpc.CallOption) (*chatv1.GetChatResponse, error)
}
type ProfileReader interface {
	GetProfile(context.Context, *userv1.GetProfileRequest, ...grpc.CallOption) (*userv1.GetProfileResponse, error)
}
type BlockReader interface {
	IsBlocked(context.Context, *socialv1.IsBlockedRequest, ...grpc.CallOption) (*socialv1.IsBlockedResponse, error)
}
type Authorizer struct {
	Chat     ChatReader
	Profiles ProfileReader
	Blocks   BlockReader
}

var ErrUnavailable = errors.New("notification recipient authority unavailable")

func canonical(raw string) bool {
	id, e := uuid.Parse(raw)
	return e == nil && id != uuid.Nil && id.String() == raw
}
func (a *Authorizer) account(ctx context.Context, profile string) (string, error) {
	if !canonical(profile) {
		return "", ErrUnavailable
	}
	response, e := a.Profiles.GetProfile(s2s.Context(ctx), &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: profile}})
	if e != nil || response.GetProfile() == nil || response.GetProfile().GetId() != profile || !canonical(response.GetProfile().GetAccountId()) {
		return "", ErrUnavailable
	}
	return response.GetProfile().GetAccountId(), nil
}

// Authorize treats ordinary permission/not-found denials as no-delivery, while
// uncertainty fails the whole decision rather than returning a partial allow set.
func (a *Authorizer) Authorize(ctx context.Context, recipient, sender, chat string) (bool, error) {
	if a == nil || a.Chat == nil || a.Profiles == nil || a.Blocks == nil || !canonical(chat) {
		return false, ErrUnavailable
	}
	recipientAccount, e := a.account(ctx, recipient)
	if e != nil {
		return false, e
	}
	senderAccount, e := a.account(ctx, sender)
	if e != nil {
		return false, e
	}
	// Replace all ambient identity/authorization/internal-caller metadata. The
	// normal Chat ACL must run as this recipient, never as Notification S2S.
	recipientCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-user-id", recipientAccount, "x-voice-profile-id", recipient))
	response, e := a.Chat.GetChat(recipientCtx, &chatv1.GetChatRequest{ChatId: chat})
	if status.Code(e) == codes.PermissionDenied || status.Code(e) == codes.NotFound {
		return false, nil
	}
	if e != nil || response.GetChat() == nil || response.GetChat().GetId() != chat {
		return false, ErrUnavailable
	}
	kind := response.GetChat().GetType()
	if kind != chatv1.ChatType_CHAT_TYPE_DM && kind != chatv1.ChatType_CHAT_TYPE_GROUP && kind != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return false, ErrUnavailable
	}
	if recipientAccount == senderAccount {
		return true, nil
	}
	pairs := [][2]string{{recipientAccount, senderAccount}}
	if kind == chatv1.ChatType_CHAT_TYPE_DM {
		pairs = append(pairs, [2]string{senderAccount, recipientAccount})
	}
	for _, pair := range pairs {
		blocked, e := a.Blocks.IsBlocked(s2s.Context(ctx), &socialv1.IsBlockedRequest{AccountIdA: pair[0], AccountIdB: pair[1]})
		if e != nil || blocked == nil {
			return false, ErrUnavailable
		}
		if blocked.GetBlocked() {
			return false, nil
		}
	}
	return true, nil
}
