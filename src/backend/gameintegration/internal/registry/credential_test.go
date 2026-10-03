package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIssueCredentialIsScopedAndRetryStable(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
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
	require.Equal(t, 90*24*time.Hour, first.ExpiresAt.Sub(first.CreatedAt))

	// Raw URL base64 uses '_' as a valid data character. Verify the full bearer
	// parsing path with a deterministic opaque secret that contains that byte.
	underscoreSecret := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xff}, 32))
	require.Contains(t, underscoreSecret, "_")
	underscoreID := uuid.New()
	underscoreCreated := time.Now().UTC().Truncate(time.Microsecond)
	underscore := Credential{ID: underscoreID, ApplicationID: app.ID, OwnerAccountID: owner,
		EnvironmentID: env.ID, Scopes: []string{"game.events.write"}, Generation: 2,
		CreatedAt: underscoreCreated, ExpiresAt: underscoreCreated.Add(time.Hour)}
	_, err = pool.Exec(ctx, `INSERT INTO service_credentials
		(id, environment_id, secret_digest, scopes, generation, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, underscoreID, env.ID,
		credentialDigest(key, underscoreSecret, underscore), underscore.Scopes, underscore.Generation,
		underscore.CreatedAt, underscore.ExpiresAt)
	require.NoError(t, err)
	principal, err := store.VerifyCredential(ctx, "vgi1_"+underscoreID.String()+"_"+underscoreSecret, "game.events.write", key)
	require.NoError(t, err)
	require.Equal(t, app.ID, principal.ApplicationID)
	require.Equal(t, env.ID, principal.EnvironmentID)
	require.Equal(t, underscoreID, principal.CredentialID)
	principal, err = store.VerifyCredential(ctx, "vgi1_"+first.ID.String()+"_"+first.Secret, "game.events.write", key)
	require.NoError(t, err)
	require.Equal(t, env.ID, principal.EnvironmentID)

	sessionCredential, err := store.IssueCredential(ctx, IssueCredentialInput{OwnerAccountID: owner,
		ApplicationID: app.ID, EnvironmentID: env.ID, Scopes: []string{"game.sessions.manage"},
		IdempotencyKey: "session-credential", SecretKey: key})
	require.NoError(t, err)
	_, err = store.VerifyCredential(ctx, "vgi1_"+sessionCredential.ID.String()+"_"+sessionCredential.Secret, "game.sessions.manage", key)
	require.NoError(t, err)
	_, err = store.VerifyCredential(ctx, "vgi1_"+sessionCredential.ID.String()+"_"+sessionCredential.Secret, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential)

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
	require.Equal(t, 3, count)
	require.NoError(t, store.RevokeCredential(ctx, owner, app.ID, env.ID, first.ID))
	_, err = store.VerifyCredential(ctx, "vgi1_"+first.ID.String()+"_"+first.Secret, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential)
	_, err = pool.Exec(ctx, `UPDATE service_credentials SET revoked_at=NULL WHERE id=$1`, first.ID)
	require.NoError(t, err)
	_, err = store.VerifyCredential(ctx, "vgi1_"+first.ID.String()+"_"+first.Secret, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential, "clearing the revoke timestamp through SQL must not revive a credential")
}

func TestCredentialAuthorizationFieldsCannotBeRewrittenThroughSQL(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Game", IdempotencyKey: "sql-app"})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "sql-approval"})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	issued, err := store.IssueCredential(ctx, IssueCredentialInput{OwnerAccountID: owner, ApplicationID: app.ID,
		EnvironmentID: env.ID, Scopes: []string{"game.events.write"}, IdempotencyKey: "sql-credential", SecretKey: key})
	require.NoError(t, err)
	bearer := "vgi1_" + issued.ID.String() + "_" + issued.Secret

	// A direct database writer can alter ordinary rows, but without the
	// deployment key it cannot recompute the digest that seals authorization.
	_, err = pool.Exec(ctx, `UPDATE service_credentials SET scopes=$2,expires_at=now()+interval '365 days' WHERE id=$1`, issued.ID, []string{"game.sessions.manage"})
	require.NoError(t, err)
	_, err = store.VerifyCredential(ctx, bearer, "game.sessions.manage", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential, "SQL scope escalation must not mint a session-management principal")
	_, err = store.VerifyCredential(ctx, bearer, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential, "the old scope row must also fail after tampering")

	// A caller-chosen token and unkeyed SHA-256 digest inserted through SQL are
	// not a valid service credential either.
	forgedID := uuid.New()
	forgedSecret := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	forgedCreated := time.Now().UTC().Truncate(time.Microsecond)
	forged := Credential{ID: forgedID, ApplicationID: app.ID, OwnerAccountID: owner,
		EnvironmentID: env.ID, Scopes: []string{"game.events.write"}, Generation: 99,
		CreatedAt: forgedCreated, ExpiresAt: forgedCreated.Add(time.Hour)}
	plainDigest := sha256.Sum256([]byte(forgedSecret))
	_, err = pool.Exec(ctx, `INSERT INTO service_credentials
		(id,environment_id,secret_digest,scopes,generation,created_at,expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		forged.ID, forged.EnvironmentID, plainDigest[:], forged.Scopes, forged.Generation, forged.CreatedAt, forged.ExpiresAt)
	require.NoError(t, err)
	_, err = store.VerifyCredential(ctx, "vgi1_"+forgedID.String()+"_"+forgedSecret, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential, "direct SQL must not mint GIS authority without the secret key")
}

func TestCredentialExpiryAndRevealBoundariesUseInjectedClock(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	store := &Store{Pool: pool, Now: func() time.Time { return now }}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Clock", IdempotencyKey: "clock-app"})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "clock-env"})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	input := IssueCredentialInput{OwnerAccountID: owner, ApplicationID: app.ID, EnvironmentID: env.ID,
		Scopes: []string{"game.events.write"}, IdempotencyKey: "clock-credential", SecretKey: key}
	issued, err := store.IssueCredential(ctx, input)
	require.NoError(t, err)
	bearer := "vgi1_" + issued.ID.String() + "_" + issued.Secret

	// Credential expiry is exclusive: one microsecond before is valid; equality
	// and any later time are expired. All decisions use the same injected clock.
	now = issued.ExpiresAt.Add(-time.Microsecond)
	_, err = store.VerifyCredential(ctx, bearer, "game.events.write", key)
	require.NoError(t, err)
	now = issued.ExpiresAt
	_, err = store.VerifyCredential(ctx, bearer, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential)
	now = issued.ExpiresAt.Add(time.Microsecond)
	_, err = store.VerifyCredential(ctx, bearer, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential)

	// Secret redisplay is allowed strictly before the ten-minute cutoff. Equality
	// and later are expired, so the boundary is deterministic and unambiguous.
	now = issued.CreatedAt.Add(10*time.Minute - time.Microsecond)
	_, err = store.IssueCredential(ctx, input)
	require.NoError(t, err)
	now = issued.CreatedAt.Add(10 * time.Minute)
	_, err = store.IssueCredential(ctx, input)
	require.ErrorIs(t, err, ErrCredentialRevealExpired)
	now = issued.CreatedAt.Add(10*time.Minute + time.Microsecond)
	_, err = store.IssueCredential(ctx, input)
	require.ErrorIs(t, err, ErrCredentialRevealExpired)
}

func TestCredentialKeyValidation(t *testing.T) {
	store := &Store{}
	_, err := store.IssueCredential(context.Background(), IssueCredentialInput{
		OwnerAccountID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(),
		Scopes: []string{"game.events.write"}, IdempotencyKey: "x", SecretKey: []byte("short"),
	})
	require.ErrorIs(t, err, ErrInvalidCredentialRequest)
}

func TestCredentialScopesAreAnExplicitAllowlist(t *testing.T) {
	input := IssueCredentialInput{
		OwnerAccountID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(),
		Scopes: []string{"game.sessions.manage"}, IdempotencyKey: "manage-only",
		SecretKey: []byte("0123456789abcdef0123456789abcdef"),
	}
	canonical, _, err := canonicalCredentialInput(input)
	require.NoError(t, err)
	require.Equal(t, []string{"game.sessions.manage"}, canonical.Scopes)

	for _, scopes := range [][]string{
		{"game.sessions.admin"},
		{"game.sessions.manage", "game.sessions.manage"},
		{"game.sessions.manage", "game.admin"},
	} {
		input.Scopes = scopes
		_, _, err := canonicalCredentialInput(input)
		require.ErrorIs(t, err, ErrInvalidCredentialRequest, "scopes=%v", scopes)
	}
}
