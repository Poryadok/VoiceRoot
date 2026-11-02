package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestResolveAppNotificationChatUsesLiveScopeRecipientAndConsent(t *testing.T) {
	ctx := context.Background()
	pool := startT32ResourceMappingPostgres(t, ctx)
	store := &Store{Pool: pool}
	migrationDir := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	migration, err := os.ReadFile(filepath.Join(migrationDir, "000031_t58_app_notification_consent.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)

	app, env := createBindingTestEnvironment(t, ctx, store)
	account, profile, bindingID := uuid.New(), uuid.New(), uuid.New()
	_, err = store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: bindingID, ApplicationID: app, EnvironmentID: env, Provider: "google",
		ProviderSubjectDigest: "hmac-sha256-v1:test:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AccountID:             account, ActorID: uuid.New(), ProfileID: profile, DeviceID: uuid.New(),
	})
	require.NoError(t, err)
	chat := resourceMappingInput(app, env, "chat", "match:consent", uuid.New())
	chat.ChatID = chat.ResourceID
	_, err = store.CreateResourceMapping(ctx, chat)
	require.NoError(t, err)
	insertResourceBindingChatFixture(t, ctx, store, app, env, bindingID, chat.ChatID)

	// An ordinary, unmapped Voice chat must bypass game consent and stay deliverable.
	gameScoped, allowed, err := store.ResolveAppNotificationChat(ctx, profile, uuid.New(), "game_activity", "push")
	require.NoError(t, err)
	require.False(t, gameScoped)
	require.False(t, allowed)

	// A mapped game chat is game-scoped but remains denied until the recipient opts in.
	gameScoped, allowed, err = store.ResolveAppNotificationChat(ctx, profile, chat.ChatID, "game_activity", "push")
	require.NoError(t, err)
	require.True(t, gameScoped)
	require.False(t, allowed)

	consent, err := store.SetAppNotificationConsent(ctx, account, profile, bindingID, 0,
		[]string{"game_activity"}, []string{"push"})
	require.NoError(t, err)
	require.EqualValues(t, 1, consent.Revision)
	gameScoped, allowed, err = store.ResolveAppNotificationChat(ctx, profile, chat.ChatID, "game_activity", "push")
	require.NoError(t, err)
	require.True(t, gameScoped)
	require.True(t, allowed)

	// Consent belongs to the mapped recipient binding, not another profile sharing the chat.
	gameScoped, allowed, err = store.ResolveAppNotificationChat(ctx, uuid.New(), chat.ChatID, "game_activity", "push")
	require.NoError(t, err)
	require.True(t, gameScoped)
	require.False(t, allowed)

	// If one Voice chat is mapped by multiple app/environment scopes, no scope can grant consent.
	otherApp, otherEnv := createBindingTestEnvironment(t, ctx, store)
	otherBindingID, otherAccountID, otherProfileID := uuid.New(), uuid.New(), uuid.New()
	_, err = store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: otherBindingID, ApplicationID: otherApp, EnvironmentID: otherEnv, Provider: "google",
		ProviderSubjectDigest: "hmac-sha256-v1:test:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		AccountID:             otherAccountID, ActorID: uuid.New(), ProfileID: otherProfileID, DeviceID: uuid.New(),
	})
	require.NoError(t, err)
	otherChat := resourceMappingInput(otherApp, otherEnv, "chat", "match:ambiguous", chat.ChatID)
	otherChat.ChatID = chat.ChatID
	_, err = store.CreateResourceMapping(ctx, otherChat)
	require.NoError(t, err)
	insertResourceBindingChatFixture(t, ctx, store, otherApp, otherEnv, otherBindingID, chat.ChatID)
	gameScoped, allowed, err = store.ResolveAppNotificationChat(ctx, profile, chat.ChatID, "game_activity", "push")
	require.NoError(t, err)
	require.True(t, gameScoped)
	require.False(t, allowed, "ambiguous game scopes fail closed even when one scope has opt-in")

	revoked, err := store.SetAppNotificationConsent(ctx, account, profile, bindingID, consent.Revision, nil, nil)
	require.NoError(t, err)
	require.EqualValues(t, 2, revoked.Revision)
	require.NotNil(t, revoked.RevokedAt)
	gameScoped, allowed, err = store.ResolveAppNotificationChat(ctx, profile, chat.ChatID, "game_activity", "push")
	require.NoError(t, err)
	require.True(t, gameScoped)
	require.False(t, allowed)
	_, err = store.SetAppNotificationConsent(ctx, account, profile, bindingID, consent.Revision, []string{"game_activity"}, []string{"push"})
	require.ErrorIs(t, err, ErrAppNotificationConsentConflict, "a stale revision cannot restore revoked consent")

	// Expired mapping leases no longer make an ordinary chat game-managed.
	_, err = pool.Exec(ctx, `UPDATE game_resource_binding_chats SET lease_expires_at=clock_timestamp()-interval '1 second'
		WHERE chat_id=$1`, chat.ChatID)
	require.NoError(t, err)
	gameScoped, allowed, err = store.ResolveAppNotificationChat(ctx, profile, chat.ChatID, "game_activity", "push")
	require.NoError(t, err)
	require.False(t, gameScoped)
	require.False(t, allowed)
}
