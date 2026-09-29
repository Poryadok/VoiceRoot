package controlledgame

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestSessionEventInboxCommitsWithGameEffectAndDeduplicatesByExactBodyDigest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_session_events_test_db", "")
	store, err := OpenPostgresStore(ctx, pool, func() time.Time { return time.Unix(1790500000, 0).UTC() })
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE controlled_game_session_effects (
		application_id uuid NOT NULL, environment_id uuid NOT NULL, event_id uuid PRIMARY KEY, activation_count integer NOT NULL
	)`)
	require.NoError(t, err)

	applicationID, environmentID, eventID := uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"event_id":"` + eventID.String() + `","application_id":"` + applicationID.String() +
		`","environment_id":"` + environmentID.String() + `","session_id":"` + uuid.NewString() +
		`","operation_id":"` + uuid.NewString() + `","kind":"party","active_at":"2026-09-28T12:00:00Z"}`)
	digest := sha256.Sum256(body)
	apply := func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO controlled_game_session_effects(application_id,environment_id,event_id,activation_count)
			VALUES ($1,$2,$3,1)`, applicationID, environmentID, eventID)
		return err
	}

	first, err := store.ConsumeSessionActiveEvent(ctx, applicationID, environmentID, eventID, body, digest[:], apply)
	require.NoError(t, err)
	require.True(t, first, "first durable inbox admission applies the game effect")
	duplicate, err := store.ConsumeSessionActiveEvent(ctx, applicationID, environmentID, eventID, body, digest[:], apply)
	require.NoError(t, err)
	require.False(t, duplicate, "same app/env/event and exact digest is a no-op")
	changedDigest := append([]byte(nil), digest[:]...)
	changedDigest[0] ^= 0xff
	_, err = store.ConsumeSessionActiveEvent(ctx, applicationID, environmentID, eventID, body, changedDigest, apply)
	require.Error(t, err, "same event identity with another digest is an integrity conflict")

	var effects int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM controlled_game_session_effects`).Scan(&effects))
	require.Equal(t, 1, effects)
	var inboxDigest, inboxBody []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload_sha256,payload_bytes FROM session_event_inbox WHERE application_id=$1 AND environment_id=$2 AND event_id=$3`,
		applicationID, environmentID, eventID).Scan(&inboxDigest, &inboxBody))
	require.Equal(t, body, inboxBody)
	require.Equal(t, digest[:], inboxDigest)
}

func TestSessionEventInboxRollbackLeavesNoEffectOrACKEligibility(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_session_events_rollback_test_db", "")
	store, err := OpenPostgresStore(ctx, pool, func() time.Time { return time.Unix(1790500000, 0).UTC() })
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE controlled_game_session_effects (event_id uuid PRIMARY KEY)`)
	require.NoError(t, err)
	applicationID, environmentID, eventID := uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"event_id":"` + eventID.String() + `"}`)
	digest := sha256.Sum256(body)
	injected := errors.New("failure after test effect write")
	apply := func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO controlled_game_session_effects(event_id) VALUES ($1)`, eventID); err != nil {
			return err
		}
		return injected
	}
	_, err = store.ConsumeSessionActiveEvent(ctx, applicationID, environmentID, eventID, body, digest[:], apply)
	require.ErrorIs(t, err, injected)
	for _, table := range []string{"session_event_inbox", "controlled_game_session_effects"} {
		var rows int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&rows))
		require.Zero(t, rows, "%s must roll back before the consumer can ACK", table)
	}
	// A retry after rollback may apply exactly once and makes the inbox durable
	// before the caller is permitted to send its HTTPS ACK.
	committed, err := store.ConsumeSessionActiveEvent(ctx, applicationID, environmentID, eventID, body, digest[:], func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO controlled_game_session_effects(event_id) VALUES ($1)`, eventID)
		return err
	})
	require.NoError(t, err)
	require.True(t, committed)
	var effects int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM controlled_game_session_effects`).Scan(&effects))
	require.Equal(t, 1, effects)
}

func TestSessionEventInboxConcurrentDuplicateDeliveryAppliesOneEffect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_session_events_concurrent_test_db", "")
	store, err := OpenPostgresStore(ctx, pool, func() time.Time { return time.Unix(1790500000, 0).UTC() })
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE controlled_game_session_effects (event_id uuid PRIMARY KEY)`)
	require.NoError(t, err)
	applicationID, environmentID, eventID := uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"event_id":"` + eventID.String() + `"}`)
	digest := sha256.Sum256(body)
	const deliveries = 8
	start := make(chan struct{})
	results := make(chan bool, deliveries)
	errs := make(chan error, deliveries)
	var workers sync.WaitGroup
	workers.Add(deliveries)
	for range deliveries {
		go func() {
			defer workers.Done()
			<-start
			applied, consumeErr := store.ConsumeSessionActiveEvent(ctx, applicationID, environmentID, eventID, body, digest[:], func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO controlled_game_session_effects(event_id) VALUES ($1)`, eventID)
				return err
			})
			results <- applied
			errs <- consumeErr
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errs)
	appliedCount := 0
	for applied := range results {
		if applied {
			appliedCount++
		}
	}
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, appliedCount, "one consumer owns the first commit; all concurrent same-digest retries are no-ops")
	var effects int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM controlled_game_session_effects`).Scan(&effects))
	require.Equal(t, 1, effects)
}

func TestSessionEventClientPollOnceCommitsReceiverBeforeLostAckAndRetriesIdempotently(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_session_client_test_db", "")
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	store, err := OpenPostgresStore(ctx, pool, func() time.Time { return now })
	require.NoError(t, err)
	applicationID, environmentID, eventID := uuid.New(), uuid.New(), uuid.New()
	body := sessionEventTestBody(eventID, applicationID, environmentID)
	digest := sha256.Sum256(body)
	leaseID := uuid.New()
	ackCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer scoped-test-credential", r.Header.Get("Authorization"))
		switch {
		case r.Method == http.MethodPost && r.URL.Path == sessionEventsClaimPath:
			require.Equal(t, http.NoBody, r.Body)
			writeSessionEventTestClaim(w, body, eventID, digest[:], leaseID, now.Add(30*time.Second))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/session-events/"+eventID.String()+"/ack":
			var inboxRows, effectRows int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_event_inbox WHERE event_id=$1`, eventID).Scan(&inboxRows))
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_activation_effects WHERE event_id=$1`, eventID).Scan(&effectRows))
			require.Equal(t, 1, inboxRows, "receiver inbox must commit before ACK is sent")
			require.Equal(t, 1, effectRows, "receiver effect must commit before ACK is sent")
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			ackCount++
			if ackCount == 1 {
				conn, _, hijackErr := w.(http.Hijacker).Hijack()
				require.NoError(t, hijackErr)
				_ = conn.Close() // GIS commits the ACK, while the client loses its response.
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := &SessionEventClient{BaseURL: server.URL, Credential: "scoped-test-credential", ApplicationID: applicationID,
		EnvironmentID: environmentID, Store: store, HTTPClient: server.Client(), Now: func() time.Time { return now }}

	applied, err := client.PollOnce(ctx)
	require.Error(t, err, "first ACK response is lost after GIS has committed it")
	require.False(t, applied)
	applied, err = client.PollOnce(ctx)
	require.NoError(t, err, "retry claims the same event and replays ACK successfully")
	require.False(t, applied, "the committed receiver inbox makes redelivery a no-op")
	require.Equal(t, 2, ackCount)
	var inboxRows, effectRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_event_inbox WHERE event_id=$1`, eventID).Scan(&inboxRows))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_activation_effects WHERE event_id=$1`, eventID).Scan(&effectRows))
	require.Equal(t, 1, inboxRows)
	require.Equal(t, 1, effectRows)
}

func TestSessionEventClientPollOnceRejectsInvalidClaimBeforeInboxOrAck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_session_client_invalid_test_db", "")
	store, err := OpenPostgresStore(ctx, pool, time.Now)
	require.NoError(t, err)
	applicationID, environmentID := uuid.New(), uuid.New()
	tests := []struct {
		name   string
		mutate func([]byte, uuid.UUID, uuid.UUID, []byte) ([]byte, uuid.UUID, uuid.UUID, []byte)
	}{
		{name: "body digest mismatch", mutate: func(body []byte, eventID, leaseID uuid.UUID, digest []byte) ([]byte, uuid.UUID, uuid.UUID, []byte) {
			changed := append([]byte(nil), digest...)
			changed[0] ^= 1
			return body, eventID, leaseID, changed
		}},
		{name: "foreign application", mutate: func(body []byte, eventID, leaseID uuid.UUID, digest []byte) ([]byte, uuid.UUID, uuid.UUID, []byte) {
			foreignBody := sessionEventTestBody(eventID, uuid.New(), environmentID)
			foreignDigest := sha256.Sum256(foreignBody)
			return foreignBody, eventID, leaseID, foreignDigest[:]
		}},
		{name: "foreign environment", mutate: func(body []byte, eventID, leaseID uuid.UUID, digest []byte) ([]byte, uuid.UUID, uuid.UUID, []byte) {
			foreignBody := sessionEventTestBody(eventID, applicationID, uuid.New())
			foreignDigest := sha256.Sum256(foreignBody)
			return foreignBody, eventID, leaseID, foreignDigest[:]
		}},
		{name: "event id header mismatch", mutate: func(body []byte, eventID, leaseID uuid.UUID, digest []byte) ([]byte, uuid.UUID, uuid.UUID, []byte) {
			return body, uuid.New(), leaseID, digest
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			eventID, leaseID := uuid.New(), uuid.New()
			body := sessionEventTestBody(eventID, applicationID, environmentID)
			digest := sha256.Sum256(body)
			body, headerEventID, leaseID, headerDigest := test.mutate(body, eventID, leaseID, digest[:])
			ackCount := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer scoped-test-credential", r.Header.Get("Authorization"))
				if r.URL.Path == sessionEventsClaimPath {
					writeSessionEventTestClaim(w, body, headerEventID, headerDigest, leaseID, time.Now().UTC().Add(time.Minute))
					return
				}
				ackCount++
				http.Error(w, "unexpected ACK", http.StatusInternalServerError)
			}))
			defer server.Close()
			client := &SessionEventClient{BaseURL: server.URL, Credential: "scoped-test-credential", ApplicationID: applicationID,
				EnvironmentID: environmentID, Store: store, HTTPClient: server.Client()}
			_, err := client.PollOnce(ctx)
			require.Error(t, err)
			require.Zero(t, ackCount, "invalid scope/body/digest/header must be rejected before ACK")
			var inboxRows int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_event_inbox WHERE event_id=$1`, eventID).Scan(&inboxRows))
			require.Zero(t, inboxRows, "invalid claim must not enter the receiver inbox")
		})
	}
}

func TestSessionEventClientPollOnceReportsClaimAndLeaseFailuresWithoutAck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := integrationtest.StartPostgres(t, ctx, "controlled_game_session_client_failure_test_db", "")
	store, err := OpenPostgresStore(ctx, pool, time.Now)
	require.NoError(t, err)
	tests := []struct {
		name   string
		status int
		expiry func() time.Time
	}{
		{name: "claim server error", status: http.StatusServiceUnavailable},
		{name: "expired lease", status: http.StatusOK, expiry: func() time.Time { return time.Now().UTC().Add(-time.Second) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			applicationID, environmentID, eventID, leaseID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			body := sessionEventTestBody(eventID, applicationID, environmentID)
			digest := sha256.Sum256(body)
			ackCount := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != sessionEventsClaimPath {
					ackCount++
					return
				}
				if test.status != http.StatusOK {
					http.Error(w, "GIS unavailable", test.status)
					return
				}
				expiresAt := time.Now().UTC().Add(time.Minute)
				if test.expiry != nil {
					expiresAt = test.expiry()
				}
				writeSessionEventTestClaim(w, body, eventID, digest[:], leaseID, expiresAt)
			}))
			defer server.Close()
			client := &SessionEventClient{BaseURL: server.URL, Credential: "scoped-test-credential", ApplicationID: applicationID,
				EnvironmentID: environmentID, Store: store, HTTPClient: server.Client()}
			_, err := client.PollOnce(ctx)
			require.Error(t, err)
			require.Zero(t, ackCount)
			var inboxRows int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_event_inbox WHERE event_id=$1`, eventID).Scan(&inboxRows))
			require.Zero(t, inboxRows)
		})
	}
}

func sessionEventTestBody(eventID, applicationID, environmentID uuid.UUID) []byte {
	return []byte(fmt.Sprintf(`{"event_id":"%s","application_id":"%s","environment_id":"%s","session_id":"%s","operation_id":"%s","kind":"party","active_at":"2026-09-28T12:00:00Z"}`,
		eventID, applicationID, environmentID, uuid.New(), uuid.New()))
}

func writeSessionEventTestClaim(w http.ResponseWriter, body []byte, eventID uuid.UUID, digest []byte, leaseID uuid.UUID, expiry time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "identity")
	w.Header().Set("X-Voice-Event-Id", eventID.String())
	w.Header().Set("X-Voice-Payload-SHA256", hex.EncodeToString(digest))
	w.Header().Set("X-Voice-Claim-Lease-Id", leaseID.String())
	w.Header().Set("X-Voice-Claim-Lease-Expires-At", expiry.UTC().Format(time.RFC3339))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
