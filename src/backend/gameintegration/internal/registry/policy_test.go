package registry

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSandboxPolicyRevisionAndFailClosedRead(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
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

func TestPendingProductionPolicyIsOwnerScopedHTTPSOnlyAndUnavailableToAuth(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	owner := uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Game", IdempotencyKey: "app-prod-policy"})
	require.NoError(t, err)
	_, err = store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "sandbox-prod-policy"})
	require.NoError(t, err)
	env, err := store.PrepareProductionAdmission(ctx, PrepareProductionAdmissionInput{ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "prod-policy"})
	require.NoError(t, err)

	input := UpdateSandboxPolicyInput{OwnerAccountID: owner, ApplicationID: app.ID, EnvironmentID: env.ID,
		ExpectedRevision: env.Revision, RedirectURIs: []string{"https://game.example/auth/callback"},
		AllowedOrigins: []string{"https://game.example"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "production-policy-1"}
	updated, err := store.UpdateSandboxPolicy(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "pending", updated.Status)
	_, err = store.LoadAuthorizationPolicy(ctx, env.ID)
	require.ErrorIs(t, err, ErrPolicyUnavailable)

	unsafe := input
	unsafe.IdempotencyKey = "production-policy-loopback"
	unsafe.ExpectedRevision = updated.Revision
	unsafe.RedirectURIs = []string{"http://localhost:8765/callback"}
	_, err = store.UpdateSandboxPolicy(ctx, unsafe)
	require.ErrorIs(t, err, ErrInvalidPolicy)

	foreign := input
	foreign.OwnerAccountID = uuid.New()
	foreign.IdempotencyKey = "production-policy-foreign"
	foreign.ExpectedRevision = updated.Revision
	_, err = store.UpdateSandboxPolicy(ctx, foreign)
	require.ErrorIs(t, err, ErrAdmissionConflict)
}
