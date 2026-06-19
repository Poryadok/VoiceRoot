package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/privacy"
)

// attachmentPrivacyChecker documents Phase 11 MessagingGRPC.Privacy extension (privacy.md: allow_files / allow_voice_messages).
type attachmentPrivacyChecker interface {
	PrivacyChecker
	AllowFilesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
	AllowVoiceMessagesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

type attachmentPrivacyStub struct {
	friendsOnlyFiles map[uuid.UUID]bool
	friendsOnlyVoice map[uuid.UUID]bool
}

func (s attachmentPrivacyStub) AllowDMAudience(_ context.Context, _ uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

func (s attachmentPrivacyStub) AllowFilesAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.friendsOnlyFiles[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

func (s attachmentPrivacyStub) AllowVoiceMessagesAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.friendsOnlyVoice[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

// TestSendMessage_FileAttachment_FriendsOnlyPrivacy_StrangerDenied documents privacy.md: friends-only allow_files blocks stranger file attachments in DM.
func TestSendMessage_FileAttachment_FriendsOnlyPrivacy_StrangerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000002_client_message_id.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000003_attachment_only_messages.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000005_thread_settings.up.sql")

	profOwner, acctOwner := uuid.New(), uuid.New()
	profStranger, acctStranger := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner, profStranger: acctStranger}

	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profOwner, profStranger)

	fileID := uuid.New().String()
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles: profiles,
		Privacy: attachmentPrivacyStub{
			friendsOnlyFiles: map[uuid.UUID]bool{profOwner: true},
		},
		Friends: noFriendsStub{},
		Files: fileMetadataMap{
			fileID: {
				Id:                fileID,
				UploaderProfileId: profStranger.String(),
				OriginalName:      "blocked.png",
				MimeType:          "image/png",
				SizeBytes:         2048,
				Status:            "ready",
				FileType:          "image",
				ScanResult:        "clean",
				Chat:              chatDMRef(chatID),
			},
		},
	})
	t.Cleanup(cleanup)

	attachments := mustAttachmentJSON(t, []map[string]any{{
		"file_id": fileID,
		"type":    "image",
		"url":     "voice-file://" + fileID,
	}})

	_, err := client.SendMessage(withProfileCtx(ctx, acctStranger, profStranger), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "",
		AttachmentsJson: attachments,
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestSendMessage_VoiceMessage_FriendsOnlyPrivacy_StrangerDenied documents privacy.md: friends-only allow_voice_messages blocks stranger voice attachments in DM.
func TestSendMessage_VoiceMessage_FriendsOnlyPrivacy_StrangerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000005_thread_settings.up.sql"))

	profOwner, acctOwner := uuid.New(), uuid.New()
	profStranger, acctStranger := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner, profStranger: acctStranger}

	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profOwner, profStranger)

	fileID := uuid.New().String()
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles: profiles,
		Privacy: attachmentPrivacyStub{
			friendsOnlyVoice: map[uuid.UUID]bool{profOwner: true},
		},
		Friends: noFriendsStub{},
		Files: fileMetadataMap{
			fileID: {
				Id:                fileID,
				UploaderProfileId: profStranger.String(),
				OriginalName:      "note.ogg",
				MimeType:          "audio/ogg",
				SizeBytes:         4096,
				Status:            "ready",
				FileType:          "audio",
				ScanResult:        "clean",
				Chat:              chatDMRef(chatID),
			},
		},
	})
	t.Cleanup(cleanup)

	attachments := mustAttachmentJSON(t, []map[string]any{{
		"file_id": fileID,
		"type":    "voice_message",
		"url":     "voice-file://" + fileID,
	}})

	_, err := client.SendMessage(withProfileCtx(ctx, acctStranger, profStranger), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "",
		AttachmentsJson: attachments,
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
