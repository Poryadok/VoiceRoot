package controlledgame

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// This black-box callback contract expects exported OpenPostgresStore and
// NewHandler seams. The configured effect writes only test state, through the
// transaction the receiver uses for inbox admission and its receipt.
func TestCallbackDurableAcceptanceReplayAndBodyConflictAcrossRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_test_db", "")
	_, err := pool.Exec(ctx, `CREATE TABLE controlled_game_test_effects (
		invocation_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		command_id text NOT NULL
	)`)
	require.NoError(t, err)
	var expected struct {
		CommandID string `json:"command_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(canonicalCommand), &expected))
	require.Equal(t, "00000000-0000-4000-8000-000000000001", expected.CommandID)

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	now := time.Unix(1790500000, 0).UTC()
	effect := func(ctx context.Context, tx pgx.Tx, body []byte) ([]byte, error) {
		var command struct {
			CommandID string `json:"command_id"`
		}
		if err := json.Unmarshal(body, &command); err != nil {
			return nil, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO controlled_game_test_effects(command_id) VALUES ($1)`, command.CommandID)
		return []byte(`{"status":"succeeded"}`), err
	}
	open := func() http.Handler {
		store, err := OpenPostgresStore(ctx, pool, func() time.Time { return now })
		require.NoError(t, err)
		return NewHandler(HandlerConfig{
			Store: store,
			Keys:  map[string][]byte{vectorKeyID: key},
			Clock: func() time.Time { return now },
			Apply: effect,
		})
	}

	server := httptest.NewServer(open())
	first := postCanonicalCommand(t, server.Client(), server.URL, []byte(canonicalCommand), key)
	require.Equal(t, http.StatusAccepted, first.status, "202 means inbox transaction committed")
	require.NotEmpty(t, first.body, "accepted callback returns its durable receipt")
	server.Close()

	// A newly constructed handler over the same PostgreSQL database must return
	// the stored receipt for a same-ID/same-body retry without applying twice.
	server = httptest.NewServer(open())
	defer server.Close()
	replay := postCanonicalCommand(t, server.Client(), server.URL, []byte(canonicalCommand), key)
	require.Equal(t, http.StatusAccepted, replay.status)
	require.Equal(t, first.body, replay.body, "replay returns the exact original receipt")
	var effects int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM controlled_game_test_effects`).Scan(&effects))
	require.Equal(t, 1, effects, "same command retry must not repeat the effect")
	var appliedCommandID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT command_id FROM controlled_game_test_effects`).Scan(&appliedCommandID))
	require.Equal(t, expected.CommandID, appliedCommandID, "effect must use the command ID from the signed envelope")

	changedBody := strings.Replace(canonicalCommand, `"encounter-42"`, `"encounter-43"`, 1)
	conflict := postCanonicalCommand(t, server.Client(), server.URL, []byte(changedBody), key)
	require.Equal(t, http.StatusConflict, conflict.status, "same command ID with different canonical bytes conflicts")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM controlled_game_test_effects`).Scan(&effects))
	require.Equal(t, 1, effects, "conflicting body must not apply an effect")
}

func TestCallbackRejectsDifferentCommandIDsForSameOneShotEffect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_effect_dedupe_test_db", "")
	createInvocationTable(t, ctx, pool)
	key := testKey()
	now := time.Unix(1790500000, 0).UTC()
	server := httptest.NewServer(openCallbackHandler(t, ctx, pool, key, now, recordInvocation([]byte(`{"status":"succeeded","effect":"canonical"}`), nil)))
	defer server.Close()

	firstBody := []byte(canonicalCommand)
	secondBody := []byte(strings.Replace(canonicalCommand, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-00000000000c", 1))
	first := postCanonicalCommand(t, server.Client(), server.URL, firstBody, key)
	second := postCanonicalCommand(t, server.Client(), server.URL, secondBody, key)
	require.Equal(t, http.StatusAccepted, first.status)
	require.Equal(t, http.StatusConflict, second.status, "one operation cannot be rebound to a new command ID")
	assertRecordedInvocations(t, ctx, pool, 1)
}

func TestCallbackRollbackAndLostAckRecoverAcrossHandlerRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_rollback_test_db", "")
	createInvocationTable(t, ctx, pool)
	key := testKey()
	now := time.Unix(1790500000, 0).UTC()
	var failOnce atomic.Bool
	failOnce.Store(true)
	effect := recordInvocation([]byte(`{"status":"succeeded"}`), &failOnce)
	body := []byte(canonicalCommand)

	server := httptest.NewServer(openCallbackHandler(t, ctx, pool, key, now, effect))
	failed := postCanonicalCommand(t, server.Client(), server.URL, body, key)
	require.GreaterOrEqual(t, failed.status, http.StatusInternalServerError, "failed transaction must not be ACKed")
	assertRecordedInvocations(t, ctx, pool, 0)
	assertNoDurableCommandRows(t, ctx, pool)
	server.Close()

	// Commit succeeds at the receiver, but this transport drops the HTTP ACK.
	server = httptest.NewServer(openCallbackHandler(t, ctx, pool, key, now, effect))
	dropper := &loseAcknowledgement{delegate: server.Client().Transport}
	client := &http.Client{Transport: dropper, Timeout: 10 * time.Second}
	requestCtx, requestCancel := context.WithTimeout(ctx, 10*time.Second)
	defer requestCancel()
	request, err := newSignedRequest(requestCtx, server.URL, body, key)
	require.NoError(t, err)
	response, err := client.Do(request)
	if response != nil {
		response.Body.Close()
	}
	require.Error(t, err, "test transport must drop the committed response")
	require.Equal(t, http.StatusAccepted, dropper.status, "the receiver committed before the transport lost its ACK")
	require.NotEmpty(t, dropper.body, "the lost ACK contained the committed receipt")
	lostAckBody := string(dropper.body)
	server.Close()
	assertRecordedInvocations(t, ctx, pool, 1)

	// A reconstructed receiver sees the durable commit and returns the exact
	// stored result without invoking the effect again.
	server = httptest.NewServer(openCallbackHandler(t, ctx, pool, key, now, effect))
	defer server.Close()
	recovered := postCanonicalCommand(t, server.Client(), server.URL, body, key)
	require.Equal(t, http.StatusAccepted, recovered.status)
	require.Equal(t, lostAckBody, recovered.body, "restart returns the exact receipt whose ACK was lost")
	assertRecordedInvocations(t, ctx, pool, 1)
}

func TestCallbackConcurrentDuplicateDeliveryAppliesEffectOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_concurrent_delivery_test_db", "")
	createInvocationTable(t, ctx, pool)
	key := testKey()
	now := time.Unix(1790500000, 0).UTC()
	server := httptest.NewServer(openCallbackHandler(t, ctx, pool, key, now, recordInvocation([]byte(`{"status":"succeeded"}`), nil)))
	defer server.Close()

	const deliveries = 8
	start := make(chan struct{})
	ready := make(chan struct{}, deliveries)
	responses := make([]callbackResponse, deliveries)
	errs := make([]error, deliveries)
	var workers sync.WaitGroup
	workers.Add(deliveries)
	for i := range responses {
		go func(i int) {
			defer workers.Done()
			ready <- struct{}{}
			<-start
			client := &http.Client{Transport: server.Client().Transport, Timeout: 10 * time.Second}
			responses[i], errs[i] = sendCanonicalCommand(client, server.URL, []byte(canonicalCommand), key)
		}(i)
	}
	for range deliveries {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			t.Fatal("duplicate delivery workers did not reach the start barrier")
		}
	}
	close(start)
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("duplicate delivery requests did not complete before their deadlines")
	}
	for i, response := range responses {
		require.NoError(t, errs[i])
		require.Equal(t, http.StatusAccepted, response.status)
		require.Equal(t, responses[0].body, response.body, "serialized duplicates return one receipt")
	}
	assertRecordedInvocations(t, ctx, pool, 1)
}

type callbackResponse struct {
	status int
	body   string
}

func postCanonicalCommand(t *testing.T, client *http.Client, endpoint string, body, key []byte) callbackResponse {
	t.Helper()
	response, err := sendCanonicalCommand(client, endpoint, body, key)
	require.NoError(t, err)
	return response
}

func sendCanonicalCommand(client *http.Client, endpoint string, body, key []byte) (callbackResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request, err := newSignedRequest(ctx, endpoint, body, key)
	if err != nil {
		return callbackResponse{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return callbackResponse{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return callbackResponse{}, err
	}
	return callbackResponse{status: response.StatusCode, body: string(responseBody)}, nil
}

func newSignedRequest(ctx context.Context, endpoint string, body, key []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+vectorPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/vnd.voice.game-command+json;version=1")
	request.Header.Set("X-Voice-Key-Id", vectorKeyID)
	request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
	request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath, vectorTimestamp, vectorKeyID, body))
	return request, nil
}

func openCallbackHandler(t *testing.T, ctx context.Context, pool *pgxpool.Pool, key []byte, now time.Time, apply func(context.Context, pgx.Tx, []byte) ([]byte, error)) http.Handler {
	t.Helper()
	store, err := OpenPostgresStore(ctx, pool, func() time.Time { return now })
	require.NoError(t, err)
	return NewHandler(HandlerConfig{
		Store: store,
		Keys:  map[string][]byte{vectorKeyID: key},
		Clock: func() time.Time { return now },
		Apply: apply,
	})
}

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func createInvocationTable(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `CREATE TABLE controlled_game_test_effects (
		invocation_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		command_id text NOT NULL
	)`)
	require.NoError(t, err)
}

func recordInvocation(result []byte, failOnce *atomic.Bool) func(context.Context, pgx.Tx, []byte) ([]byte, error) {
	return func(ctx context.Context, tx pgx.Tx, body []byte) ([]byte, error) {
		var command struct {
			CommandID string `json:"command_id"`
		}
		if err := json.Unmarshal(body, &command); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO controlled_game_test_effects(command_id) VALUES ($1)`, command.CommandID); err != nil {
			return nil, err
		}
		if failOnce != nil && failOnce.CompareAndSwap(true, false) {
			return nil, errors.New("injected transaction failure after test effect")
		}
		return result, nil
	}
}

func assertRecordedInvocations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, expected int) {
	t.Helper()
	var got int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM controlled_game_test_effects`).Scan(&got))
	require.Equal(t, expected, got, "each effect application must leave an append-only invocation row")
}

func assertNoDurableCommandRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	// These receiver-owned tables are the atomic HTTP acceptance transaction.
	for _, table := range []string{"command_inbox", "effect_ledger", "command_results", "result_outbox"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s`, table)).Scan(&count))
		require.Zero(t, count, "%s must roll back with the failed callback", table)
	}
}

type loseAcknowledgement struct {
	delegate http.RoundTripper
	status   int
	body     []byte
}

func (t *loseAcknowledgement) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.delegate.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	t.status = response.StatusCode
	t.body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		return nil, err
	}
	return nil, errors.New("simulated lost acknowledgement after receiver commit")
}
