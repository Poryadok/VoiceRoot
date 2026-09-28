package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/pkg/privacy"
)

type gameResourceAuthorityFunc func(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error

func (f gameResourceAuthorityFunc) AuthorizeAppBindingChat(ctx context.Context, authority gameprotocol.DeviceAuthority, message gameprotocol.Message) error {
	return f(ctx, authority, message)
}

type gameAuthorityChatGuard struct {
	profile, chat uuid.UUID
	calls         int
	deny          bool
}

func (g *gameAuthorityChatGuard) EnsureMember(_ context.Context, chatID, profileID uuid.UUID) error {
	g.calls++
	if g.deny || chatID != g.chat || profileID != g.profile {
		return errors.New("not a member")
	}
	return nil
}
func (*gameAuthorityChatGuard) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (*gameAuthorityChatGuard) OtherMemberProfileIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (*gameAuthorityChatGuard) MemberRole(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "member", nil
}
func (*gameAuthorityChatGuard) ResolveChatType(context.Context, uuid.UUID, uuid.UUID) (chatv1.ChatType, error) {
	return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, nil
}
func (*gameAuthorityChatGuard) AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (*gameAuthorityChatGuard) DeletedAmong(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return nil, nil
}
func (*gameAuthorityChatGuard) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (*gameAuthorityChatGuard) AllowDMAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, nil
}
func (*gameAuthorityChatGuard) AllowGuestDM(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
func (*gameAuthorityChatGuard) AllowFilesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, nil
}
func (*gameAuthorityChatGuard) AllowVoiceMessagesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, nil
}
func (*gameAuthorityChatGuard) AllowForward(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}
func (*gameAuthorityChatGuard) GetBulkMetadata(context.Context, *filev1.GetBulkMetadataRequest, ...grpc.CallOption) (*filev1.GetBulkMetadataResponse, error) {
	return nil, nil
}

func TestGameBindingRequiresResourceMappingBeforeChatMembership(t *testing.T) {
	profile, chat := uuid.New(), uuid.New()
	authority := gameprotocol.DeviceAuthority{AuthorityRevision: 7}
	message := gameprotocol.Message{ChatID: chat}
	guard := &gameAuthorityChatGuard{profile: profile, chat: chat}
	resourceCalls := 0
	resourceAuthority := gameResourceAuthorityFunc(func(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
		resourceCalls++
		return nil
	})
	resolver := &AuthBackedGameBindingAuthority{Chats: guard, ResourceMappings: resourceAuthority}
	require.NoError(t, resolver.AuthorizeMessageResource(context.Background(), authority, message))
	require.NoError(t, resolver.AuthorizeMessageChat(context.Background(), profile, authority, message))
	require.Equal(t, 1, resourceCalls)
	require.Equal(t, 1, guard.calls)
	withoutMapping := &AuthBackedGameBindingAuthority{Chats: guard}
	priorMembershipCalls := guard.calls
	require.Error(t, withoutMapping.AuthorizeMessageResource(context.Background(), authority, message), "app/environment/binding-to-chat mapping is mandatory")
	require.Equal(t, priorMembershipCalls, guard.calls, "membership cannot substitute for app-scoped resource authority")
	deniedMapping := &AuthBackedGameBindingAuthority{Chats: guard, ResourceMappings: gameResourceAuthorityFunc(func(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
		return errors.New("not linked")
	})}
	require.Error(t, deniedMapping.AuthorizeMessageResource(context.Background(), authority, message), "mapping denial fails closed")
	require.Equal(t, priorMembershipCalls, guard.calls, "membership must not run after resource mapping denial")
	guard.deny = true
	require.Error(t, resolver.AuthorizeMessageChat(context.Background(), profile, authority, message), "membership denial fails closed")
}
