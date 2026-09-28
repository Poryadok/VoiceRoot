package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestCreateResourceMappingReplaysExactReceiptWithNewStoreAndPool(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	app, env := createBindingTestEnvironment(t, ctx, store)
	in := resourceMappingInput(app, env, "chat", "match:one", uuid.New())
	in.ChatID = in.ResourceID

	first, err := store.CreateResourceMapping(ctx, in)
	require.NoError(t, err)
	require.NotZero(t, first)

	replacementPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer replacementPool.Close()
	require.NoError(t, replacementPool.Ping(ctx))
	replacementStore := &Store{Pool: replacementPool}
	retry, err := replacementStore.CreateResourceMapping(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first, retry, "the stored receipt is replayed through a replacement Store and database pool")

	changed := in
	changed.ResourceID = uuid.New()
	changed.ChatID = changed.ResourceID
	_, err = replacementStore.CreateResourceMapping(ctx, changed)
	require.Error(t, err, "reusing an operation ID with a changed request must conflict")
	retry, err = replacementStore.CreateResourceMapping(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first, retry, "a failed changed-body retry preserves the original receipt")
	assertResourceMappingRowUnchanged(t, replacementStore, ctx, in, "active")
}

func TestT30ResourceMappingMigrationDownAndUp(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	migrationDir := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	down, err := os.ReadFile(filepath.Join(migrationDir, "000011_t30_resource_mappings.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)
	var exists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables
		WHERE table_schema='public' AND table_name='game_resource_mappings')`).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists)

	up, err := os.ReadFile(filepath.Join(migrationDir, "000011_t30_resource_mappings.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables
		WHERE table_schema='public' AND table_name='game_resource_mappings')`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestCreateResourceMappingCannotRemapScopedExternalKey(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	chat := resourceMappingInput(app, env, "chat", "match:one", uuid.New())
	first, err := store.CreateResourceMapping(ctx, chat)
	require.NoError(t, err)

	changed := chat
	changed.OperationID = uuid.New()
	changed.ResourceKind = "voice"
	changed.ResourceID = uuid.New()
	_, err = store.CreateResourceMapping(ctx, changed)
	require.Error(t, err, "a scoped key cannot change resource kind or ID")

	changed = chat
	changed.OperationID = uuid.New()
	changed.ChatOperationID = uuid.New()
	changed.ChatRequestHash = chatRequestHash(t, app, env, changed.ChatOperationID, chat.ExternalKey, "Resource mapping changed", "T30 deterministic receipt")
	_, err = store.CreateResourceMapping(ctx, changed)
	require.Error(t, err, "a scoped key cannot replace its Chat operation proof")

	changed = chat
	changed.OperationID = uuid.New()
	changed.ChatRequestHash = chatRequestHash(t, app, env, chat.ChatOperationID, chat.ExternalKey, "Different full-protobuf body", "T30 deterministic receipt")
	_, err = store.CreateResourceMapping(ctx, changed)
	require.Error(t, err, "a scoped key cannot replace its Chat request hash")

	retry, err := store.CreateResourceMapping(ctx, chat)
	require.NoError(t, err)
	require.Equal(t, first, retry, "failed remaps preserve the original operation receipt")
	assertResourceMappingRowUnchanged(t, store, ctx, chat, "active")
}

func TestCreateResourceMappingSameExternalKeyIsIndependentAcrossScopes(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	appA, envA := createBindingTestEnvironment(t, ctx, store)
	appB, envB := createBindingTestEnvironment(t, ctx, store)
	firstInput := resourceMappingInput(appA, envA, "space", "party:shared-text", uuid.New())
	secondInput := resourceMappingInput(appB, envB, "space", "party:shared-text", uuid.New())

	first, err := store.CreateResourceMapping(ctx, firstInput)
	require.NoError(t, err)
	second, err := store.CreateResourceMapping(ctx, secondInput)
	require.NoError(t, err)
	require.NotEqual(t, first, second)

	_, envA2 := createResourceMappingEnvironment(t, ctx, store, appA)
	thirdInput := resourceMappingInput(appA, envA2, "space", "party:shared-text", uuid.New())
	third, err := store.CreateResourceMapping(ctx, thirdInput)
	require.NoError(t, err, "the same key text in a different environment is independent")
	require.NotEqual(t, first, third)
	whitespaceKey := resourceMappingInput(appA, envA, "space", "party:shared-text ", uuid.New())
	whitespaceReceipt, err := store.CreateResourceMapping(ctx, whitespaceKey)
	require.NoError(t, err)
	require.NotEqual(t, first, whitespaceReceipt, "external keys are exact opaque strings and are not trimmed")

	changedScope := resourceMappingInput(appA, envA, "space", "party:shared-text", uuid.New())
	_, err = store.CreateResourceMapping(ctx, changedScope)
	require.Error(t, err, "a key already used in this app/environment remains fenced")
}

func TestCreateResourceMappingRequiresExactChatReceiptForChatAndVoice(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	chat := resourceMappingInput(app, env, "chat", "match:chat", uuid.New())
	chat.ChatID = chat.ResourceID
	_, err := store.CreateResourceMapping(ctx, chat)
	require.NoError(t, err)

	badChat := resourceMappingInput(app, env, "chat", "match:bad-chat", uuid.New())
	badChat.ChatID = uuid.New()
	_, err = store.CreateResourceMapping(ctx, badChat)
	require.Error(t, err, "chat resource ID must equal the returned chat ID in its receipt")

	validVoice := resourceMappingInput(app, env, "voice", "match:voice", uuid.New())
	validVoice.ChatID = chat.ChatID
	validVoice.ChatOperationID = chat.ChatOperationID
	validVoice.ChatRequestHash = chat.ChatRequestHash
	_, err = store.CreateResourceMapping(ctx, validVoice)
	require.NoError(t, err, "a voice resource persists its exact same-scope Chat receipt")

	voiceWithoutReceipt := resourceMappingInput(app, env, "voice", "match:voice-missing", uuid.New())
	voiceWithoutReceipt.ChatID = uuid.New()
	_, err = store.CreateResourceMapping(ctx, voiceWithoutReceipt)
	require.Error(t, err, "voice mapping needs a same-scope Chat receipt")

	_, otherEnv := createResourceMappingEnvironment(t, ctx, store, app)
	foreignChat := resourceMappingInput(app, otherEnv, "chat", "match:foreign-chat", uuid.New())
	foreignChat.ChatID = foreignChat.ResourceID
	_, err = store.CreateResourceMapping(ctx, foreignChat)
	require.NoError(t, err)

	voiceWithForeignReceipt := resourceMappingInput(app, env, "voice", "match:voice-foreign", uuid.New())
	voiceWithForeignReceipt.ChatID = foreignChat.ChatID
	voiceWithForeignReceipt.ChatOperationID = foreignChat.ChatOperationID
	voiceWithForeignReceipt.ChatRequestHash = foreignChat.ChatRequestHash
	_, err = store.CreateResourceMapping(ctx, voiceWithForeignReceipt)
	require.Error(t, err, "a Chat receipt from another environment cannot authorize a voice mapping")
}

func TestAuthorizeAppBindingChatRequiresExactActiveTuple(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	app, env := createBindingTestEnvironment(t, ctx, store)
	bindingID := uuid.New()
	_, err := store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: bindingID, ApplicationID: app, EnvironmentID: env, Provider: "google",
		ProviderSubjectDigest: "hmac-sha256-v1:test:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AccountID:             uuid.New(), ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New(),
	})
	require.NoError(t, err)

	chat := resourceMappingInput(app, env, "chat", "match:authorized", uuid.New())
	chat.ChatID = chat.ResourceID
	_, err = store.CreateResourceMapping(ctx, chat)
	require.NoError(t, err)
	insertResourceBindingChatFixture(t, ctx, store, app, env, bindingID, chat.ChatID)
	otherApp, otherEnv := createBindingTestEnvironment(t, ctx, store)
	otherBindingID := uuid.New()
	_, err = store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: otherBindingID, ApplicationID: otherApp, EnvironmentID: otherEnv, Provider: "google",
		ProviderSubjectDigest: "hmac-sha256-v1:test:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		AccountID:             uuid.New(), ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New(),
	})
	require.NoError(t, err)
	otherChat := resourceMappingInput(otherApp, otherEnv, "chat", "match:foreign-authorized", uuid.New())
	otherChat.ChatID = otherChat.ResourceID
	_, err = store.CreateResourceMapping(ctx, otherChat)
	require.NoError(t, err)
	insertResourceBindingChatFixture(t, ctx, store, otherApp, otherEnv, otherBindingID, otherChat.ChatID)
	_, foreignEnv := createResourceMappingEnvironment(t, ctx, store, app)
	tombstonedChat := resourceMappingInput(app, env, "chat", "match:tombstoned", uuid.New())
	tombstonedChat.ChatID = tombstonedChat.ResourceID
	_, err = store.CreateResourceMapping(ctx, tombstonedChat)
	require.NoError(t, err)
	insertResourceBindingChatFixture(t, ctx, store, app, env, bindingID, tombstonedChat.ChatID)

	allowed, revision, err := store.AuthorizeAppBindingChat(ctx, app, env, bindingID, chat.ChatID)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Positive(t, revision)
	_, err = pool.Exec(ctx, `UPDATE applications SET status='suspended' WHERE id=$1`, app)
	require.NoError(t, err)
	allowed, revision, err = store.AuthorizeAppBindingChat(ctx, app, env, bindingID, chat.ChatID)
	require.NoError(t, err)
	require.False(t, allowed, "suspended applications fail closed")
	require.Zero(t, revision)
	_, err = pool.Exec(ctx, `UPDATE applications SET status='sandbox' WHERE id=$1`, app)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE environments SET status='suspended' WHERE id=$1`, env)
	require.NoError(t, err)
	allowed, revision, err = store.AuthorizeAppBindingChat(ctx, app, env, bindingID, chat.ChatID)
	require.NoError(t, err)
	require.False(t, allowed, "suspended environments fail closed")
	require.Zero(t, revision)
	_, err = pool.Exec(ctx, `UPDATE environments SET status='active' WHERE id=$1`, env)
	require.NoError(t, err)

	for name, tuple := range map[string][4]uuid.UUID{
		"missing application": {uuid.New(), env, bindingID, chat.ChatID},
		"foreign application": {otherApp, env, bindingID, chat.ChatID},
		"missing environment": {app, uuid.New(), bindingID, chat.ChatID},
		"foreign environment": {app, foreignEnv, bindingID, chat.ChatID},
		"missing binding":     {app, env, uuid.New(), chat.ChatID},
		"foreign binding":     {app, env, otherBindingID, chat.ChatID},
		"missing chat":        {app, env, bindingID, uuid.New()},
		"foreign chat":        {app, env, bindingID, otherChat.ChatID},
	} {
		appID, envID, candidateBinding, chatID := tuple[0], tuple[1], tuple[2], tuple[3]
		t.Run(name, func(t *testing.T) {
			allowed, revision, err := store.AuthorizeAppBindingChat(ctx, appID, envID, candidateBinding, chatID)
			require.NoError(t, err)
			require.False(t, allowed)
			require.Zero(t, revision)
		})
	}

	for _, terminalStatus := range []string{"retired", "tombstoned"} {
		t.Run(terminalStatus+" mapping fences its key and is denied", func(t *testing.T) {
			_, err := pool.Exec(ctx, `UPDATE game_resource_mappings SET status=$4
				WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, app, env, tombstonedChat.ExternalKey, terminalStatus)
			require.NoError(t, err)
			allowed, revision, err := store.AuthorizeAppBindingChat(ctx, app, env, bindingID, tombstonedChat.ChatID)
			require.NoError(t, err)
			require.False(t, allowed)
			require.Zero(t, revision)

			recreated := tombstonedChat
			recreated.OperationID = uuid.New()
			_, err = store.CreateResourceMapping(ctx, recreated)
			require.Error(t, err, "a terminal key cannot be recreated")
			assertResourceMappingRowUnchanged(t, store, ctx, tombstonedChat, terminalStatus)
		})
	}

	_, err = pool.Exec(ctx, `UPDATE player_bindings SET status='pending' WHERE binding_id=$1`, bindingID)
	require.NoError(t, err)
	allowed, revision, err = store.AuthorizeAppBindingChat(ctx, app, env, bindingID, chat.ChatID)
	require.NoError(t, err)
	require.False(t, allowed, "inactive/pending GIS bindings fail closed")
	require.Zero(t, revision)

	// Re-activate the fixture so the public revocation transition remains testable.
	_, err = pool.Exec(ctx, `UPDATE player_bindings SET status='active' WHERE binding_id=$1`, bindingID)
	require.NoError(t, err)
	_, err = store.RevokePlayerBinding(ctx, bindingID, 1, uuid.New())
	require.NoError(t, err)
	allowed, revision, err = store.AuthorizeAppBindingChat(ctx, app, env, bindingID, chat.ChatID)
	require.NoError(t, err)
	require.False(t, allowed, "revoked GIS bindings fail closed")
	require.Zero(t, revision)
}

func resourceMappingInput(appID, envID uuid.UUID, kind, externalKey string, resourceID uuid.UUID) ResourceMappingInput {
	in := ResourceMappingInput{
		ApplicationID: appID, EnvironmentID: envID, OperationID: uuid.New(),
		ResourceKind: kind, ExternalKey: externalKey, ResourceID: resourceID,
	}
	if kind == "chat" || kind == "voice" {
		in.ChatOperationID = uuid.New()
		in.ChatRequestHash = chatRequestHash(nil, appID, envID, in.ChatOperationID, externalKey, "Resource mapping chat", "T30 deterministic receipt")
		if kind == "chat" {
			in.ChatID = resourceID
		}
	}
	return in
}

func chatRequestHash(t *testing.T, appID, envID, operationID uuid.UUID, externalKey, name, topic string) string {
	var topicValue *string
	if topic != "" {
		topicValue = &topic
	}
	request := &chatv1.ProvisionManagedChatRequest{
		ApplicationId: appID.String(), EnvironmentId: envID.String(), OperationId: operationID.String(),
		ExternalChatKey: externalKey, Name: name, Topic: topicValue,
	}
	hash, err := principal.RequestHash(request)
	if t != nil {
		require.NoError(t, err)
	} else if err != nil {
		panic(err)
	}
	return hash
}

func assertResourceMappingRowUnchanged(t *testing.T, store *Store, ctx context.Context, input ResourceMappingInput, status string) {
	t.Helper()
	var got struct {
		ResourceKind, Status                string
		ResourceID, ChatID, ChatOperationID uuid.UUID
		ChatRequestHash                     string
	}
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT resource_kind, resource_id, chat_id, chat_operation_id,
		chat_request_hash, status FROM game_resource_mappings
		WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`,
		input.ApplicationID, input.EnvironmentID, input.ExternalKey).Scan(
		&got.ResourceKind, &got.ResourceID, &got.ChatID, &got.ChatOperationID, &got.ChatRequestHash, &got.Status))
	require.Equal(t, input.ResourceKind, got.ResourceKind)
	require.Equal(t, input.ResourceID, got.ResourceID)
	require.Equal(t, input.ChatID, got.ChatID)
	require.Equal(t, input.ChatOperationID, got.ChatOperationID)
	require.Equal(t, input.ChatRequestHash, got.ChatRequestHash)
	require.Equal(t, status, got.Status)
}

func createResourceMappingEnvironment(t *testing.T, ctx context.Context, store *Store, appID uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	envID := uuid.New()
	_, err := store.Pool.Exec(ctx, `UPDATE applications SET status='active' WHERE id=$1`, appID)
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `INSERT INTO environments (id, application_id, kind, status)
		VALUES ($1, $2, 'production', 'active')`, envID, appID)
	require.NoError(t, err)
	return appID, envID
}

func insertResourceBindingChatFixture(t *testing.T, ctx context.Context, store *Store,
	appID, envID, bindingID, chatID uuid.UUID) {
	t.Helper()
	_, err := store.Pool.Exec(ctx, `INSERT INTO game_resource_binding_chats
		(application_id, environment_id, binding_id, chat_id, roster_revision, status, lease_expires_at)
		VALUES ($1,$2,$3,$4,1,'active',clock_timestamp() + interval '60 seconds')`, appID, envID, bindingID, chatID)
	require.NoError(t, err)
}
