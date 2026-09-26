package registry

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestSandboxPolicyRevisionAndFailClosedRead(t *testing.T) {
	ctx := context.Background()
	migration := filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", migration)
	store := &Store{Pool: pool}
	owner := uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Game", IdempotencyKey: "app"})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "approve"})
	require.NoError(t, err)
	_, err = store.LoadAuthorizationPolicy(ctx, env.ID)
	require.ErrorIs(t, err, ErrPolicyUnavailable)
	input := UpdateSandboxPolicyInput{OwnerAccountID: owner, ApplicationID: app.ID, EnvironmentID: env.ID,
		ExpectedRevision: env.Revision, RedirectURIs: []string{"https://game.example/oauth/callback"},
		AllowedOrigins: []string{"https://game.example"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "policy-1"}
	updated, err := store.UpdateSandboxPolicy(ctx, input)
	require.NoError(t, err)
	require.Equal(t, env.Revision+1, updated.Revision)
	retried, err := store.UpdateSandboxPolicy(ctx, input)
	require.NoError(t, err)
	require.Equal(t, updated.Revision, retried.Revision)
	policy, err := store.LoadAuthorizationPolicy(ctx, env.ID)
	require.NoError(t, err)
	require.Equal(t, app.ID, policy.ApplicationID)
	require.Equal(t, []string{"https://game.example/oauth/callback"}, policy.RedirectURIs)
	input.ExpectedRevision = env.Revision
	input.IdempotencyKey = "policy-stale"
	_, err = store.UpdateSandboxPolicy(ctx, input)
	require.ErrorIs(t, err, ErrPolicyConflict)
	input.OwnerAccountID = uuid.New()
	input.IdempotencyKey = "policy-foreign"
	_, err = store.UpdateSandboxPolicy(ctx, input)
	require.ErrorIs(t, err, ErrAdmissionConflict)
}

func TestSandboxPolicyRejectsUnsafeRedirectAndScope(t *testing.T) {
	base := UpdateSandboxPolicyInput{OwnerAccountID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(),
		ExpectedRevision: 1, RedirectURIs: []string{"https://game.example/callback"},
		Providers: []string{"google"}, PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "policy"}
	for _, unsafe := range []string{"http://game.example/callback", "https://evil.example/callback#fragment",
		"https://user:pass@game.example/callback", "javascript://auth/callback"} {
		input := base
		input.RedirectURIs = []string{unsafe}
		_, _, err := normalizePolicy(input)
		require.ErrorIs(t, err, ErrInvalidPolicy, unsafe)
	}
	for _, allowed := range []string{"http://127.0.0.1:8765/callback", "http://localhost:8765/callback", "voicegame://auth/callback"} {
		input := base
		input.RedirectURIs = []string{allowed}
		_, _, err := normalizePolicy(input)
		require.NoError(t, err, allowed)
	}
	base.PlayerScopes = []string{"admin"}
	_, _, err := normalizePolicy(base)
	require.ErrorIs(t, err, ErrInvalidPolicy)
}
