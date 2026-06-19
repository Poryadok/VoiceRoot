package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/privacy"
)

func TestAttachmentRequiresVoicePrivacy(t *testing.T) {
	t.Parallel()
	require.True(t, attachmentRequiresVoicePrivacy("voice_message"))
	require.False(t, attachmentRequiresVoicePrivacy("image"))
	require.False(t, attachmentRequiresVoicePrivacy("audio"))
}

func TestAttachmentTypeMatchesFileMeta(t *testing.T) {
	t.Parallel()
	require.True(t, attachmentTypeMatchesFileMeta("image", "image"))
	require.True(t, attachmentTypeMatchesFileMeta("voice_message", "audio"))
	require.False(t, attachmentTypeMatchesFileMeta("voice_message", "image"))
	require.True(t, attachmentTypeMatchesFileMeta("image", ""))
}

type dmOtherProfileGuard struct {
	other uuid.UUID
	err   error
}

func (g dmOtherProfileGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (g dmOtherProfileGuard) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	if g.err != nil {
		return uuid.Nil, g.err
	}
	return g.other, nil
}

func TestCheckAttachmentPrivacyForSend_NilPrivacySkips(t *testing.T) {
	t.Parallel()
	s := &MessagingGRPC{Privacy: nil, ChatGuard: dmOtherProfileGuard{other: uuid.New()}}
	require.NoError(t, s.checkAttachmentPrivacyForSend(context.Background(), uuid.New(), uuid.New(), `[{"type":"image"}]`))
}

func TestCheckAttachmentPrivacyForSend_GroupChatSkips(t *testing.T) {
	t.Parallel()
	recipient := uuid.New()
	s := &MessagingGRPC{
		Privacy: attachmentPrivacyStub{friendsOnlyFiles: map[uuid.UUID]bool{recipient: true}},
		Friends: noFriendsStub{},
		ChatGuard: dmOtherProfileGuard{
			err: status.Error(codes.FailedPrecondition, "not a dm chat"),
		},
	}
	require.NoError(t, s.checkAttachmentPrivacyForSend(context.Background(), uuid.New(), uuid.New(), `[{"type":"image"}]`))
}

func TestCheckAttachmentPrivacyForSend_VoiceDenied(t *testing.T) {
	t.Parallel()
	recipient := uuid.New()
	sender := uuid.New()
	s := &MessagingGRPC{
		Privacy: attachmentPrivacyStub{
			friendsOnlyVoice: map[uuid.UUID]bool{recipient: true},
		},
		Friends:   noFriendsStub{},
		ChatGuard: dmOtherProfileGuard{other: recipient},
	}
	err := s.checkAttachmentPrivacyForSend(context.Background(), uuid.New(), sender, `[{"type":"voice_message"}]`)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCheckAttachmentPrivacyForSend_PrivacyDepsUnavailable(t *testing.T) {
	t.Parallel()
	recipient := uuid.New()
	s := &MessagingGRPC{
		Privacy: failingAttachmentPrivacy{},
		Friends: noFriendsStub{},
		ChatGuard: dmOtherProfileGuard{other: recipient},
	}
	err := s.checkAttachmentPrivacyForSend(context.Background(), uuid.New(), uuid.New(), `[{"type":"image"}]`)
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestCheckAttachmentPrivacyForSend_NotMemberDenied(t *testing.T) {
	t.Parallel()
	s := &MessagingGRPC{
		Privacy: attachmentPrivacyStub{},
		ChatGuard: dmOtherProfileGuard{
			err: store.ErrNotChatMember,
		},
	}
	err := s.checkAttachmentPrivacyForSend(context.Background(), uuid.New(), uuid.New(), `[{"type":"image"}]`)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

type failingAttachmentPrivacy struct{}

func (failingAttachmentPrivacy) AllowDMAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

func (failingAttachmentPrivacy) AllowFilesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, errors.New("user grpc unavailable")
}

func (failingAttachmentPrivacy) AllowVoiceMessagesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, errors.New("user grpc unavailable")
}
