package grpcsvc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

func TestMessagingListSharedMedia_listsImageAttachment(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	fileID := uuid.New()
	msgID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	require.NoError(t, store.InsertMessageAttachments(ctx, pool, msgID, chatID, profA, []map[string]string{
		{"file_id": fileID.String(), "type": "image"},
	}, " "))

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Files: fileMetadataMap{
			fileID.String(): {
				Id:           fileID.String(),
				Status:       "ready",
				FileType:     "image",
				ScanResult:   "clean",
				OriginalName: "photo.png",
				SizeBytes:    1024,
				Chat:         chatDMRef(chatID),
			},
		},
	})

	resp, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA,
	})
	require.NoError(t, err)
	items := resp.GetSharedMediaList().GetItems()
	require.Len(t, items, 1)
	require.Equal(t, msgID.String(), items[0].GetMessageId())
	require.Equal(t, fileID.String(), items[0].GetFileId())
	require.Equal(t, "image", items[0].GetAttachmentType())
}

func TestMessagingListSharedMediaEnforcesMessageTimeEntitlementForEveryAttachmentKind(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)

	chatID, sender, viewer, account := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chatID, sender, viewer)
	cutoff := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	attachments := []string{
		"image", "video", "document", "other", "audio", "voice_message", "sticker",
	}
	oldMessageID, joinedMessageID := uuid.New(), uuid.New()
	oldFileIDs := make([]string, 0, len(attachments))
	joinedFileIDs := make([]string, 0, len(attachments))
	makeAttachments := func(ids *[]string) []map[string]string {
		items := make([]map[string]string, 0, len(attachments))
		for _, attachmentType := range attachments {
			fileID := uuid.NewString()
			*ids = append(*ids, fileID)
			items = append(items, map[string]string{"file_id": fileID, "type": attachmentType})
		}
		return items
	}
	require.NoError(t, store.InsertMessageAttachments(ctx, pool, oldMessageID, chatID, sender, makeAttachments(&oldFileIDs), " "))
	require.NoError(t, store.InsertMessageAttachments(ctx, pool, joinedMessageID, chatID, sender, makeAttachments(&joinedFileIDs), " "))
	_, err := pool.Exec(ctx, `UPDATE messages SET created_at = $2 WHERE id = $1`, oldMessageID, cutoff.Add(-time.Nanosecond))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET created_at = $2 WHERE id = $1`, joinedMessageID, cutoff)
	require.NoError(t, err)

	metadata := &recordingFileMetadataLookup{metadata: fileMetadataMap{}}
	for i := range attachments {
		for _, id := range []string{oldFileIDs[i], joinedFileIDs[i]} {
			metadata.metadata[id] = &filev1.FileMetadata{
				Id: id, Status: "ready", FileType: attachments[i], ScanResult: "clean", Chat: chatDMRef(chatID),
			}
		}
	}
	guard := &cutoffMessageReadGuard{
		entitledSQLChatGuard: entitledSQLChatGuard{SQLChatGuard: &store.SQLChatGuard{Pool: pool}},
		cutoff:               cutoff,
	}
	client, _ := startMessagingServerWired(t, pool, messagingWire{ChatGuard: guard, Files: metadata})
	kinds := []messagingv1.SharedMediaKind{
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA,
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_FILES,
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_VOICE,
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_STICKERS,
	}
	expectedCounts := map[messagingv1.SharedMediaKind]int{
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA:    2,
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_FILES:    2,
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_VOICE:    2,
		messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_STICKERS: 1,
	}
	for _, kind := range kinds {
		response, err := client.ListSharedMedia(withProfileCtx(ctx, account, viewer), &messagingv1.ListSharedMediaRequest{
			Chat: chatDMRef(chatID), Kind: kind,
		})
		require.NoError(t, err)
		items := response.GetSharedMediaList().GetItems()
		require.Len(t, items, expectedCounts[kind])
		for _, item := range items {
			require.Equal(t, joinedMessageID.String(), item.GetMessageId(), "join boundary is inclusive")
		}
	}
	for _, requested := range metadata.requested {
		require.NotContains(t, oldFileIDs, requested, "denied attachment IDs must not be sent to File for metadata enrichment")
	}

	guard.err = errors.New("chat entitlement unavailable")
	requestedBeforeFailure := len(metadata.requested)
	_, err = client.ListSharedMedia(withProfileCtx(ctx, account, viewer), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID), Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA,
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Len(t, metadata.requested, requestedBeforeFailure, "File metadata lookup must not run when Chat cannot decide entitlement")
}

type cutoffMessageReadGuard struct {
	entitledSQLChatGuard
	cutoff time.Time
	err    error
}

func (g *cutoffMessageReadGuard) MessageReadEntitled(_ context.Context, _, _ uuid.UUID, createdAt time.Time) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return !createdAt.Before(g.cutoff), nil
}

type recordingFileMetadataLookup struct {
	metadata  fileMetadataMap
	requested []string
}

func (m *recordingFileMetadataLookup) GetBulkMetadata(ctx context.Context, req *filev1.GetBulkMetadataRequest, opts ...grpc.CallOption) (*filev1.GetBulkMetadataResponse, error) {
	m.requested = append(m.requested, req.GetFileIds()...) //nolint:staticcheck // This fixture records the supported legacy file-id request path.
	return m.metadata.GetBulkMetadata(ctx, req, opts...)
}

func TestMessagingListSharedMedia_nonMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	outsider := uuid.New()
	acctOut := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	_, err := client.ListSharedMedia(withProfileCtx(ctx, acctOut, outsider), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingListSharedMedia_excludesDeletedMessage(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	fileID := uuid.New()
	msgID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	require.NoError(t, store.InsertMessageAttachments(ctx, pool, msgID, chatID, profA, []map[string]string{
		{"file_id": fileID.String(), "type": "image"},
	}, " "))
	_, err := pool.Exec(ctx, `UPDATE messages SET deleted_at = now() WHERE id = $1`, msgID)
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Files: fileMetadataMap{
			fileID.String(): {
				Id: fileID.String(), Status: "ready", FileType: "image", ScanResult: "clean", Chat: chatDMRef(chatID),
			},
		},
	})
	resp, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA,
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetSharedMediaList().GetItems())
}

func TestMessagingListSharedMedia_linksTab(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	msgID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	_, err := pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'dm', $3, $4, '[]'::jsonb, '[]'::jsonb)
`, msgID, chatID, profA, "Check [docs](https://voice.app/docs) and https://example.com")
	require.NoError(t, err)

	client, _ := startMessagingServer(t, pool)
	resp, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_LINKS,
	})
	require.NoError(t, err)
	items := resp.GetSharedMediaList().GetItems()
	require.Len(t, items, 2)
	require.Equal(t, "https://voice.app/docs", items[0].GetExternalUrl())
	require.Equal(t, "docs", items[0].GetTitle())
	require.Equal(t, "https://example.com", items[1].GetExternalUrl())
}

func TestMessagingListSharedMedia_voiceTab(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	fileID := uuid.New()
	msgID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	require.NoError(t, store.InsertMessageAttachments(ctx, pool, msgID, chatID, profA, []map[string]string{
		{"file_id": fileID.String(), "type": "audio"},
	}, " "))

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Files: fileMetadataMap{
			fileID.String(): {
				Id: fileID.String(), Status: "ready", FileType: "audio", ScanResult: "clean", Chat: chatDMRef(chatID),
			},
		},
	})
	resp, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_VOICE,
	})
	require.NoError(t, err)
	require.Len(t, resp.GetSharedMediaList().GetItems(), 1)
	require.Equal(t, "audio", resp.GetSharedMediaList().GetItems()[0].GetAttachmentType())
}

func TestMessagingListSharedMedia_stickersTabFiltersStickerAttachments(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	stickerID := uuid.New()
	imageID := uuid.New()
	stickerMessageID := uuid.New()
	imageMessageID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	require.NoError(t, store.InsertMessageAttachments(ctx, pool, stickerMessageID, chatID, profA, []map[string]string{{"file_id": stickerID.String(), "type": "sticker"}}, " "))
	require.NoError(t, store.InsertMessageAttachments(ctx, pool, imageMessageID, chatID, profA, []map[string]string{{"file_id": imageID.String(), "type": "image"}}, " "))

	client, _ := startMessagingServerWired(t, pool, messagingWire{Files: fileMetadataMap{
		stickerID.String(): {Id: stickerID.String(), Status: "ready", FileType: "image", ScanResult: "clean", Chat: chatDMRef(chatID)},
		imageID.String():   {Id: imageID.String(), Status: "ready", FileType: "image", ScanResult: "clean", Chat: chatDMRef(chatID)},
	}})
	resp, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{Chat: chatDMRef(chatID), Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_STICKERS})
	require.NoError(t, err)
	require.Len(t, resp.GetSharedMediaList().GetItems(), 1)
	require.Equal(t, stickerMessageID.String(), resp.GetSharedMediaList().GetItems()[0].GetMessageId())
	require.Equal(t, "sticker", resp.GetSharedMediaList().GetItems()[0].GetAttachmentType())
}

func TestMessagingListSharedMedia_invalidKind(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	_, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_UNSPECIFIED,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestMessagingListSharedMedia_filesKindFiltersDocument(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	imgID := uuid.New()
	docID := uuid.New()
	msgImg := uuid.New()
	msgDoc := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	require.NoError(t, store.InsertMessageAttachments(ctx, pool, msgImg, chatID, profA, []map[string]string{
		{"file_id": imgID.String(), "type": "image"},
	}, " "))
	require.NoError(t, store.InsertMessageAttachments(ctx, pool, msgDoc, chatID, profA, []map[string]string{
		{"file_id": docID.String(), "type": "document"},
	}, " "))

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Files: fileMetadataMap{
			imgID.String(): {
				Id: imgID.String(), Status: "ready", FileType: "image", ScanResult: "clean", Chat: chatDMRef(chatID),
			},
			docID.String(): {
				Id: docID.String(), Status: "ready", FileType: "document", ScanResult: "clean", Chat: chatDMRef(chatID),
			},
		},
	})
	resp, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_FILES,
	})
	require.NoError(t, err)
	require.Len(t, resp.GetSharedMediaList().GetItems(), 1)
	require.Equal(t, docID.String(), resp.GetSharedMediaList().GetItems()[0].GetFileId())
}

func TestMessagingListSharedMedia_returnsE2eKeyWire(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000018_t52_game_cards.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000019_t57_game_action_results.up.sql")
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	fileID := uuid.New()
	msgID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	const keyWire = "voice-e2e-file-key-v1:opaque"
	require.NoError(t, store.InsertMessageAttachments(ctx, pool, msgID, chatID, profA, []map[string]string{
		{"file_id": fileID.String(), "type": "image", "e2e_key_wire": keyWire},
	}, " "))

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Files: fileMetadataMap{
			fileID.String(): {
				Id:           fileID.String(),
				Status:       "ready",
				FileType:     "image",
				ScanResult:   "clean",
				OriginalName: "cipher.png",
				SizeBytes:    512,
				Chat:         chatDMRef(chatID),
			},
		},
	})

	resp, err := client.ListSharedMedia(withProfileCtx(ctx, acctA, profB), &messagingv1.ListSharedMediaRequest{
		Chat: chatDMRef(chatID),
		Kind: messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA,
	})
	require.NoError(t, err)
	items := resp.GetSharedMediaList().GetItems()
	require.Len(t, items, 1)
	require.Equal(t, keyWire, items[0].GetE2EKeyWire())
}
