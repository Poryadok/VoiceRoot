package grpcsvc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/messaging/internal/store"
)

type unexpectedGameAuthKeys struct{ calls int }

func (s *unexpectedGameAuthKeys) Snapshot(context.Context) (GameAuthKeySnapshot, error) {
	s.calls++
	return GameAuthKeySnapshot{}, nil
}

type unexpectedGameBinding struct{ calls int }

func (s *unexpectedGameBinding) AuthorizeMessageResource(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
	s.calls++
	return nil
}
func (s *unexpectedGameBinding) AuthorizeMessageChat(context.Context, uuid.UUID, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
	s.calls++
	return nil
}

type unexpectedGameFiles struct{ calls int }

func (s *unexpectedGameFiles) VerifyGameAttachmentManifest(context.Context, uuid.UUID, uuid.UUID, []gameprotocol.Attachment) error {
	s.calls++
	return nil
}

type testGameAuthKeys struct {
	keys map[string]*rsa.PublicKey
	at   time.Time
}

func (s testGameAuthKeys) Snapshot(context.Context) (GameAuthKeySnapshot, error) {
	return GameAuthKeySnapshot{Keys: s.keys, RefreshedAt: s.at}, nil
}

type allowGameBinding struct{ profile uuid.UUID }

func (allowGameBinding) AuthorizeMessageResource(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
	return nil
}
func (allowGameBinding) AuthorizeMessageChat(context.Context, uuid.UUID, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
	return nil
}

type denyGameChatBinding struct{}

func (denyGameChatBinding) AuthorizeMessageResource(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
	return nil
}
func (denyGameChatBinding) AuthorizeMessageChat(context.Context, uuid.UUID, gameprotocol.DeviceAuthority, gameprotocol.Message) error {
	return errors.New("chat access revoked")
}

type testGamePermitIssuer struct {
	key              *rsa.PrivateKey
	keyID            string
	profileID        uuid.UUID
	now              time.Time
	issued           int
	outcomes         []string
	completeFailures int
	issueDelay       time.Duration
}

func (s *testGamePermitIssuer) Issue(_ context.Context, authority gameprotocol.DeviceAuthority, operationID uuid.UUID, mutation []byte) (string, error) {
	s.issued++
	issuedAt := s.now
	if s.issueDelay > 0 {
		time.Sleep(s.issueDelay)
		issuedAt = time.Now().UTC().Truncate(time.Millisecond)
	}
	digest := sha256.Sum256(mutation)
	expires := issuedAt.Add(3750 * time.Millisecond)
	if authority.ExpiresAt.Before(expires) {
		expires = authority.ExpiresAt
	}
	claims := map[string]any{
		"version": 1, "iss": "auth", "aud": "voice.game-message", "jti": uuid.NewString(),
		"operation": "message.send", "scope": "game.chat.send", "operation_id": operationID.String(), "request_sha256": hex.EncodeToString(digest[:]),
		"application_id": authority.ApplicationID.String(), "environment_id": authority.EnvironmentID.String(), "account_id": authority.AccountID.String(),
		"actor_id": authority.ActorID.String(), "binding_id": authority.BindingID.String(), "profile_id": s.profileID.String(),
		"device_id": authority.DeviceID.String(), "key_id": authority.KeyID.String(), "device_generation": authority.DeviceGeneration,
		"authority_revision": authority.AuthorityRevision, "gis_permit_id": uuid.NewString(), "binding_revision": int64(5),
		"assertion_jti": authority.AssertionJTI.String(), "iat_ms": issuedAt.UnixMilli(), "expires_at_ms": expires.UnixMilli(), "exp": expires.Unix(),
	}
	header, err := json.Marshal(map[string]any{"alg": "RS256", "kid": s.keyID, "typ": "voice.game-message-execution-permit+jwt"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digestBytes := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digestBytes[:])
	if err != nil {
		return "", err
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *testGamePermitIssuer) Complete(_ context.Context, _, _, _ uuid.UUID, outcome string) error {
	if s.completeFailures > 0 {
		s.completeFailures--
		return errors.New("controlled completion outage")
	}
	s.outcomes = append(s.outcomes, outcome)
	return nil
}

type allowGameFiles struct {
	calls       int
	profileID   uuid.UUID
	chatID      uuid.UUID
	attachments []gameprotocol.Attachment
}

func (s *allowGameFiles) VerifyGameAttachmentManifest(_ context.Context, profileID, chatID uuid.UUID, attachments []gameprotocol.Attachment) error {
	s.calls++
	s.profileID = profileID
	s.chatID = chatID
	s.attachments = append([]gameprotocol.Attachment(nil), attachments...)
	return nil
}

func TestGameMessageProcessorReturnsExpiredExactReceiptBeforeAuthOrFileLookup(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000017_game_message_execution_permits.up.sql"))
	compact, message := expiredSignedCreate(t)
	profileID := uuid.New()
	storeMessages := &store.MessagesStore{Pool: pool}
	_, err := storeMessages.ApplyGameMessage(ctx, message, profileID)
	require.NoError(t, err)

	keys := &unexpectedGameAuthKeys{}
	binding := &unexpectedGameBinding{}
	files := &unexpectedGameFiles{}
	processor := &VerifiedGameMessageProcessor{Store: storeMessages, AuthKeys: keys, Bindings: binding, Files: files}
	row, err := processor.ProcessGameMessage(ctx, compact, "")
	require.NoError(t, err)
	require.Equal(t, message.MessageID, row.ID)
	require.Equal(t, "expired receipt", row.Content)
	require.Zero(t, keys.calls)
	require.Zero(t, binding.calls)
	require.Zero(t, files.calls)
}

func TestGameMessageProcessorFailsClosedForMissingT16BindingAndFileProof(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000017_game_message_execution_permits.up.sql"))
	storeMessages := &store.MessagesStore{Pool: pool}
	now := time.Now().UTC().Truncate(time.Millisecond)
	profile := uuid.New()

	textJWS, textMessage, textAuthority, authPrivate := freshSignedCreate(t, now, false)
	authPublic := &authPrivate.PublicKey
	processor := &VerifiedGameMessageProcessor{Store: storeMessages, AuthKeys: testGameAuthKeys{keys: map[string]*rsa.PublicKey{"auth-current": authPublic}, at: now}, Clock: func() time.Time { return now }}
	_, err := processor.ProcessGameMessage(ctx, textJWS, textAuthority)
	require.Error(t, err)
	require.Contains(t, err.Error(), "execution permit")
	textChatID := textMessage.ChatID
	permits := &testGamePermitIssuer{key: authPrivate, keyID: "auth-current", profileID: profile, now: now}
	processor.Permits = permits
	processor.Bindings = &AuthBackedGameBindingAuthority{
		Chats: &gameAuthorityChatGuard{profile: profile, chat: textChatID},
		// T30/T31 resource mapping is deliberately absent here. Membership
		// alone must not authorize this app/environment/binding/chat tuple.
	}
	_, err = processor.ProcessGameMessage(ctx, textJWS, textAuthority)
	require.Error(t, err)
	require.Contains(t, err.Error(), "game message app-scoped resource authorization denied")
	require.Zero(t, permits.issued, "T30/T31 mapping must be checked before GIS permit issuance")
	require.Empty(t, permits.outcomes)

	attachmentJWS, _, attachmentAuthority, attachmentPrivate := freshSignedCreate(t, now, true)
	authPublic = &attachmentPrivate.PublicKey
	processor.AuthKeys = testGameAuthKeys{keys: map[string]*rsa.PublicKey{"auth-current": authPublic}, at: now}
	processor.Permits = &testGamePermitIssuer{key: attachmentPrivate, keyID: "auth-current", profileID: profile, now: now}
	processor.Bindings = allowGameBinding{profile: profile}
	_, err = processor.ProcessGameMessage(ctx, attachmentJWS, attachmentAuthority)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file attachment provenance")

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id = ANY($1::uuid[])`, []uuid.UUID{textMessage.MessageID}).Scan(&count))
	require.Zero(t, count)
	var total int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions`).Scan(&total))
	require.Zero(t, total)
}

func TestGameMessageProcessorStoresExactVerifiedFileManifest(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000017_game_message_execution_permits.up.sql"))
	storeMessages := &store.MessagesStore{Pool: pool}
	now := time.Now().UTC().Truncate(time.Millisecond)
	profile := uuid.New()
	compact, message, assertion, authPrivate := freshSignedCreate(t, now, true)
	authPublic := &authPrivate.PublicKey
	files := &allowGameFiles{}
	permits := &testGamePermitIssuer{key: authPrivate, keyID: "auth-current", profileID: profile, now: now, completeFailures: 1}
	processor := &VerifiedGameMessageProcessor{
		Store:    storeMessages,
		AuthKeys: testGameAuthKeys{keys: map[string]*rsa.PublicKey{"auth-current": authPublic}, at: now},
		Permits:  permits, Bindings: allowGameBinding{profile: profile}, Files: files, Clock: func() time.Time { return now },
	}
	row, err := processor.ProcessGameMessage(ctx, compact, assertion)
	require.NoError(t, err)
	require.Empty(t, permits.outcomes, "failed remote completion must remain pending")
	require.Equal(t, 1, files.calls)
	require.Equal(t, profile, files.profileID)
	require.Equal(t, message.ChatID, files.chatID)
	require.Len(t, files.attachments, 1)
	require.Equal(t, message.Attachments, files.attachments)
	require.JSONEq(t, `[{"file_id":"`+message.Attachments[0].FileID.String()+`","object_revision":1,"byte_length":5,"content_sha256":"`+message.Attachments[0].ContentSHA256+`","media_type":"image/png"}]`, row.AttachmentsJSON)
	var revisions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, message.MessageID).Scan(&revisions))
	require.Equal(t, 1, revisions)
	var outcome, status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT outcome,status FROM game_message_execution_permit_completions WHERE operation_id=$1`, message.OperationID).Scan(&outcome, &status))
	require.Equal(t, "committed", outcome)
	require.Equal(t, "pending", status)
	require.NoError(t, processor.dispatchPendingGamePermitCompletions(ctx))
	require.Equal(t, []string{"committed"}, permits.outcomes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM game_message_execution_permit_completions WHERE operation_id=$1`, message.OperationID).Scan(&status))
	require.Equal(t, "completed", status)
	issued := permits.issued
	retryRow, err := processor.ProcessGameMessage(ctx, compact, "")
	require.NoError(t, err)
	require.Equal(t, row.ID, retryRow.ID)
	require.Equal(t, issued, permits.issued, "receipt-first retry must not mint another permit")
}

func TestGameMessageProcessorPersistsAbortedPermitAndRejectsRetry(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000017_game_message_execution_permits.up.sql"))
	now := time.Now().UTC().Truncate(time.Millisecond)
	profile := uuid.New()
	compact, message, assertion, authPrivate := freshSignedCreate(t, now, false)
	authPublic := &authPrivate.PublicKey
	permits := &testGamePermitIssuer{key: authPrivate, keyID: "auth-current", profileID: profile, now: now}
	processor := &VerifiedGameMessageProcessor{
		Store:    &store.MessagesStore{Pool: pool},
		AuthKeys: testGameAuthKeys{keys: map[string]*rsa.PublicKey{"auth-current": authPublic}, at: now},
		Permits:  permits, Bindings: denyGameChatBinding{}, Clock: func() time.Time { return now },
	}
	_, err := processor.ProcessGameMessage(ctx, compact, assertion)
	require.ErrorContains(t, err, "chat authorization denied")
	require.Equal(t, 1, permits.issued)
	require.Equal(t, []string{"aborted"}, permits.outcomes)
	var outcome, status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT outcome,status FROM game_message_execution_permit_completions WHERE operation_id=$1`, message.OperationID).Scan(&outcome, &status))
	require.Equal(t, "aborted", outcome)
	require.Equal(t, "completed", status)
	_, err = processor.ProcessGameMessage(ctx, compact, assertion)
	require.ErrorContains(t, err, "already aborted")
	require.Equal(t, 1, permits.issued, "an aborted operation must never mint or consume a replacement permit")
	var revisions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, message.MessageID).Scan(&revisions))
	require.Zero(t, revisions)
}

func TestGameMessageProcessorVerifiesPermitUsingPostNetworkClock(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000017_game_message_execution_permits.up.sql"))
	now := time.Now().UTC().Truncate(time.Millisecond)
	profile := uuid.New()
	compact, _, assertion, authPrivate := freshSignedCreate(t, now, false)
	authPublic := &authPrivate.PublicKey
	permits := &testGamePermitIssuer{key: authPrivate, keyID: "auth-current", profileID: profile, now: now, issueDelay: 350 * time.Millisecond}
	processor := &VerifiedGameMessageProcessor{
		Store:    &store.MessagesStore{Pool: pool},
		AuthKeys: testGameAuthKeys{keys: map[string]*rsa.PublicKey{"auth-current": authPublic}, at: now},
		Permits:  permits, Bindings: allowGameBinding{profile: profile},
	}
	_, err := processor.ProcessGameMessage(ctx, compact, assertion)
	require.NoError(t, err, "a valid permit issued after a bounded Auth call must be checked against the post-call clock")
	require.Equal(t, 1, permits.issued)
	require.Equal(t, []string{"committed"}, permits.outcomes)
}

func expiredSignedCreate(t *testing.T) (string, gameprotocol.Message) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	app, env, account, actor, binding, device := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	operationID, chatID := uuid.New(), uuid.New()
	messageID, err := uuid.NewV7()
	require.NoError(t, err)
	keyID := uuid.New()
	content := []byte("expired receipt")
	digest := sha256.Sum256(content)
	issued := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	expires := issued.Add(4 * time.Minute)
	claims := map[string]any{
		"version": 1, "operation": "create", "audience": "voice.game-message", "application_id": app.String(),
		"environment_id": env.String(), "account_id": account.String(), "actor_id": actor.String(), "binding_id": binding.String(),
		"device_id": device.String(), "operation_id": operationID.String(), "authority_revision": 1, "chat_id": chatID.String(),
		"message_id": messageID.String(), "revision": 1, "previous_revision_hash": nil,
		"issued_at": issued.Format("2006-01-02T15:04:05Z"), "expires_at": expires.Format("2006-01-02T15:04:05Z"),
		"content_type": "text/plain", "content_b64": base64.RawURLEncoding.EncodeToString(content), "content_sha256": hex.EncodeToString(digest[:]),
		"attachment_manifest_b64": nil, "attachment_manifest_sha256": nil,
	}
	headerBytes, err := json.Marshal(map[string]any{"alg": "ES256", "kid": keyID.String(), "typ": "voice.game-message+jws"})
	require.NoError(t, err)
	payloadBytes, err := json.Marshal(claims)
	require.NoError(t, err)
	input := base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(payloadBytes)
	hash := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	require.NoError(t, err)
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	compact := input + "." + base64.RawURLEncoding.EncodeToString(signature)
	message := gameprotocol.Message{
		Compact: compact, KeyID: keyID, Operation: "create", ApplicationID: app, EnvironmentID: env,
		AccountID: account, ActorID: actor, BindingID: binding, DeviceID: device, OperationID: operationID,
		AuthorityRevision: 1, ChatID: chatID, MessageID: messageID, Revision: 1, IssuedAt: issued, ExpiresAt: expires,
		Content: content, ContentSHA256: hex.EncodeToString(digest[:]),
	}
	return compact, message
}

func freshSignedCreate(t *testing.T, now time.Time, withAttachment bool) (string, gameprotocol.Message, string, *rsa.PrivateKey) {
	return freshSignedCreateWithAuthKey(t, now, withAttachment, nil)
}

func freshSignedCreateWithAuthKey(t *testing.T, now time.Time, withAttachment bool, authPrivate *rsa.PrivateKey) (string, gameprotocol.Message, string, *rsa.PrivateKey) {
	t.Helper()
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	app, env, account, actor, binding, device := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	operationID, chatID := uuid.New(), uuid.New()
	messageID, err := uuid.NewV7()
	require.NoError(t, err)
	keyID, authKeyID := uuid.New(), "auth-current"
	content := []byte("hello")
	contentDigest := sha256.Sum256(content)
	issued, expires := now.Add(-time.Second).Truncate(time.Second), now.Add(4*time.Minute).Truncate(time.Second)
	claims := map[string]any{
		"version": 1, "operation": "create", "audience": "voice.game-message", "application_id": app.String(),
		"environment_id": env.String(), "account_id": account.String(), "actor_id": actor.String(), "binding_id": binding.String(),
		"device_id": device.String(), "operation_id": operationID.String(), "authority_revision": 1, "chat_id": chatID.String(),
		"message_id": messageID.String(), "revision": 1, "previous_revision_hash": nil,
		"issued_at": issued.Format("2006-01-02T15:04:05Z"), "expires_at": expires.Format("2006-01-02T15:04:05Z"),
		"content_type": "text/plain", "content_b64": base64.RawURLEncoding.EncodeToString(content), "content_sha256": hex.EncodeToString(contentDigest[:]),
		"attachment_manifest_b64": nil, "attachment_manifest_sha256": nil,
	}
	var attachments []gameprotocol.Attachment
	if withAttachment {
		fileID := uuid.New()
		fileDigest := sha256.Sum256([]byte("image"))
		fileDigestHex := hex.EncodeToString(fileDigest[:])
		manifest, marshalErr := json.Marshal([]map[string]any{{"file_id": fileID.String(), "object_revision": 1, "byte_length": 5, "content_sha256": fileDigestHex, "media_type": "image/png"}})
		require.NoError(t, marshalErr)
		manifestDigest := sha256.Sum256(manifest)
		claims["attachment_manifest_b64"] = base64.RawURLEncoding.EncodeToString(manifest)
		claims["attachment_manifest_sha256"] = hex.EncodeToString(manifestDigest[:])
		attachments = []gameprotocol.Attachment{{FileID: fileID, ObjectRevision: 1, ByteLength: 5, ContentSHA256: fileDigestHex, MediaType: "image/png"}}
	}
	header, err := json.Marshal(map[string]any{"alg": "ES256", "kid": keyID.String(), "typ": "voice.game-message+jws"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, deviceKey, digest[:])
	require.NoError(t, err)
	deviceSignature := make([]byte, 64)
	r.FillBytes(deviceSignature[:32])
	s.FillBytes(deviceSignature[32:])
	compact := input + "." + base64.RawURLEncoding.EncodeToString(deviceSignature)
	jwk := map[string]any{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(deviceKey.X.FillBytes(make([]byte, 32))), "y": base64.RawURLEncoding.EncodeToString(deviceKey.Y.FillBytes(make([]byte, 32)))}
	thumbBytes, err := json.Marshal(map[string]string{"crv": jwk["crv"].(string), "kty": jwk["kty"].(string), "x": jwk["x"].(string), "y": jwk["y"].(string)})
	require.NoError(t, err)
	thumbDigest := sha256.Sum256(thumbBytes)
	thumb := base64.RawURLEncoding.EncodeToString(thumbDigest[:])
	notAfter := now.Add(90 * 24 * time.Hour)
	assertionClaims := map[string]any{
		"version": 1, "iss": "auth", "aud": "voice.game-message", "jti": uuid.NewString(),
		"application_id": app.String(), "environment_id": env.String(), "account_id": account.String(), "actor_id": actor.String(),
		"binding_id": binding.String(), "device_id": device.String(), "key_id": keyID.String(), "public_jwk": jwk,
		"key_thumbprint": thumb, "device_generation": 1, "authority_revision": 1, "status": "active",
		"not_after": notAfter.UnixMilli(), "iat": now.UnixMilli(), "exp": now.Add(4 * time.Second).UnixMilli(),
	}
	if authPrivate == nil {
		authPrivate, err = rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
	}
	authHeader, err := json.Marshal(map[string]any{"alg": "RS256", "kid": authKeyID, "typ": "voice.game-device-status+jwt"})
	require.NoError(t, err)
	authPayload, err := json.Marshal(assertionClaims)
	require.NoError(t, err)
	authInput := base64.RawURLEncoding.EncodeToString(authHeader) + "." + base64.RawURLEncoding.EncodeToString(authPayload)
	authDigest := sha256.Sum256([]byte(authInput))
	authSignature, err := rsa.SignPKCS1v15(rand.Reader, authPrivate, crypto.SHA256, authDigest[:])
	require.NoError(t, err)
	message := gameprotocol.Message{
		Compact: compact, KeyID: keyID, Operation: "create", ApplicationID: app, EnvironmentID: env,
		AccountID: account, ActorID: actor, BindingID: binding, DeviceID: device, OperationID: operationID,
		AuthorityRevision: 1, ChatID: chatID, MessageID: messageID, Revision: 1, IssuedAt: issued, ExpiresAt: expires,
		Content: content, ContentSHA256: hex.EncodeToString(contentDigest[:]), Attachments: attachments,
	}
	return compact, message, authInput + "." + base64.RawURLEncoding.EncodeToString(authSignature), authPrivate
}
