package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func createActiveSessionEventForDeliveryTest(t *testing.T, ctx context.Context, store *Store, p SessionPrincipal, key string) uuid.UUID {
	t.Helper()
	owners := newSessionOwnerScript()
	orchestrator := NewSessionOrchestrator(store, owners.adapters())
	created, err := orchestrator.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: key, DisplayName: "Raid",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, orchestrator, p, created.OperationID, "active")
	return created.SessionID
}

func TestSessionEventOutboxPersistsTheSevenFieldBodyBytesAndDigest(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	sessionID := createActiveSessionEventForDeliveryTest(t, ctx, store, p, "delivery-exact-bytes")

	var eventID, applicationID, environmentID, savedSessionID, operationID uuid.UUID
	var kind string
	var body, digest []byte
	err := store.Pool.QueryRow(ctx, `SELECT o.event_id,o.application_id,o.environment_id,o.session_id,
		op.operation_id,s.kind,o.payload_bytes,o.payload_sha256
		FROM gis_session_outbox o
		JOIN gis_sessions s ON s.id=o.session_id
		JOIN gis_session_operations op ON op.session_id=s.id AND op.operation_kind='create'
		WHERE o.session_id=$1`, sessionID).Scan(
		&eventID, &applicationID, &environmentID, &savedSessionID, &operationID, &kind, &body, &digest)
	require.NoError(t, err)
	require.Equal(t, p.ApplicationID, applicationID)
	require.Equal(t, p.EnvironmentID, environmentID)
	require.Equal(t, sessionID, savedSessionID)
	require.Equal(t, "party", kind)
	bodyDigest := sha256.Sum256(body)
	require.Equal(t, bodyDigest[:], digest)
	require.Len(t, digest, sha256.Size)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Len(t, decoded, 7, "active event must have exactly the frozen seven fields")
	require.Equal(t, eventID.String(), decoded["event_id"])
	require.Equal(t, p.ApplicationID.String(), decoded["application_id"])
	require.Equal(t, p.EnvironmentID.String(), decoded["environment_id"])
	require.Equal(t, sessionID.String(), decoded["session_id"])
	require.Equal(t, operationID.String(), decoded["operation_id"])
	require.Equal(t, "party", decoded["kind"])
	require.NotEmpty(t, decoded["active_at"])

	claim, err := store.ClaimSessionEvent(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, eventID, claim.EventID)
	require.Equal(t, body, claim.PayloadBytes, "delivery returns the persisted byte slice without re-encoding JSON")
	require.Equal(t, digest, claim.PayloadSHA256)
	require.Equal(t, hex.EncodeToString(digest), hex.EncodeToString(claim.PayloadSHA256))
}

func TestSessionEventClaimIsAppEnvironmentScopedAndConcurrentClaimsAreExclusive(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	appA, appB := uuid.New(), uuid.New()
	envA, envB := uuid.New(), uuid.New()
	pA := SessionPrincipal{ApplicationID: appA, EnvironmentID: envA, Scopes: []string{"game.sessions.manage"}}
	pB := SessionPrincipal{ApplicationID: appB, EnvironmentID: envB, Scopes: []string{"game.sessions.manage"}}
	sessionA := createActiveSessionEventForDeliveryTest(t, ctx, store, pA, "claim-a")
	secondSessionA := createActiveSessionEventForDeliveryTest(t, ctx, store, pA, "claim-a-second")
	sessionB := createActiveSessionEventForDeliveryTest(t, ctx, store, pB, "claim-b")

	claimA, err := store.ClaimSessionEvent(ctx, pA)
	require.NoError(t, err)
	require.NotNil(t, claimA)
	require.Equal(t, sessionA, claimA.SessionID)
	claimB, err := store.ClaimSessionEvent(ctx, pB)
	require.NoError(t, err)
	require.NotNil(t, claimB)
	require.Equal(t, sessionB, claimB.SessionID)

	// Keep one app's event available and race multiple consumers in the same
	// scope. Exactly one may reserve the row while its 30-second lease is live.
	var claims sync.WaitGroup
	start := make(chan struct{})
	got := make(chan *SessionEventClaim, 8)
	errs := make(chan error, 8)
	for i := 0; i < cap(got); i++ {
		claims.Add(1)
		go func() {
			defer claims.Done()
			<-start
			value, claimErr := store.ClaimSessionEvent(ctx, pA)
			got <- value
			errs <- claimErr
		}()
	}
	close(start)
	claims.Wait()
	close(got)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	reserved := 0
	for value := range got {
		if value != nil {
			reserved++
			require.Equal(t, secondSessionA, value.SessionID)
			require.NotEqual(t, claimA.EventID, value.EventID)
		}
	}
	require.Equal(t, 1, reserved, "the remaining event must be leased once across concurrent consumers")
}

func TestSessionEventLeaseExpiryReclaimsIdenticalBytesWithNewLeaseAndACKIsDigestBound(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	sessionID := createActiveSessionEventForDeliveryTest(t, ctx, store, p, "claim-expiry")
	first, err := store.ClaimSessionEvent(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.WithinDuration(t, time.Now().Add(30*time.Second), first.LeaseExpiresAt, 10*time.Second)

	wrongDigest := append([]byte(nil), first.PayloadSHA256...)
	wrongDigest[0] ^= 0xff
	err = store.AckSessionEvent(ctx, p, first.EventID, first.LeaseID, wrongDigest)
	require.Error(t, err, "digest mismatch must not mark the outbox event delivered")
	var delivered *time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.Nil(t, delivered)

	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_outbox SET claim_lease_until=clock_timestamp()-interval '1 second' WHERE event_id=$1`, first.EventID)
	require.NoError(t, err)
	second, err := store.ClaimSessionEvent(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.EventID, second.EventID)
	require.Equal(t, first.PayloadBytes, second.PayloadBytes)
	require.Equal(t, first.PayloadSHA256, second.PayloadSHA256)
	require.NotEqual(t, first.LeaseID, second.LeaseID)

	staleErr := store.AckSessionEvent(ctx, p, first.EventID, first.LeaseID, first.PayloadSHA256)
	require.Error(t, staleErr, "superseded lease cannot ACK after a newer claim")
	wrongApplication := p
	wrongApplication.ApplicationID = uuid.New()
	require.Error(t, store.AckSessionEvent(ctx, wrongApplication, second.EventID, second.LeaseID, second.PayloadSHA256),
		"another application cannot ACK this event")
	wrongEnvironment := p
	wrongEnvironment.EnvironmentID = uuid.New()
	require.Error(t, store.AckSessionEvent(ctx, wrongEnvironment, second.EventID, second.LeaseID, second.PayloadSHA256),
		"another environment cannot ACK this event")
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.Nil(t, delivered)

	require.NoError(t, store.AckSessionEvent(ctx, p, second.EventID, second.LeaseID, second.PayloadSHA256))
	require.NoError(t, store.AckSessionEvent(ctx, p, second.EventID, second.LeaseID, second.PayloadSHA256), "retry after lost response is idempotent")
	require.Error(t, store.AckSessionEvent(ctx, p, second.EventID, second.LeaseID, wrongDigest), "changed ACK after success conflicts")
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.NotNil(t, delivered)
}

func TestSessionEventReceiverCommitLostAckExpiryReclaimAndNewLeaseAck(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	sessionID := createActiveSessionEventForDeliveryTest(t, ctx, store, p, "receiver-ack-reclaim")
	receiverPool := integrationtest.StartPostgres(t, ctx, "controlled_game_t31_inbox_test_db", "")
	_, err := receiverPool.Exec(ctx, `CREATE TABLE session_event_inbox (
		application_id uuid NOT NULL, environment_id uuid NOT NULL, event_id uuid NOT NULL,
		payload_sha256 bytea NOT NULL CHECK (octet_length(payload_sha256)=32), payload_bytes bytea NOT NULL,
		PRIMARY KEY(application_id,environment_id,event_id)
	);
	CREATE TABLE session_event_game_effects(event_id uuid PRIMARY KEY)`)
	require.NoError(t, err)

	first, err := store.ClaimSessionEvent(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, sessionID, first.SessionID)

	applyReceiver := func(claim *SessionEventClaim, failAfterEffect bool) (bool, error) {
		tx, err := receiverPool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return false, err
		}
		defer tx.Rollback(ctx)
		insert, err := tx.Exec(ctx, `INSERT INTO session_event_inbox(application_id,environment_id,event_id,payload_sha256,payload_bytes)
			VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, p.ApplicationID, p.EnvironmentID, claim.EventID, claim.PayloadSHA256, claim.PayloadBytes)
		if err != nil {
			return false, err
		}
		if insert.RowsAffected() == 0 {
			var savedDigest []byte
			if err := tx.QueryRow(ctx, `SELECT payload_sha256 FROM session_event_inbox WHERE application_id=$1 AND environment_id=$2 AND event_id=$3 FOR SHARE`,
				p.ApplicationID, p.EnvironmentID, claim.EventID).Scan(&savedDigest); err != nil {
				return false, err
			}
			if !equalBytes(savedDigest, claim.PayloadSHA256) {
				return false, errors.New("receiver inbox digest conflict")
			}
			return false, nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO session_event_game_effects(event_id) VALUES ($1)`, claim.EventID); err != nil {
			return false, err
		}
		if failAfterEffect {
			return false, errors.New("injected receiver rollback before commit")
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}

	// The effect fails after its tentative write. The receiver transaction rolls
	// back both inbox and effect, and GIS must remain undelivered because there
	// has not been an ACK.
	_, err = applyReceiver(first, true)
	require.Error(t, err)
	for _, table := range []string{"session_event_inbox", "session_event_game_effects"} {
		var rows int
		require.NoError(t, receiverPool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&rows))
		require.Zero(t, rows, "receiver transaction rollback must remove %s", table)
	}
	var delivered *time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.Nil(t, delivered, "receiver rollback must not produce an ACK or mark delivery")

	committed, err := applyReceiver(first, false)
	require.NoError(t, err)
	require.True(t, committed)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.Nil(t, delivered, "receiver DB commit alone does not mark GIS delivery")

	// The lease can expire between receiver commit and GIS ACK admission. GIS
	// rejects the old lease; simulate the HTTP response being lost so the
	// receiver recovers through expiry/reclaim instead of assuming success.
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_outbox SET claim_lease_until=clock_timestamp()-interval '1 second' WHERE event_id=$1`, first.EventID)
	require.NoError(t, err)
	ackClientErr := func() error {
		ackErr := store.AckSessionEvent(ctx, p, first.EventID, first.LeaseID, first.PayloadSHA256)
		if ackErr == nil {
			return errors.New("test setup error: expired first lease unexpectedly committed")
		}
		return errors.New("simulated dropped HTTP ACK response")
	}
	require.Error(t, ackClientErr(), "receiver must not assume success without the ACK response")
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.Nil(t, delivered, "expired-lease ACK was rejected despite the response loss")
	second, err := store.ClaimSessionEvent(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.EventID, second.EventID)
	require.NotEqual(t, first.LeaseID, second.LeaseID)
	require.Equal(t, first.PayloadBytes, second.PayloadBytes)
	require.Equal(t, first.PayloadSHA256, second.PayloadSHA256)

	duplicate, err := applyReceiver(second, false)
	require.NoError(t, err)
	require.False(t, duplicate, "the independent receiver recognizes same-digest redelivery as a no-op")
	var effects int
	require.NoError(t, receiverPool.QueryRow(ctx, `SELECT count(*) FROM session_event_game_effects`).Scan(&effects))
	require.Equal(t, 1, effects)
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.Nil(t, delivered, "duplicate receiver delivery is not success until the new lease ACK commits")

	require.NoError(t, store.AckSessionEvent(ctx, p, second.EventID, second.LeaseID, second.PayloadSHA256))
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&delivered))
	require.NotNil(t, delivered, "delivery is recorded only after authenticated ACK with the current lease")
}

func TestSessionEventAckCommittedResponseLostRetriesSameAckSuccessfully(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	p := SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	sessionID := createActiveSessionEventForDeliveryTest(t, ctx, store, p, "receiver-lost-successful-ack")
	receiverPool := integrationtest.StartPostgres(t, ctx, "controlled_game_t31_lost_ack_test_db", "")
	_, err := receiverPool.Exec(ctx, `CREATE TABLE session_event_inbox (
		application_id uuid NOT NULL, environment_id uuid NOT NULL, event_id uuid NOT NULL,
		payload_sha256 bytea NOT NULL CHECK (octet_length(payload_sha256)=32), payload_bytes bytea NOT NULL,
		PRIMARY KEY(application_id,environment_id,event_id)
	);
	CREATE TABLE session_event_game_effects(event_id uuid PRIMARY KEY)`)
	require.NoError(t, err)

	claim, err := store.ClaimSessionEvent(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, sessionID, claim.SessionID)

	// Commit the receiver inbox and effect in its own database before any ACK.
	tx, err := receiverPool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO session_event_inbox(application_id,environment_id,event_id,payload_sha256,payload_bytes)
		VALUES ($1,$2,$3,$4,$5)`, p.ApplicationID, p.EnvironmentID, claim.EventID, claim.PayloadSHA256, claim.PayloadBytes)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO session_event_game_effects(event_id) VALUES ($1)`, claim.EventID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	// Model the HTTPS client observing a connection reset after GIS has
	// committed the 200. The next request sends precisely the same ACK body.
	lostResponse := true
	postAck := func() (int, error) {
		if err := store.AckSessionEvent(ctx, p, claim.EventID, claim.LeaseID, claim.PayloadSHA256); err != nil {
			return 409, err
		}
		if lostResponse {
			lostResponse = false
			return 0, errors.New("simulated connection loss after GIS committed HTTP 200")
		}
		return 200, nil
	}
	status, err := postAck()
	require.Error(t, err, "the client loses the first successful HTTP response")
	require.Zero(t, status, "the lost response is not observed by the client")

	var ackLease uuid.UUID
	var ackDigest []byte
	var deliveredAt time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at,consumer_ack_lease_id,consumer_ack_payload_sha256
		FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&deliveredAt, &ackLease, &ackDigest))
	require.NotZero(t, deliveredAt, "GIS committed delivery before the HTTP response was lost")
	require.Equal(t, claim.LeaseID, ackLease)
	require.Equal(t, claim.PayloadSHA256, ackDigest)

	status, err = postAck()
	require.NoError(t, err, "exact ACK replay after lost response is idempotent")
	require.Equal(t, 200, status)
	var replayDeliveredAt time.Time
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT delivered_at FROM gis_session_outbox WHERE session_id=$1`, sessionID).Scan(&replayDeliveredAt))
	require.Equal(t, deliveredAt, replayDeliveredAt, "ACK replay preserves the original delivery commit")

	var inboxRows, effects int
	require.NoError(t, receiverPool.QueryRow(ctx, `SELECT count(*) FROM session_event_inbox`).Scan(&inboxRows))
	require.NoError(t, receiverPool.QueryRow(ctx, `SELECT count(*) FROM session_event_game_effects`).Scan(&effects))
	require.Equal(t, 1, inboxRows)
	require.Equal(t, 1, effects)
}
