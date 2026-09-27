package controlledgame

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestT07aPermitAuthoritySerializesIssueAndRevoke(t *testing.T) {
	now := time.Unix(1790500015, 0).UTC()
	authority := newT07aPermitAuthority(func() time.Time { return now })
	request := t07aPermitRequest(1)

	permit, err := authority.Admit(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, int64(1790500015), permit.PermitIssuedAt)
	require.Equal(t, request.OperationID, permit.PermitID)
	authority.Revoke()

	retry, err := authority.Admit(context.Background(), request)
	require.NoError(t, err, "permit-first preserves only the already issued window")
	require.Equal(t, permit, retry, "retry/lost-response must preserve the same permit epoch")
	_, err = authority.Admit(context.Background(), t07aPermitRequest(2))
	require.ErrorIs(t, err, errPermitDenied, "revoke-first denies new issuance")
}

func TestT07aCallbackPersistsPermitAndImmutableReceiptThenReplays(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_t07a_replay_test_db", "")
	createT07aAuthorizationFixtures(t, ctx, pool)
	createT07aEffectFixtures(t, ctx, pool)

	now := time.Unix(1790500015, 0).UTC() // A first delivery at retry slot t=15 receives a fresh epoch.
	clock := func() time.Time { return now }
	authority := newT07aPermitAuthority(clock)
	body := t07aCommandBody(t, 1, 2, nil)
	seedT07aAuthorization(t, ctx, pool, body)
	var effectCalls int
	apply := func(ctx context.Context, tx pgx.Tx, raw []byte) ([]byte, error) {
		effectCalls++
		var command callbackCommand
		if err := json.Unmarshal(raw, &command); err != nil {
			return nil, err
		}
		var characterID string
		err := tx.QueryRow(ctx, `SELECT character_id FROM controlled_game_test_authorizations
			WHERE app_id=$1 AND environment_id=$2 AND installation_id=$3 AND profile_id=$4
			AND proof_id=$5 AND action_id=$6 AND binding_revision=$7 AND state_version=$8`,
			command.AppID, command.EnvironmentID, command.InstallationID, command.ActorProof.ProfileID,
			command.ActorProof.ProofID, command.ActionID, command.BindingRevision, command.StateVersion).Scan(&characterID)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO controlled_game_test_effects(command_id, character_id) VALUES ($1,$2)`, command.CommandID, characterID); err != nil {
			return nil, err
		}
		return []byte(canonicalResult), nil
	}

	store, err := OpenPostgresStore(ctx, pool, clock)
	require.NoError(t, err)
	handler := NewHandler(HandlerConfig{
		Store:           store,
		PermitAuthority: authority,
		Authorize:       authorizeT07aTestBinding,
		Credentials:     map[string]SigningCredential{vectorKeyID: testCredential(testKey())},
		Clock:           clock,
		Apply:           apply,
	})
	server := httptest.NewServer(handler)
	first := postCanonicalCommand(t, server.Client(), server.URL, body, testKey())
	require.Equal(t, http.StatusAccepted, first.status)
	assertCanonicalResult(t, first.body)
	server.Close()
	require.Equal(t, 1, effectCalls)
	require.Equal(t, 1, authority.Calls())

	var permitID string
	var issuedAt, startBefore, completeBefore int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT permit_id, permit_issued_at, start_before, complete_before FROM execution_permits`).
		Scan(&permitID, &issuedAt, &startBefore, &completeBefore))
	require.Equal(t, "00000000-0000-4000-8000-000000000002", permitID)
	require.Equal(t, int64(1790500015), issuedAt)
	require.Equal(t, issuedAt+10, startBefore)
	require.Equal(t, issuedAt+60, completeBefore)
	var characterID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT character_id FROM controlled_game_test_effects`).Scan(&characterID))
	require.Equal(t, "game-character-7", characterID, "the effect uses the character resolved by the locked game-owned binding")

	for _, statement := range []string{
		`UPDATE command_inbox SET body_bytes=''`, `DELETE FROM command_inbox`,
		`UPDATE execution_permits SET permit_issued_at=0`, `DELETE FROM execution_permits`,
		`UPDATE effect_ledger SET consumed_at=now()`, `DELETE FROM effect_ledger`,
		`UPDATE command_results SET result_body=''`, `DELETE FROM command_results`,
		`UPDATE result_outbox SET result_body=''`, `DELETE FROM result_outbox`,
		`TRUNCATE command_inbox, execution_permits, effect_ledger, command_results, result_outbox`,
	} {
		_, err := pool.Exec(ctx, statement)
		require.Error(t, err, "durable command, permit, effect and result rows are append-only: %s", statement)
	}
	for _, table := range []string{"command_inbox", "execution_permits", "effect_ledger", "command_results", "result_outbox"} {
		var rows int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&rows))
		require.Equal(t, 1, rows, "rejected mutation must preserve the committed %s receipt", table)
	}

	// A committed receipt is replayable after the permit window without asking
	// authority to mint or refresh a permit, and without reapplying the effect.
	now = time.Unix(1790500090, 0).UTC()
	store, err = OpenPostgresStore(ctx, pool, clock)
	require.NoError(t, err)
	restarted := NewHandler(HandlerConfig{
		Store:           store,
		PermitAuthority: authority,
		Authorize:       authorizeT07aTestBinding,
		Credentials:     map[string]SigningCredential{vectorKeyID: testCredential(testKey())},
		Clock:           clock,
		Apply:           apply,
	})
	server = httptest.NewServer(restarted)
	defer server.Close()
	replayed := postCanonicalCommand(t, server.Client(), server.URL, body, testKey())
	require.Equal(t, http.StatusAccepted, replayed.status)
	require.Equal(t, first.body, replayed.body)
	require.Equal(t, 1, authority.Calls())
	require.Equal(t, 1, effectCalls)
}

func TestT07aAuthorizationDenialAndAdmissionFailuresHaveNoEffect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_t07a_denial_test_db", "")
	createT07aAuthorizationFixtures(t, ctx, pool)
	createT07aEffectFixtures(t, ctx, pool)
	now := time.Unix(1790500015, 0).UTC()
	clock := func() time.Time { return now }
	authority := newT07aPermitAuthority(clock)
	store, err := OpenPostgresStore(ctx, pool, clock)
	require.NoError(t, err)
	var effectCalls int
	handler := NewHandler(HandlerConfig{
		Store:           store,
		PermitAuthority: authority,
		Authorize:       authorizeT07aTestBinding,
		Credentials:     map[string]SigningCredential{vectorKeyID: testCredential(testKey())},
		Clock:           clock,
		Apply: func(context.Context, pgx.Tx, []byte) ([]byte, error) {
			effectCalls++
			return []byte(canonicalResult), nil
		},
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	for index, test := range []struct {
		name   string
		mutate func(*callbackCommand)
		status int
	}{
		{name: "profile mismatch", mutate: func(command *callbackCommand) { command.ActorProof.ProfileID = t07aUUID(40) }, status: http.StatusForbidden},
		{name: "proof mismatch", mutate: func(command *callbackCommand) { command.ActorProof.ProofID = "unknown-proof" }, status: http.StatusForbidden},
		{name: "binding revision mismatch", mutate: func(command *callbackCommand) { command.BindingRevision++ }, status: http.StatusForbidden},
		{name: "game state mismatch", mutate: func(command *callbackCommand) { command.StateVersion = "stale-state" }, status: http.StatusForbidden},
		{name: "application mismatch", mutate: func(command *callbackCommand) { command.AppID = t07aUUID(41) }, status: http.StatusUnauthorized},
		{name: "environment mismatch", mutate: func(command *callbackCommand) { command.EnvironmentID = t07aUUID(42) }, status: http.StatusUnauthorized},
		{name: "installation mismatch", mutate: func(command *callbackCommand) { command.InstallationID = t07aUUID(43) }, status: http.StatusUnauthorized},
	} {
		validBody := t07aCommandBody(t, uint64(10+index), uint64(20+index), nil)
		seedT07aAuthorization(t, ctx, pool, validBody)
		invalidBody := t07aCommandBody(t, uint64(10+index), uint64(20+index), test.mutate)
		response := postCanonicalCommand(t, server.Client(), server.URL, invalidBody, testKey())
		require.Equal(t, test.status, response.status, test.name+" must fail closed")
		assertNoDurableCommandRows(t, ctx, pool)
	}
	require.Zero(t, effectCalls)
	require.Equal(t, 4, authority.Calls(), "credential scope rejects app/env/installation mismatches before permit admission")

	// Revoked authority denies new permits; unavailable authority is distinct and
	// also leaves the receiver DB untouched.
	deniedAuthority := newT07aPermitAuthority(clock)
	deniedAuthority.Revoke()
	deniedHandler := newT07aHandler(t, ctx, pool, clock, deniedAuthority, func(context.Context, pgx.Tx, []byte) ([]byte, error) {
		effectCalls++
		return []byte(canonicalResult), nil
	})
	deniedServer := httptest.NewServer(deniedHandler)
	denied := postCanonicalCommand(t, deniedServer.Client(), deniedServer.URL, t07aCommandBody(t, 30, 31, nil), testKey())
	deniedServer.Close()
	require.Equal(t, http.StatusForbidden, denied.status)
	assertNoDurableCommandRows(t, ctx, pool)

	unavailableHandler := newT07aHandler(t, ctx, pool, clock, t07aUnavailableAuthority{}, func(context.Context, pgx.Tx, []byte) ([]byte, error) {
		effectCalls++
		return []byte(canonicalResult), nil
	})
	unavailableServer := httptest.NewServer(unavailableHandler)
	unavailable := postCanonicalCommand(t, unavailableServer.Client(), unavailableServer.URL, t07aCommandBody(t, 32, 33, nil), testKey())
	unavailableServer.Close()
	require.Equal(t, http.StatusServiceUnavailable, unavailable.status)
	assertNoDurableCommandRows(t, ctx, pool)
	require.Zero(t, effectCalls)
}

func TestT07aPermitStartAndCompletionBoundsAreExclusive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_t07a_bounds_test_db", "")
	createT07aAuthorizationFixtures(t, ctx, pool)
	createT07aEffectFixtures(t, ctx, pool)
	base := time.Unix(1790500000, 0).UTC()
	now := base
	clock := func() time.Time { return now }
	authority := newT07aPermitAuthority(clock)
	store, err := OpenPostgresStore(ctx, pool, clock)
	require.NoError(t, err)
	var effectCalls int
	apply := func(ctx context.Context, tx pgx.Tx, raw []byte) ([]byte, error) {
		effectCalls++
		command, err := parseCallbackCommand(raw)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO controlled_game_test_effects(command_id, character_id) VALUES ($1,$2)`, command.CommandID, "game-character-7"); err != nil {
			return nil, err
		}
		return t07aResultForCommand(raw)
	}
	handler := NewHandler(HandlerConfig{
		Store:           store,
		PermitAuthority: authority,
		Authorize:       authorizeT07aTestBinding,
		Credentials:     map[string]SigningCredential{vectorKeyID: testCredential(testKey())},
		Clock:           clock,
		Apply:           apply,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	// A permit minted at base cannot start at its exclusive +10s boundary.
	body := t07aCommandBody(t, 50, 60, nil)
	seedT07aAuthorization(t, ctx, pool, body)
	authority.AfterIssue(func() { now = base.Add(10 * time.Second) })
	response := postCanonicalCommand(t, server.Client(), server.URL, body, testKey())
	require.Equal(t, http.StatusGone, response.status)
	assertNoDurableCommandRows(t, ctx, pool)
	require.Zero(t, effectCalls)

	// Effect work that reaches exactly +60s rolls every receiver row and effect
	// back; completing one millisecond before +60s commits.
	for _, test := range []struct {
		commandID uint64
		elapsed   time.Duration
		want      int
	}{
		{commandID: 51, elapsed: 60 * time.Second, want: http.StatusGone},
		{commandID: 52, elapsed: 60*time.Second - time.Millisecond, want: http.StatusAccepted},
	} {
		now = base
		body := t07aCommandBody(t, test.commandID, test.commandID+10, nil)
		seedT07aAuthorization(t, ctx, pool, body)
		authority.AfterIssue(func() { now = base.Add(9 * time.Second) })
		apply = func(ctx context.Context, tx pgx.Tx, raw []byte) ([]byte, error) {
			effectCalls++
			command, err := parseCallbackCommand(raw)
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO controlled_game_test_effects(command_id, character_id) VALUES ($1,$2)`, command.CommandID, "game-character-7"); err != nil {
				return nil, err
			}
			now = base.Add(test.elapsed)
			return t07aResultForCommand(raw)
		}
		// Recreate the handler to bind this subcase's deterministic Apply.
		handler := newT07aHandler(t, ctx, pool, clock, authority, apply)
		caseServer := httptest.NewServer(handler)
		response := postCanonicalCommand(t, caseServer.Client(), caseServer.URL, body, testKey())
		caseServer.Close()
		require.Equal(t, test.want, response.status)
		if test.want == http.StatusGone {
			assertNoDurableCommandRows(t, ctx, pool)
		} else {
			require.Equal(t, 2, effectCalls, "late attempt executes the transaction callback before the deadline check rolls it back")
			var committedEffects int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM controlled_game_test_effects`).Scan(&committedEffects))
			require.Equal(t, 1, committedEffects, "only the pre-completion-bound effect may commit")
		}
	}
}

func newT07aHandler(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clock func() time.Time, authority PermitAuthority, apply EffectApplier) http.Handler {
	t.Helper()
	store, err := OpenPostgresStore(ctx, pool, clock)
	require.NoError(t, err)
	return NewHandler(HandlerConfig{
		Store:           store,
		PermitAuthority: authority,
		Authorize:       authorizeT07aTestBinding,
		Credentials:     map[string]SigningCredential{vectorKeyID: testCredential(testKey())},
		Clock:           clock,
		Apply:           apply,
	})
}

func newT07aPermitAuthority(clock func() time.Time) *t07aPermitAuthority {
	return &t07aPermitAuthority{clock: clock, permits: make(map[string]t07aPermitRecord)}
}

type t07aPermitRecord struct {
	request PermitRequest
	permit  ExecutionPermit
}

type t07aPermitAuthority struct {
	mu         sync.Mutex
	clock      func() time.Time
	revoked    bool
	calls      int
	permits    map[string]t07aPermitRecord
	afterIssue func()
}

func (authority *t07aPermitAuthority) Admit(_ context.Context, request PermitRequest) (ExecutionPermit, error) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.calls++
	if record, exists := authority.permits[request.CommandID]; exists {
		if record.request != request {
			return ExecutionPermit{}, errCommandConflict
		}
		return record.permit, nil
	}
	if authority.revoked {
		return ExecutionPermit{}, errPermitDenied
	}
	permit := ExecutionPermit{PermitID: request.OperationID, PermitIssuedAt: authority.clock().UTC().Unix()}
	authority.permits[request.CommandID] = t07aPermitRecord{request: request, permit: permit}
	if authority.afterIssue != nil {
		authority.afterIssue()
	}
	return permit, nil
}

func (authority *t07aPermitAuthority) Revoke() {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.revoked = true
}

func (authority *t07aPermitAuthority) Calls() int {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	return authority.calls
}

func (authority *t07aPermitAuthority) AfterIssue(action func()) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.afterIssue = action
}

type t07aUnavailableAuthority struct{}

func (t07aUnavailableAuthority) Admit(context.Context, PermitRequest) (ExecutionPermit, error) {
	return ExecutionPermit{}, errPermitUnavailable
}

func createT07aAuthorizationFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `CREATE TABLE controlled_game_test_authorizations (
		app_id UUID NOT NULL,
		environment_id UUID NOT NULL,
		installation_id UUID NOT NULL,
		profile_id UUID NOT NULL,
		proof_id TEXT NOT NULL,
		action_id UUID NOT NULL,
		binding_revision BIGINT NOT NULL,
		state_version TEXT NOT NULL,
		character_id TEXT NOT NULL,
		PRIMARY KEY (app_id, environment_id, installation_id, profile_id, proof_id, action_id, binding_revision, state_version)
	)`)
	require.NoError(t, err)
}

func createT07aEffectFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `CREATE TABLE controlled_game_test_effects (
		command_id UUID PRIMARY KEY,
		character_id TEXT NOT NULL
	)`)
	require.NoError(t, err)
}

func seedT07aAuthorization(t *testing.T, ctx context.Context, pool *pgxpool.Pool, body []byte) {
	t.Helper()
	command, err := parseCallbackCommand(body)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO controlled_game_test_authorizations
		(app_id, environment_id, installation_id, profile_id, proof_id, action_id, binding_revision, state_version, character_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, command.AppID, command.EnvironmentID, command.InstallationID,
		command.ActorProof.ProfileID, command.ActorProof.ProofID, command.ActionID, command.BindingRevision,
		command.StateVersion, "game-character-7")
	require.NoError(t, err)
}

func authorizeT07aTestBinding(ctx context.Context, tx pgx.Tx, command callbackCommand) error {
	var characterID string
	err := tx.QueryRow(ctx, `SELECT character_id FROM controlled_game_test_authorizations
		WHERE app_id=$1 AND environment_id=$2 AND installation_id=$3 AND profile_id=$4
		AND proof_id=$5 AND action_id=$6 AND binding_revision=$7 AND state_version=$8 FOR SHARE`,
		command.AppID, command.EnvironmentID, command.InstallationID, command.ActorProof.ProfileID,
		command.ActorProof.ProofID, command.ActionID, command.BindingRevision, command.StateVersion).Scan(&characterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errAuthorizationDenied
	}
	return err
}

// allowT07aTestBinding keeps pre-T07a black-box callback fixtures explicit:
// those tests isolate transport/receipt behavior and do not model game state.
func allowT07aTestBinding(context.Context, pgx.Tx, callbackCommand) error { return nil }

func t07aPermitRequest(index uint64) PermitRequest {
	commandID := t07aUUID(index)
	return PermitRequest{
		CommandID: commandID, OperationID: t07aUUID(index + 100), InvocationID: t07aUUID(index + 200),
		ActionID: "00000000-0000-4000-8000-000000000004", MessageID: "00000000-0000-4000-8000-00000000000b",
		AppID: "00000000-0000-4000-8000-000000000005", EnvironmentID: "00000000-0000-4000-8000-000000000006",
		InstallationID: "00000000-0000-4000-8000-000000000007", ProfileID: "00000000-0000-4000-8000-000000000008",
		ProofID: "opaque", BindingRevision: 8,
	}
}

func t07aCommandBody(t *testing.T, commandID, operationID uint64, mutate func(*callbackCommand)) []byte {
	t.Helper()
	var command callbackCommand
	require.NoError(t, json.Unmarshal([]byte(canonicalCommand), &command))
	command.CommandID = t07aUUID(commandID)
	command.OperationID = t07aUUID(operationID)
	command.InvocationID = t07aUUID(operationID + 100)
	if mutate != nil {
		mutate(&command)
	}
	encoded, err := json.Marshal(command)
	require.NoError(t, err)
	canonical, err := canonicalJSON(encoded)
	require.NoError(t, err)
	return canonical
}

func t07aResultForCommand(body []byte) ([]byte, error) {
	var command callbackCommand
	if err := json.Unmarshal(body, &command); err != nil {
		return nil, err
	}
	result := callbackResult{
		CommandID: command.CommandID, OperationID: command.OperationID, ResultID: command.InvocationID,
		SchemaVersion: 1, StateVersion: command.StateVersion + ":applied", Status: "succeeded", Summary: "Applied",
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return canonicalJSON(encoded)
}

func t07aUUID(value uint64) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012x", value)
}
