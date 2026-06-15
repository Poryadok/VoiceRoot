package grpcsvc

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

// Phase 15 red tests: Signal pre-key exchange + ciphertext-only storage (docs/PLAN.md).

func seedE2EEnabledDM(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `UPDATE chats SET e2e_enabled = true WHERE id = $1`, chatID)
	require.NoError(t, err)
}

func messageContentInDB(t *testing.T, ctx context.Context, pool *pgxpool.Pool, messageID uuid.UUID) string {
	t.Helper()
	var content string
	err := pool.QueryRow(ctx, `SELECT content FROM messages WHERE id = $1`, messageID).Scan(&content)
	require.NoError(t, err)
	return content
}

func samplePreKeyBundleB64() string {
	// Placeholder Signal pre-key bundle blob until client lib generates real payloads.
	return base64.StdEncoding.EncodeToString([]byte("phase15-test-prekey-bundle-v1"))
}

// TestUploadPreKeyBundle_GetPreKeyBundle_Roundtrip documents server-side pre-key directory for X3DH.
func TestUploadPreKeyBundle_GetPreKeyBundle_Roundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profOwner, acctOwner := uuid.New(), uuid.New()
	profPeer, acctPeer := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner, profPeer: acctPeer}

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles})
	t.Cleanup(cleanup)

	bundle := samplePreKeyBundleB64()
	_, err := client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: bundle,
	})
	require.NoError(t, err)

	got, err := client.GetPreKeyBundle(withProfileCtx(ctx, acctPeer, profPeer), &messagingv1.GetPreKeyBundleRequest{
		ProfileId: profOwner.String(),
	})
	require.NoError(t, err)
	require.Equal(t, bundle, got.GetBundle())
}

// TestSendMessage_E2E_WhenChatEnabled_StoresOpaqueContent documents ciphertext-at-rest in messaging_db.messages.content.
func TestSendMessage_E2E_WhenChatEnabled_StoresOpaqueContent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	seedE2EEnabledDM(t, ctx, pool, chatID)

	client, _ := startMessagingServer(t, pool)

	plaintext := "phase15-secret-plaintext"
	ciphertextEnvelope := base64.StdEncoding.EncodeToString([]byte("opaque-e2e-ciphertext-not-plaintext"))
	isE2E := true

	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:         chatDMRef(chatID),
		Content:      ciphertextEnvelope,
		IsE2E:        &isE2E,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	msgID, err := uuid.Parse(sent.GetMessage().GetId())
	require.NoError(t, err)

	stored := messageContentInDB(t, ctx, pool, msgID)
	require.NotEqual(t, plaintext, stored, "server must not store client plaintext for E2E messages")
	require.Equal(t, ciphertextEnvelope, stored)
	require.True(t, sent.GetMessage().GetIsE2E())
}

// TestSendMessage_E2E_WhenChatNotEnabled_Fails documents is_e2e rejected unless chat e2e_enabled.
func TestSendMessage_E2E_WhenChatNotEnabled_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)

	isE2E := true
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "should-not-persist",
		IsE2E:           &isE2E,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestSendMessage_E2E_OnGroup_Fails documents E2E sends are DM-only.
func TestSendMessage_E2E_OnGroup_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	chatID := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)

	isE2E := true
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "group-ciphertext",
		IsE2E:           &isE2E,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestSendMessage_PlaintextRejected_WhenE2EEnabled documents Batch E2E-A: plaintext send
// is rejected when chat e2e_enabled=true (docs/features/encryption.md).
func TestSendMessage_PlaintextRejected_WhenE2EEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	seedE2EEnabledDM(t, ctx, pool, chatID)

	client, _ := startMessagingServer(t, pool)

	isE2E := false
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "plaintext-not-allowed",
		IsE2E:           &isE2E,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestSendMessage_E2E_RequiredFlagWhenEnabled documents is_e2e must be set when chat e2e_enabled.
func TestSendMessage_E2E_RequiredFlagWhenEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	seedE2EEnabledDM(t, ctx, pool, chatID)

	client, _ := startMessagingServer(t, pool)

	// Omit is_e2e — must be rejected same as explicit false.
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "implicit-plaintext",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestEditMessage_Rejected_WhenMessageIsE2E documents E2E ciphertext messages cannot be edited in place.
func TestEditMessage_Rejected_WhenMessageIsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	seedE2EEnabledDM(t, ctx, pool, chatID)

	client, _ := startMessagingServer(t, pool)

	isE2E := true
	ciphertext := base64.StdEncoding.EncodeToString([]byte("opaque-e2e-edit-test"))
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         ciphertext,
		IsE2E:           &isE2E,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)

	_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: sent.GetMessage().GetId(),
		Content:   "edited-plaintext",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestEditMessage_Rejected_WhenChatE2EEnabled documents plain EditMessage is rejected in e2e_enabled chats.
func TestEditMessage_Rejected_WhenChatE2EEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR

	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "legacy-plain-before-gate",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.NoError(t, err)

	seedE2EEnabledDM(t, ctx, pool, chatID)

	_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: sent.GetMessage().GetId(),
		Content:   "edited-in-e2e-chat",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
