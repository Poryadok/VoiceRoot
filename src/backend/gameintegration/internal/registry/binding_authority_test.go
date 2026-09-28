package registry

import (
	"context"
	"crypto/sha256"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestPlayerBindingAuthorityPersistsAndRevokesWithReplaySafety(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	bindingID := uuid.New()
	in := CreatePlayerBindingInput{
		BindingID: bindingID, ApplicationID: app, EnvironmentID: env,
		Provider: "google", ProviderSubjectDigest: "hmac-sha256-v1:test:" + strings.Repeat("a", 64),
		AccountID: uuid.New(), ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New(),
	}

	created, err := store.CreatePlayerBinding(ctx, in)
	require.NoError(t, err)
	require.Equal(t, bindingID, created.BindingID)
	require.Equal(t, app, created.ApplicationID)
	require.Equal(t, env, created.EnvironmentID)
	require.Equal(t, "active", created.Status)
	require.EqualValues(t, 1, created.BindingRevision)

	retry, err := store.CreatePlayerBinding(ctx, in)
	require.NoError(t, err)
	require.Equal(t, created, retry, "a retry by the future binding writer must not mint or revise a binding")

	operationID := uuid.New()
	_, err = store.RevokePlayerBindingForOwner(ctx, bindingID, uuid.New(), 1, uuid.New())
	require.ErrorIs(t, err, ErrPlayerBindingNotFound, "another account cannot revoke the selected profile binding")
	stillActive, err := store.LoadPlayerBindingAuthority(ctx, bindingID)
	require.NoError(t, err)
	require.Equal(t, "active", stillActive.Status)
	revoked, err := store.RevokePlayerBindingForOwner(ctx, bindingID, in.AccountID, 1, operationID)
	require.NoError(t, err)
	require.Equal(t, "revoked", revoked.Status)
	require.EqualValues(t, 2, revoked.BindingRevision)

	revocationRetry, err := store.RevokePlayerBinding(ctx, bindingID, 1, operationID)
	require.NoError(t, err)
	require.Equal(t, revoked, revocationRetry)

	loaded, err := store.LoadPlayerBindingAuthority(ctx, bindingID)
	require.NoError(t, err)
	require.Equal(t, revoked, loaded)
	_, err = store.RevokePlayerBinding(ctx, bindingID, 1, uuid.New())
	require.ErrorIs(t, err, ErrPlayerBindingConflict)
}

func TestPlayerBindingRevocationRacesSerializeToOneEpoch(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	binding, err := store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: uuid.New(), ApplicationID: app, EnvironmentID: env,
		Provider: "google", ProviderSubjectDigest: "hmac-sha256-v1:test:" + strings.Repeat("a", 64),
		AccountID: uuid.New(), ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New(),
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.RevokePlayerBinding(ctx, binding.BindingID, 1, uuid.New())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if err == ErrPlayerBindingConflict {
			conflicts++
		} else {
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	current, err := store.LoadPlayerBindingAuthority(ctx, binding.BindingID)
	require.NoError(t, err)
	require.Equal(t, "revoked", current.Status)
	require.EqualValues(t, 2, current.BindingRevision)
}

func TestPlayerBindingRejectsInvalidAndCrossEnvironmentReferences(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	_, err := store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: env,
		AccountID: uuid.New(), ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New(),
	})
	require.ErrorIs(t, err, ErrInvalidPlayerBinding)
	_, err = store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: uuid.New(), ApplicationID: app, EnvironmentID: uuid.New(),
		Provider: "google", ProviderSubjectDigest: "hmac-sha256-v1:test:" + strings.Repeat("a", 64),
		AccountID: uuid.New(), ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New(),
	})
	require.ErrorIs(t, err, ErrInvalidPlayerBinding)
	_, err = store.LoadPlayerBindingAuthority(ctx, uuid.New())
	require.ErrorIs(t, err, ErrPlayerBindingNotFound)
}

func TestPlayerBindingExecutionPermitIsBoundedIdempotentAndRevocationDrains(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	claims := activeBindingClaims(app, env)
	binding, err := store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: claims.BindingID, ApplicationID: app, EnvironmentID: env,
		Provider: "google", ProviderSubjectDigest: "hmac-sha256-v1:test:" + strings.Repeat("a", 64),
		AccountID: claims.AccountID, ActorID: claims.ActorID, ProfileID: uuid.New(), DeviceID: claims.DeviceID,
	})
	require.NoError(t, err)

	operationID := uuid.New()
	permit, err := store.IssuePlayerBindingExecutionPermit(ctx, binding.BindingID, operationID, claims)
	require.NoError(t, err)
	require.Equal(t, binding.BindingID, permit.BindingID)
	require.Equal(t, claims.AssertionID, permit.AssertionID)
	require.Equal(t, operationID, permit.OperationID)
	require.LessOrEqual(t, permit.ExpiresAt.Sub(time.Now()), gameMessagePermitLifetime+time.Second)
	require.Greater(t, permit.ExpiresAt, time.Now())

	retry, err := store.IssuePlayerBindingExecutionPermit(ctx, binding.BindingID, operationID, claims)
	require.NoError(t, err)
	require.Equal(t, permit, retry, "same op/assertion retry returns the original deadline")

	drainCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	revocationID := uuid.New()
	_, err = store.RevokePlayerBinding(drainCtx, binding.BindingID, 1, revocationID)
	require.ErrorIs(t, err, ErrPlayerBindingDrainPending)
	current, err := store.LoadPlayerBindingAuthority(ctx, binding.BindingID)
	require.NoError(t, err)
	require.Equal(t, "revoking", current.Status)
	_, err = store.IssuePlayerBindingExecutionPermit(ctx, binding.BindingID, uuid.New(), claims)
	require.ErrorIs(t, err, ErrPlayerBindingPermitDenied, "revoking binding denies fresh operation permits")
	_, err = store.IssuePlayerBindingExecutionPermit(ctx, binding.BindingID, operationID, claims)
	require.NoError(t, err, "an exact previously issued op retry recovers the durable permit")

	ack, err := store.CompletePlayerBindingExecutionPermit(ctx, permit.PermitID, operationID, "committed")
	require.NoError(t, err)
	require.Equal(t, "completed", ack.Status)
	ackRetry, err := store.CompletePlayerBindingExecutionPermit(ctx, permit.PermitID, operationID, "committed")
	require.NoError(t, err)
	require.Equal(t, ack, ackRetry)
	_, err = store.CompletePlayerBindingExecutionPermit(ctx, permit.PermitID, operationID, "aborted")
	require.ErrorIs(t, err, ErrPlayerBindingConflict)

	revoked, err := store.RevokePlayerBinding(ctx, binding.BindingID, 1, revocationID)
	require.NoError(t, err)
	require.Equal(t, "revoked", revoked.Status)
	require.EqualValues(t, 2, revoked.BindingRevision)
}

func TestPlayerBindingExecutionPermitRejectsStaleAndMismatchedClaims(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	claims := activeBindingClaims(app, env)
	binding, err := store.CreatePlayerBinding(ctx, CreatePlayerBindingInput{
		BindingID: claims.BindingID, ApplicationID: app, EnvironmentID: env,
		Provider: "google", ProviderSubjectDigest: "hmac-sha256-v1:test:" + strings.Repeat("a", 64),
		AccountID: claims.AccountID, ActorID: claims.ActorID, ProfileID: uuid.New(), DeviceID: claims.DeviceID,
	})
	require.NoError(t, err)

	stale := claims
	stale.ExpiresAt = time.Now().Add(-time.Second)
	_, err = store.IssuePlayerBindingExecutionPermit(ctx, binding.BindingID, uuid.New(), stale)
	require.ErrorIs(t, err, ErrPlayerBindingPermitDenied)

	other := claims
	other.AccountID = uuid.New()
	_, err = store.IssuePlayerBindingExecutionPermit(ctx, binding.BindingID, uuid.New(), other)
	require.ErrorIs(t, err, ErrPlayerBindingPermitDenied)
}

func TestPlayerBindingExchangeOutboxPersistsReplaySafeBindingLifecycle(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	challenge := BindingChallenge{
		ChallengeID: uuid.New(), Nonce: strings.Repeat("n", 43), ApplicationID: app, EnvironmentID: env,
		Provider: "google", RedirectURIHash: strings.Repeat("a", 64), PKCEChallenge: strings.Repeat("b", 43),
		DeviceKeyID: uuid.New(), DeviceKeyThumbprint: strings.Repeat("c", 43), OperationID: uuid.New(),
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute), Status: "pending",
	}
	require.NoError(t, store.CreateBindingChallenge(ctx, challenge))

	requestHash := strings.Repeat("d", 64)
	operation, err := store.BeginPlayerBindingExchange(ctx, challenge.ChallengeID, requestHash)
	require.NoError(t, err)
	require.Equal(t, challenge.OperationID, operation.OperationID)
	retry, err := store.BeginPlayerBindingExchange(ctx, challenge.ChallengeID, requestHash)
	require.NoError(t, err)
	require.Equal(t, operation.OperationID, retry.OperationID)
	_, err = store.BeginPlayerBindingExchange(ctx, challenge.ChallengeID, strings.Repeat("e", 64))
	require.ErrorIs(t, err, ErrBindingExchangeConflict)

	claimID, assertionJTI := uuid.New(), uuid.New()
	const handoff = "header.payload.signature"
	require.NoError(t, store.RecordPlayerBindingExchangeClaim(ctx, operation.OperationID, requestHash,
		handoff, claimID, assertionJTI))
	require.NoError(t, store.RecordPlayerBindingExchangeClaim(ctx, operation.OperationID, requestHash,
		handoff, claimID, assertionJTI), "exact Auth receipt replay is idempotent")
	require.ErrorIs(t, store.RecordPlayerBindingExchangeClaim(ctx, operation.OperationID, requestHash,
		"different.handoff.signature", claimID, assertionJTI), ErrBindingExchangeConflict)

	in := CreatePlayerBindingInput{
		ApplicationID: app, EnvironmentID: env, Provider: "google",
		ProviderSubjectDigest: "hmac-sha256-v1:test:" + strings.Repeat("f", 64),
		AccountID:             uuid.New(), ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New(),
	}
	created, err := store.CommitPlayerBindingExchange(ctx, operation.OperationID, requestHash, claimID, assertionJTI, in)
	require.NoError(t, err)
	require.Equal(t, "pending", created.Status)
	storedOperation, err := store.LoadPlayerBindingOperationID(ctx, created.BindingID)
	require.NoError(t, err)
	require.Equal(t, operation.OperationID, storedOperation, "Auth revocation uses the stable grant operation ID")
	retryCreated, err := store.CommitPlayerBindingExchange(ctx, operation.OperationID, requestHash, claimID, assertionJTI, in)
	require.NoError(t, err)
	require.Equal(t, created, retryCreated, "creation retry preserves GIS binding UUID")
	_, err = store.LoadBindingChallenge(ctx, challenge.ChallengeID)
	require.ErrorIs(t, err, ErrBindingChallengeNotFound, "challenge is consumed atomically with binding and outbox")

	completion, err := store.NextPlayerBindingCompletion(ctx)
	require.NoError(t, err)
	require.Equal(t, operation.OperationID, completion.OperationID)
	require.Equal(t, claimID, completion.ClaimID)
	require.Equal(t, assertionJTI, completion.AssertionJTI)
	require.Equal(t, created.BindingID, completion.BindingID)
	require.NoError(t, store.RecordPlayerBindingCompletionAttempt(ctx, operation.OperationID))
	require.NoError(t, store.CompletePlayerBindingExchange(ctx, operation.OperationID, claimID,
		assertionJTI, created.BindingID))
	require.NoError(t, store.CompletePlayerBindingExchange(ctx, operation.OperationID, claimID,
		assertionJTI, created.BindingID), "completion ACK retry is idempotent")

	active, err := store.LoadPlayerBindingAuthority(ctx, created.BindingID)
	require.NoError(t, err)
	require.Equal(t, "active", active.Status)
	require.EqualValues(t, 2, active.BindingRevision)
	completed, err := store.CommitPlayerBindingExchange(ctx, operation.OperationID, requestHash, claimID, assertionJTI, in)
	require.NoError(t, err)
	require.Equal(t, BindingExchangeResult{OperationID: operation.OperationID, BindingID: created.BindingID, Status: "active"}, completed)
}

func TestPlayerBindingExchangeFailureUsesDurableAuthCompletionOutbox(t *testing.T) {
	ctx := context.Background()
	store := &Store{Pool: startT12Postgres(t, ctx)}
	app, env := createBindingTestEnvironment(t, ctx, store)
	challenge := BindingChallenge{ChallengeID: uuid.New(), Nonce: strings.Repeat("n", 43),
		ApplicationID: app, EnvironmentID: env, Provider: "google", RedirectURIHash: strings.Repeat("a", 64),
		PKCEChallenge: strings.Repeat("b", 43), DeviceKeyID: uuid.New(), DeviceKeyThumbprint: strings.Repeat("c", 43),
		OperationID: uuid.New(), ExpiresAt: time.Now().Add(5 * time.Minute), Status: "pending"}
	require.NoError(t, store.CreateBindingChallenge(ctx, challenge))
	requestHash, claimID, jti := strings.Repeat("d", 64), uuid.New(), uuid.New()
	operation, err := store.BeginPlayerBindingExchange(ctx, challenge.ChallengeID, requestHash)
	require.NoError(t, err)
	require.NoError(t, store.RecordPlayerBindingExchangeClaim(ctx, operation.OperationID, requestHash,
		"header.payload.signature", claimID, jti))
	require.NoError(t, store.FailPlayerBindingExchange(ctx, operation.OperationID, requestHash, claimID, jti))

	completion, err := store.NextPlayerBindingCompletion(ctx)
	require.NoError(t, err)
	require.Equal(t, "failed", completion.Outcome)
	require.Equal(t, uuid.Nil, completion.BindingID)
	require.NoError(t, store.RecordPlayerBindingCompletionAttempt(ctx, operation.OperationID))
	require.NoError(t, store.CompleteFailedPlayerBindingExchange(ctx, operation.OperationID, claimID, jti))
	_, err = store.NextPlayerBindingCompletion(ctx)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	input := CreatePlayerBindingInput{ApplicationID: app, EnvironmentID: env, Provider: "google",
		ProviderSubjectDigest: "hmac-sha256-v1:test:" + strings.Repeat("f", 64), AccountID: uuid.New(),
		ActorID: uuid.New(), ProfileID: uuid.New(), DeviceID: uuid.New()}
	failed, err := store.CommitPlayerBindingExchange(ctx, operation.OperationID, requestHash, claimID, jti, input)
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	_, err = store.LoadBindingChallenge(ctx, challenge.ChallengeID)
	require.ErrorIs(t, err, ErrBindingChallengeNotFound)
	var bindings int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM player_bindings WHERE application_id=$1`, app).Scan(&bindings))
	require.Zero(t, bindings, "a terminal denied exchange cannot leave an active or pending binding")
}

func TestPlayerBindingChallengeCreationReplaysExactOperationAndRejectsSubstitution(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool, Now: func() time.Time { return now }}
	request := BindingChallengeCreate{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Provider: "google",
		RedirectURIHash: strings.Repeat("a", 64), PKCEChallenge: strings.Repeat("p", 43), DeviceKeyID: uuid.New(),
		DeviceKeyThumbprint: strings.Repeat("t", 43), OperationID: uuid.New(), ExpiresAt: now.Add(4 * time.Minute),
		SourceAccountID: uuid.New(), SourceActorID: uuid.New(), SourceDeviceID: uuid.Nil, SourceGeneration: 4,
		TargetAccountID: uuid.New(), TargetProfileID: uuid.New(), ProfileRevision: 5, ConsentRevision: 6,
		PolicyRevision: 7, Scopes: []string{"game.chat.read", "game.chat.send"}}
	request.SourceDeviceID = request.DeviceKeyID
	requestHash := strings.Repeat("b", 64)
	first, err := store.CreateBindingChallengeForAuth(ctx, request, requestHash, uuid.New(), strings.Repeat("n", 43))
	require.NoError(t, err)
	retry, err := store.CreateBindingChallengeForAuth(ctx, request, requestHash, uuid.New(), strings.Repeat("m", 43))
	require.NoError(t, err)
	require.Equal(t, first, retry, "operation retry returns original id and nonce")
	loaded, err := store.LoadBindingChallenge(ctx, first.ChallengeID)
	require.NoError(t, err)
	require.Equal(t, first, loaded)

	changed := request
	changed.TargetProfileID = uuid.New()
	_, err = store.CreateBindingChallengeForAuth(ctx, changed, requestHash, uuid.New(), strings.Repeat("x", 43))
	require.ErrorIs(t, err, ErrInvalidBindingChallenge)
}

func activeBindingClaims(app, env uuid.UUID) GameDeviceAuthorityClaims {
	issued := time.Now().UTC().Truncate(time.Millisecond)
	claims := GameDeviceAuthorityClaims{Issuer: "auth", Audience: "voice.game-message", Version: 1,
		ApplicationID: app, EnvironmentID: env, AccountID: uuid.New(), ActorID: uuid.New(), BindingID: uuid.New(),
		DeviceID: uuid.New(), KeyID: uuid.New(), DeviceGeneration: 1, AuthorityRevision: 1,
		AssertionID: uuid.New(), Status: "active", NotAfter: issued.Add(4 * time.Second), IssuedAt: issued,
		ExpiresAt: issued.Add(4 * time.Second)}
	claims.AssertionSHA256 = sha256.Sum256([]byte(claims.AssertionID.String()))
	return claims
}

func createBindingTestEnvironment(t *testing.T, ctx context.Context, store *Store) (uuid.UUID, uuid.UUID) {
	t.Helper()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: uuid.New(), Name: "Binding Test", IdempotencyKey: "create-binding-app-" + uuid.NewString(),
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "approve-binding-env-" + uuid.NewString(),
	})
	require.NoError(t, err)
	return app.ID, env.ID
}
