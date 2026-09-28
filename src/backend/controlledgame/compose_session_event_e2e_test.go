package controlledgame

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestT31ComposeHTTPSClaimInboxEffectAckReclaim is an opt-in acceptance test.
// It talks through the test TLS ingress, real Compose Gateway, and GIS, while
// writing receiver inbox/effect rows only to the dedicated controlledgame DB.
func TestT31ComposeHTTPSClaimInboxEffectAckReclaim(t *testing.T) {
	if os.Getenv("VOICE_T31_COMPOSE_SESSION_EVENTS") != "1" {
		t.Skip("set VOICE_T31_COMPOSE_SESSION_EVENTS=1 for the isolated Compose acceptance")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	databaseURL := requireT31Env(t, "CONTROLLEDGAME_DATABASE_URL")
	baseURL := requireT31Env(t, "T31_GIS_HTTPS_BASE_URL")
	caPath := requireT31Env(t, "T31_GATEWAY_CA_FILE")
	applicationID := uuid.MustParse(requireT31Env(t, "T31_APPLICATION_ID"))
	environmentID := uuid.MustParse(requireT31Env(t, "T31_ENVIRONMENT_ID"))
	otherApplicationID := uuid.MustParse(requireT31Env(t, "T31_OTHER_APPLICATION_ID"))
	otherEnvironmentID := uuid.MustParse(requireT31Env(t, "T31_OTHER_ENVIRONMENT_ID"))
	eventID := uuid.MustParse(requireT31Env(t, "T31_EXPECTED_EVENT_ID"))
	credential := requireT31Env(t, "T31_GAME_SERVER_CREDENTIAL")
	otherCredential := requireT31Env(t, "T31_OTHER_GAME_SERVER_CREDENTIAL")

	caPEM, err := os.ReadFile(caPath)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM), "test gateway CA must parse")
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))
	store, err := OpenPostgresStore(ctx, pool, time.Now)
	require.NoError(t, err)
	statePath := requireT31Env(t, "T31_ACCEPTANCE_STATE_FILE")
	switch requireT31Env(t, "T31_ACCEPTANCE_PHASE") {
	case "claim-before-gis-restart":
		t31RunInitialClaimPhase(t, ctx, httpClient, baseURL, statePath, applicationID, environmentID,
			otherApplicationID, otherEnvironmentID, eventID, credential, otherCredential)
	case "reclaim-and-commit-before-ack":
		t31RunReclaimAndCommitPhase(t, ctx, httpClient, baseURL, statePath, applicationID, environmentID,
			eventID, credential, store, pool)
	case "drop-ack-response-after-receiver-restart":
		t31RunDropAckResponsePhase(t, ctx, transport, baseURL, applicationID, environmentID, eventID,
			credential, store, pool)
	case "replay-ack-after-runner-restart":
		t31RunAckReplayPhase(t, ctx, httpClient, baseURL, applicationID, environmentID, eventID,
			credential, store, pool)
	default:
		t.Fatal("T31_ACCEPTANCE_PHASE must select a documented Compose acceptance phase")
	}
}

type t31ClaimCheckpoint struct {
	EventID        string    `json:"event_id"`
	PayloadSHA256  string    `json:"payload_sha256"`
	Payload        []byte    `json:"payload_bytes"`
	LeaseID        string    `json:"lease_id"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

func t31RunInitialClaimPhase(t *testing.T, ctx context.Context, client *http.Client, baseURL, statePath string,
	applicationID, environmentID, otherApplicationID, otherEnvironmentID, eventID uuid.UUID,
	credential, otherCredential string) {
	t.Helper()
	otherClaim := t31Claim(t, ctx, client, baseURL, otherCredential)
	require.Equal(t, http.StatusNoContent, otherClaim.StatusCode)
	require.Equal(t, "1", otherClaim.Header.Get("Retry-After"))
	t.Logf("scope isolation: target_app=%s target_env=%s foreign_app=%s foreign_env=%s foreign_claim_status=%d retry_after=%s",
		applicationID, environmentID, otherApplicationID, otherEnvironmentID, otherClaim.StatusCode, otherClaim.Header.Get("Retry-After"))

	first := t31Claim(t, ctx, client, baseURL, credential)
	require.Equal(t, http.StatusOK, first.StatusCode)
	body, err := io.ReadAll(first.Body)
	require.NoError(t, err)
	require.NoError(t, first.Body.Close())
	claim, err := parseSessionEventClaim(first.Header, body, applicationID, environmentID)
	require.NoError(t, err)
	require.Equal(t, eventID, claim.EventID)
	require.Equal(t, eventID.String(), claim.Event.EventID)
	require.NotEqual(t, applicationID, otherApplicationID)
	require.NotEqual(t, environmentID, otherEnvironmentID)
	require.NoError(t, writeT31ClaimCheckpoint(statePath, claim, body))
	t.Logf("pre-restart HTTPS claim: status=%d event_id=%s payload_sha256=%x lease_id=%s lease_expires_at=%s body_bytes=%d",
		first.StatusCode, claim.EventID, claim.PayloadSHA256, claim.LeaseID,
		claim.LeaseExpiresAt.UTC().Format(time.RFC3339Nano), len(body))
}

func t31RunReclaimAndCommitPhase(t *testing.T, ctx context.Context, client *http.Client, baseURL, statePath string,
	applicationID, environmentID, eventID uuid.UUID, credential string, store *PostgresStore, pool *pgxpool.Pool) {
	t.Helper()
	checkpoint, err := readT31ClaimCheckpoint(statePath)
	require.NoError(t, err)
	require.Equal(t, eventID.String(), checkpoint.EventID)

	stillLeased := t31Claim(t, ctx, client, baseURL, credential)
	require.Equal(t, http.StatusNoContent, stillLeased.StatusCode, "GIS restart must preserve the active durable lease")
	t.Logf("GIS restart with active lease: claim_status=%d event_id=%s lease_id=%s expires_at=%s",
		stillLeased.StatusCode, checkpoint.EventID, checkpoint.LeaseID, checkpoint.LeaseExpiresAt.UTC().Format(time.RFC3339Nano))
	waitT31LeaseExpiry(t, ctx, checkpoint.LeaseExpiresAt)

	response := t31Claim(t, ctx, client, baseURL, credential)
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	claim, err := parseSessionEventClaim(response.Header, body, applicationID, environmentID)
	require.NoError(t, err)
	require.Equal(t, eventID, claim.EventID)
	require.Equal(t, checkpoint.PayloadSHA256, fmt.Sprintf("%x", claim.PayloadSHA256), "GIS restart reclaim preserves exact payload digest")
	require.True(t, bytes.Equal(checkpoint.Payload, body), "GIS restart reclaim preserves exact payload bytes")
	require.NotEqual(t, checkpoint.LeaseID, claim.LeaseID)
	t.Logf("post-restart lease reclaim: status=%d event_id=%s payload_sha256=%x old_lease_id=%s new_lease_id=%s bytes_identical_digest=true",
		response.StatusCode, claim.EventID, claim.PayloadSHA256, checkpoint.LeaseID, claim.LeaseID)
	waitT31LeaseExpiry(t, ctx, claim.LeaseExpiresAt)

	// Persist the receiver effect and its pending ACK, then exit this one-shot
	// runner. The workflow starts a new runner container before any ACK reaches GIS.
	preAckFailure := &t31FailAckBeforeGIS{next: client.Transport}
	consumer := t31SessionEventClient(baseURL, credential, applicationID, environmentID, store,
		&http.Client{Transport: preAckFailure, Timeout: 10 * time.Second})
	applied, err := consumer.PollOnce(ctx)
	require.Error(t, err, "injected transport failure occurs before the ACK reaches GIS")
	require.False(t, applied)
	require.EqualValues(t, 1, preAckFailure.ackRequests.Load())
	assertT31ReceiverRows(t, ctx, pool, applicationID, environmentID, eventID, 1, 1, false)
	t.Logf("receiver stopped before ACK: ack_requests=%d gis_ack_sent=false inbox_rows=1 effect_rows=1 acknowledged=false",
		preAckFailure.ackRequests.Load())
}

func t31RunDropAckResponsePhase(t *testing.T, ctx context.Context, transport http.RoundTripper, baseURL string,
	applicationID, environmentID, eventID uuid.UUID, credential string, store *PostgresStore, pool *pgxpool.Pool) {
	t.Helper()
	assertT31ReceiverRows(t, ctx, pool, applicationID, environmentID, eventID, 1, 1, false)
	dropper := &t31DropFirstAckResponse{next: transport}
	consumer := t31SessionEventClient(baseURL, credential, applicationID, environmentID, store,
		&http.Client{Transport: dropper, Timeout: 10 * time.Second})
	applied, err := consumer.PollOnce(ctx)
	require.Error(t, err, "first ACK response is intentionally lost after GIS commit")
	require.False(t, applied)
	require.EqualValues(t, 1, dropper.ackResponses.Load())
	assertT31ReceiverRows(t, ctx, pool, applicationID, environmentID, eventID, 1, 1, false)
	t.Logf("receiver restart then lost ACK response: upstream_ack_responses=%d inbox_rows=1 effect_rows=1 acknowledged=false",
		dropper.ackResponses.Load())
}

func t31RunAckReplayPhase(t *testing.T, ctx context.Context, client *http.Client, baseURL string,
	applicationID, environmentID, eventID uuid.UUID, credential string, store *PostgresStore, pool *pgxpool.Pool) {
	t.Helper()
	assertT31ReceiverRows(t, ctx, pool, applicationID, environmentID, eventID, 1, 1, false)
	consumer := t31SessionEventClient(baseURL, credential, applicationID, environmentID, store, client)
	applied, err := consumer.PollOnce(ctx)
	require.NoError(t, err, "new runner process must replay the exact durable ACK")
	require.False(t, applied, "ACK replay completes before the client claims another event")
	assertT31ReceiverRows(t, ctx, pool, applicationID, environmentID, eventID, 1, 1, true)
	t.Log("exact ACK replay after GIS and runner restart: inbox_rows=1 effect_rows=1 acknowledged=true")
}

func t31SessionEventClient(baseURL, credential string, applicationID, environmentID uuid.UUID,
	store *PostgresStore, client *http.Client) *SessionEventClient {
	return &SessionEventClient{BaseURL: baseURL, Credential: credential, ApplicationID: applicationID,
		EnvironmentID: environmentID, Store: store, HTTPClient: client}
}

func writeT31ClaimCheckpoint(path string, claim parsedSessionEventClaim, payload []byte) error {
	checkpoint := t31ClaimCheckpoint{EventID: claim.EventID.String(), PayloadSHA256: fmt.Sprintf("%x", claim.PayloadSHA256),
		Payload: payload, LeaseID: claim.LeaseID.String(), LeaseExpiresAt: claim.LeaseExpiresAt.UTC()}
	encoded, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o600)
}

func readT31ClaimCheckpoint(path string) (t31ClaimCheckpoint, error) {
	var checkpoint t31ClaimCheckpoint
	encoded, err := os.ReadFile(path)
	if err != nil {
		return checkpoint, err
	}
	err = json.Unmarshal(encoded, &checkpoint)
	return checkpoint, err
}

func waitT31LeaseExpiry(t *testing.T, ctx context.Context, expiresAt time.Time) {
	t.Helper()
	deadline := time.Until(expiresAt.Add(250 * time.Millisecond))
	require.Greater(t, deadline, time.Duration(0))
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-time.After(deadline):
	}
}

type t31ObservedResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

func t31Claim(t *testing.T, ctx context.Context, client *http.Client, baseURL, credential string) *t31ObservedResponse {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+sessionEventsClaimPath, http.NoBody)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+credential)
	response, err := client.Do(request)
	require.NoError(t, err)
	return &t31ObservedResponse{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: response.Body}
}

type t31DropFirstAckResponse struct {
	next         http.RoundTripper
	dropped      atomic.Bool
	ackResponses atomic.Int32
}

type t31FailAckBeforeGIS struct {
	next        http.RoundTripper
	ackRequests atomic.Int32
}

func (transport *t31FailAckBeforeGIS) RoundTrip(request *http.Request) (*http.Response, error) {
	if strings.HasSuffix(request.URL.Path, "/ack") {
		transport.ackRequests.Add(1)
		return nil, errors.New("injected receiver stop before GIS ACK request")
	}
	return transport.next.RoundTrip(request)
}

func (transport *t31DropFirstAckResponse) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.next.RoundTrip(request)
	if err != nil || !strings.HasSuffix(request.URL.Path, "/ack") || response.StatusCode != http.StatusOK {
		return response, err
	}
	transport.ackResponses.Add(1)
	if transport.dropped.CompareAndSwap(false, true) {
		_ = response.Body.Close()
		return nil, errors.New("injected lost ACK response after upstream commit")
	}
	return response, nil
}

func assertT31ReceiverRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, appID, envID, eventID uuid.UUID, inboxWant, effectWant int, acked bool) {
	t.Helper()
	var inboxRows, effectRows int
	var hasAckLease, hasAcknowledgedAt bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT ack_lease_id IS NOT NULL,acknowledged_at IS NOT NULL
		FROM session_event_inbox WHERE application_id=$1 AND environment_id=$2 AND event_id=$3`, appID, envID, eventID).Scan(&hasAckLease, &hasAcknowledgedAt))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_event_inbox WHERE application_id=$1 AND environment_id=$2 AND event_id=$3`,
		appID, envID, eventID).Scan(&inboxRows))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_activation_effects WHERE application_id=$1 AND environment_id=$2 AND event_id=$3`,
		appID, envID, eventID).Scan(&effectRows))
	require.Equal(t, inboxWant, inboxRows)
	require.Equal(t, effectWant, effectRows)
	if acked {
		require.True(t, hasAckLease)
		require.True(t, hasAcknowledgedAt)
	} else {
		require.True(t, hasAckLease, "the receiver has durably saved the ACK lease before its response is lost")
		require.False(t, hasAcknowledgedAt, "receiver ACK state advances only after GIS confirms delivery")
	}
}

func requireT31Env(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	require.NotEmpty(t, value, "%s is required", name)
	return value
}
