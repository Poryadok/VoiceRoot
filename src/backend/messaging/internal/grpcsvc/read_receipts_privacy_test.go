package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/privacy"

	chatv1 "voice.app/voice/chat/v1"
)

type receiptPrivacyStub struct {
	enabled map[uuid.UUID]bool
	err     error
}

func (s receiptPrivacyStub) AllowDMAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}
func (s receiptPrivacyStub) AllowGuestDM(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (s receiptPrivacyStub) AllowFilesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}
func (s receiptPrivacyStub) AllowVoiceMessagesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}
func (s receiptPrivacyStub) AllowForward(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (s receiptPrivacyStub) ShowReadReceipts(_ context.Context, profileID uuid.UUID) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.enabled[profileID], nil
}

func TestShouldPublishReadReceipt_requiresBothDMParticipantsToOptIn(t *testing.T) {
	t.Parallel()
	reader := uuid.New()
	peer := uuid.New()
	chat := uuid.New()
	for _, tc := range []struct {
		name    string
		enabled map[uuid.UUID]bool
		want    bool
	}{
		{"both enabled", map[uuid.UUID]bool{reader: true, peer: true}, true},
		{"reader disabled", map[uuid.UUID]bool{reader: false, peer: true}, false},
		{"peer disabled", map[uuid.UUID]bool{reader: true, peer: false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &MessagingGRPC{
				ChatGuard:        stubChatGuard{peer: peer},
				ChatTypeResolver: faultingTestAuthoritativeChatTypeResolver{typ: chatv1.ChatType_CHAT_TYPE_DM},
				Privacy:          receiptPrivacyStub{enabled: tc.enabled},
			}
			require.Equal(t, tc.want, s.shouldPublishReadReceipt(context.Background(), chat, reader))
		})
	}
}

func TestShouldPublishReadReceipt_failsClosedWhenPrivacyLookupFails(t *testing.T) {
	t.Parallel()
	s := &MessagingGRPC{
		ChatGuard:        stubChatGuard{peer: uuid.New()},
		ChatTypeResolver: faultingTestAuthoritativeChatTypeResolver{typ: chatv1.ChatType_CHAT_TYPE_DM},
		Privacy:          receiptPrivacyStub{err: errors.New("user service unavailable")},
	}
	require.False(t, s.shouldPublishReadReceipt(context.Background(), uuid.New(), uuid.New()))
}

func TestShouldPublishReadReceipt_leavesGroupsUnaffected(t *testing.T) {
	t.Parallel()
	s := &MessagingGRPC{
		ChatGuard:        stubChatGuard{peer: uuid.New()},
		ChatTypeResolver: faultingTestAuthoritativeChatTypeResolver{typ: chatv1.ChatType_CHAT_TYPE_GROUP},
		Privacy:          receiptPrivacyStub{enabled: map[uuid.UUID]bool{}},
	}
	require.True(t, s.shouldPublishReadReceipt(context.Background(), uuid.New(), uuid.New()))
}

func TestShouldPublishReadReceipt_failsClosedWhenDMDependenciesAreMissing(t *testing.T) {
	t.Parallel()
	chat, reader := uuid.New(), uuid.New()
	resolver := faultingTestAuthoritativeChatTypeResolver{typ: chatv1.ChatType_CHAT_TYPE_DM}
	for _, tc := range []struct {
		name   string
		server *MessagingGRPC
	}{
		{"resolver missing", &MessagingGRPC{}},
		{"guard missing", &MessagingGRPC{ChatTypeResolver: resolver}},
		{"privacy missing", &MessagingGRPC{ChatTypeResolver: resolver, ChatGuard: stubChatGuard{peer: uuid.New()}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.False(t, tc.server.shouldPublishReadReceipt(context.Background(), chat, reader))
		})
	}
}
