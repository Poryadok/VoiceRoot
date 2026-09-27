package registry

import (
	"bytes"
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestIssueCredentialIsScopedAndRetryStable(t *testing.T) {
	ctx := context.Background()
	migration := filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", migration)
	store := &Store{Pool: pool}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Game", IdempotencyKey: "app"})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "approval"})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	in := IssueCredentialInput{OwnerAccountID: owner, ApplicationID: app.ID, EnvironmentID: env.ID,
		Scopes: []string{"game.events.write"}, IdempotencyKey: "credential", SecretKey: key}
	first, err := store.IssueCredential(ctx, in)
	require.NoError(t, err)
	require.NotEmpty(t, first.Secret)
	require.Equal(t, int64(1), first.Generation)

	// Raw URL base64 uses '_' as a valid data character. Verify the full bearer
	// parsing path with a deterministic opaque secret that contains that byte.
	underscoreSecret := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xff}, 32))
	require.Contains(t, underscoreSecret, "_")
	underscoreID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO service_credentials
		(id, environment_id, secret_digest, scopes, generation, expires_at)
		VALUES ($1, $2, $3, $4, 2, now() + interval '1 hour')`,
		underscoreID, env.ID, credentialDigest(key, underscoreSecret), []string{"game.events.write"})
	require.NoError(t, err)
	principal, err := store.VerifyCredential(ctx, "vgi1_"+underscoreID.String()+"_"+underscoreSecret, "game.events.write", key)
	require.NoError(t, err)
	require.Equal(t, app.ID, principal.ApplicationID)
	require.Equal(t, env.ID, principal.EnvironmentID)
	require.Equal(t, underscoreID, principal.CredentialID)
	principal, err = store.VerifyCredential(ctx, "vgi1_"+first.ID.String()+"_"+first.Secret, "game.events.write", key)
	require.NoError(t, err)
	require.Equal(t, env.ID, principal.EnvironmentID)

	_, err = store.VerifyCredential(ctx, "vgi1_"+first.ID.String()+"_"+first.Secret, "game.roster.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential)
	_, err = store.VerifyCredential(ctx, "vgi1_"+first.ID.String()+"_wrong", "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential)
	retry, err := store.IssueCredential(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, retry.ID)
	require.Equal(t, first.Secret, retry.Secret)
	in.Scopes = []string{"game.roster.write"}
	_, err = store.IssueCredential(ctx, in)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	in.OwnerAccountID = uuid.New()
	in.IdempotencyKey = "foreign"
	_, err = store.IssueCredential(ctx, in)
	require.ErrorIs(t, err, ErrAdmissionConflict)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials WHERE environment_id=$1`, env.ID).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, store.RevokeCredential(ctx, owner, app.ID, env.ID, first.ID))
	_, err = store.VerifyCredential(ctx, "vgi1_"+first.ID.String()+"_"+first.Secret, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential)
}

func TestCredentialKeyValidation(t *testing.T) {
	store := &Store{}
	_, err := store.IssueCredential(context.Background(), IssueCredentialInput{
		OwnerAccountID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(),
		Scopes: []string{"game.events.write"}, IdempotencyKey: "x", SecretKey: []byte("short"),
	})
	require.ErrorIs(t, err, ErrInvalidCredentialRequest)
}
