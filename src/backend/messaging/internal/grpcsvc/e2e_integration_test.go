package grpcsvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

// app stack5 red tests: Signal pre-key exchange + ciphertext-only storage (docs/PLAN.md).

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

// validTestPreKeyBundleB64 returns the committed libsignal golden wire for integration tests.
func validTestPreKeyBundleB64(t *testing.T) string {
	t.Helper()
	return libsignalGoldenPreKeyBundleB64(t)
}

func decodePreKeyBundlePayload(t *testing.T, wire string) map[string]any {
	t.Helper()
	jsonBytes, err := base64.StdEncoding.DecodeString(wire)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &payload))
	return payload
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

	bundle := validTestPreKeyBundleB64(t)
	_, err := client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: bundle,
	})
	require.NoError(t, err)

	got, err := client.GetPreKeyBundle(withProfileCtx(ctx, acctPeer, profPeer), &messagingv1.GetPreKeyBundleRequest{
		ProfileId: profOwner.String(),
	})
	require.NoError(t, err)
	uploaded := decodePreKeyBundlePayload(t, bundle)
	fetched := decodePreKeyBundlePayload(t, got.GetBundle())
	require.Equal(t, uploaded["identity_key"], fetched["identity_key"])
	require.Equal(t, uploaded["signed_pre_key_public"], fetched["signed_pre_key_public"])
	require.NotNil(t, fetched["pre_key_id"])
}

// TestUploadPreKeyBundle_RejectsInvalidBundle documents invalid wire is rejected before persistence.
func TestUploadPreKeyBundle_RejectsInvalidBundle(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profOwner, acctOwner := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner}

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles})
	t.Cleanup(cleanup)

	invalidBundle := base64.StdEncoding.EncodeToString([]byte("not-a-signal-prekey-bundle"))
	_, err := client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: invalidBundle,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUploadPreKeyBundle_RejectsOversizedBundle(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profOwner, acctOwner := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner}

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles})
	t.Cleanup(cleanup)

	oversized := strings.Repeat("a", maxPreKeyBundleWireBytes+1)
	_, err := client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: oversized,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestUploadPreKeyBundle_AcceptsLibsignalGoldenBundle documents upload accepts libsignal-signed pre-key wire.
func TestUploadPreKeyBundle_AcceptsLibsignalGoldenBundle(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profOwner, acctOwner := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner}

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles})
	t.Cleanup(cleanup)

	bundle := libsignalGoldenPreKeyBundleB64(t)
	_, err := client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: bundle,
	})
	require.NoError(t, err)
}

func libsignalGoldenPreKeyBundleB64(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "src", "backend", "messaging", "testfixture", "prekey_libsignal_golden.b64")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "run src/frontend/test/tools/export_prekey_golden_test.dart to generate golden")
	return strings.TrimSpace(string(raw))
}

// TestUploadPreKeyBundle_RejectsInvalidSignedPreKeySignature documents libsignal signature verify on upload.
func TestUploadPreKeyBundle_RejectsInvalidSignedPreKeySignature(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	profOwner, acctOwner := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner}

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles})
	t.Cleanup(cleanup)

	bundle := validTestPreKeyBundleB64(t)
	payload := decodePreKeyBundlePayload(t, bundle)
	sigB64, ok := payload["signed_pre_key_signature"].(string)
	require.True(t, ok)
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	require.NoError(t, err)
	sigBytes[0] ^= 0xff
	payload["signed_pre_key_signature"] = base64.StdEncoding.EncodeToString(sigBytes)
	tamperedRaw, err := json.Marshal(payload)
	require.NoError(t, err)
	tampered := base64.StdEncoding.EncodeToString(tamperedRaw)

	_, err = client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: tampered,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func multiOTPKPreKeyBundleB64(t *testing.T) string {
	t.Helper()
	return libsignalGoldenPreKeyBundleB64(t)
}

func preKeyIDFromPayload(t *testing.T, payload map[string]any) int {
	t.Helper()
	id, ok := payload["pre_key_id"].(float64)
	require.True(t, ok, "pre_key_id missing from bundle")
	return int(id)
}

// TestGetPreKeyBundle_ConsumesOTPKFromPool documents multi-OTPK pool: each fetch serves next key.
func TestGetPreKeyBundle_ConsumesOTPKFromPool(t *testing.T) {
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

	bundle := multiOTPKPreKeyBundleB64(t)
	_, err := client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: bundle,
	})
	require.NoError(t, err)

	wantIDs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for _, wantID := range wantIDs {
		got, err := client.GetPreKeyBundle(withProfileCtx(ctx, acctPeer, profPeer), &messagingv1.GetPreKeyBundleRequest{
			ProfileId: profOwner.String(),
		})
		require.NoError(t, err)
		payload := decodePreKeyBundlePayload(t, got.GetBundle())
		require.Equal(t, wantID, preKeyIDFromPayload(t, payload))
	}

	empty, err := client.GetPreKeyBundle(withProfileCtx(ctx, acctPeer, profPeer), &messagingv1.GetPreKeyBundleRequest{
		ProfileId: profOwner.String(),
	})
	require.NoError(t, err)
	emptyPayload := decodePreKeyBundlePayload(t, empty.GetBundle())
	_, hasPreKeyID := emptyPayload["pre_key_id"]
	_, hasPreKeyPublic := emptyPayload["pre_key_public"]
	require.False(t, hasPreKeyID, "OTPK pool exhausted")
	require.False(t, hasPreKeyPublic, "OTPK pool exhausted")
}

// TestGetPreKeyBundle_ConsumesOTPK documents one-time pre-key is removed after first fetch.
func TestGetPreKeyBundle_ConsumesOTPK(t *testing.T) {
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

	bundle := validTestPreKeyBundleB64(t)
	_, err := client.UploadPreKeyBundle(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.UploadPreKeyBundleRequest{
		Bundle: bundle,
	})
	require.NoError(t, err)

	first, err := client.GetPreKeyBundle(withProfileCtx(ctx, acctPeer, profPeer), &messagingv1.GetPreKeyBundleRequest{
		ProfileId: profOwner.String(),
	})
	require.NoError(t, err)
	firstPayload := decodePreKeyBundlePayload(t, first.GetBundle())
	require.NotNil(t, firstPayload["pre_key_id"])
	firstID := preKeyIDFromPayload(t, firstPayload)

	second, err := client.GetPreKeyBundle(withProfileCtx(ctx, acctPeer, profPeer), &messagingv1.GetPreKeyBundleRequest{
		ProfileId: profOwner.String(),
	})
	require.NoError(t, err)
	secondPayload := decodePreKeyBundlePayload(t, second.GetBundle())
	require.NotNil(t, secondPayload["pre_key_id"])
	secondID := preKeyIDFromPayload(t, secondPayload)
	require.NotEqual(t, firstID, secondID, "OTPK pool must serve next key after first fetch")
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

// TestEditMessage_AllowsE2ECiphertextUpdate documents E2E ciphertext edits replace opaque content.
func TestEditMessage_AllowsE2ECiphertextUpdate(t *testing.T) {
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
	ciphertextV1 := base64.StdEncoding.EncodeToString([]byte("opaque-e2e-edit-v1"))
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         ciphertextV1,
		IsE2E:           &isE2E,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)

	ciphertextV2 := base64.StdEncoding.EncodeToString([]byte("opaque-e2e-edit-v2"))
	edited, err := client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: sent.GetMessage().GetId(),
		Content:   ciphertextV2,
	})
	require.NoError(t, err)
	require.Equal(t, ciphertextV2, edited.GetMessage().GetContent())
	require.True(t, edited.GetMessage().GetIsE2E())

	msgID, err := uuid.Parse(sent.GetMessage().GetId())
	require.NoError(t, err)
	require.Equal(t, ciphertextV2, messageContentInDB(t, ctx, pool, msgID))
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
