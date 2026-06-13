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
	"voice/backend/messaging/internal/store"
)

func TestMessagingListSharedMedia_listsImageAttachment(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

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

func TestMessagingListSharedMedia_nonMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

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

func TestMessagingListSharedMedia_invalidKind(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

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
