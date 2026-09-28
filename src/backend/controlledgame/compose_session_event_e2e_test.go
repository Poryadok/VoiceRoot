package controlledgame

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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

	// A credential for another application/environment cannot claim this event.
	otherClaim := t31Claim(t, ctx, httpClient, baseURL, otherCredential)
	require.Equal(t, http.StatusNoContent, otherClaim.StatusCode)
	require.Equal(t, "1", otherClaim.Header.Get("Retry-After"))
	t.Logf("scope isolation: target_app=%s target_env=%s foreign_app=%s foreign_env=%s foreign_claim_status=%d retry_after=%s",
		applicationID, environmentID, otherApplicationID, otherEnvironmentID, otherClaim.StatusCode, otherClaim.Header.Get("Retry-After"))

	first := t31Claim(t, ctx, httpClient, baseURL, credential)
	require.Equal(t, http.StatusOK, first.StatusCode)
	firstBody, err := io.ReadAll(first.Body)
	require.NoError(t, err)
	require.NoError(t, first.Body.Close())
	firstClaim, err := parseSessionEventClaim(first.Header, firstBody, applicationID, environmentID)
	require.NoError(t, err)
	require.Equal(t, eventID, firstClaim.EventID)
	require.Equal(t, eventID.String(), firstClaim.Event.EventID)
	require.NotEqual(t, applicationID, otherApplicationID)
	require.NotEqual(t, environmentID, otherEnvironmentID)
	t.Logf("first HTTPS claim: status=%d event_id=%s payload_sha256=%x lease_id=%s lease_expires_at=%s body_bytes=%d",
		first.StatusCode, firstClaim.EventID, firstClaim.PayloadSHA256, firstClaim.LeaseID,
		firstClaim.LeaseExpiresAt.UTC().Format(time.RFC3339Nano), len(firstBody))

	// Let GIS expire the first real lease. Re-claim must return the exact stored
	// body/digest with a new lease, proving reclaim over Gateway rather than a DB
	// update or an in-memory delivery stub.
	waitT31LeaseExpiry(t, ctx, firstClaim.LeaseExpiresAt)
	second := t31Claim(t, ctx, httpClient, baseURL, credential)
	require.Equal(t, http.StatusOK, second.StatusCode)
	secondBody, err := io.ReadAll(second.Body)
	require.NoError(t, err)
	require.NoError(t, second.Body.Close())
	secondClaim, err := parseSessionEventClaim(second.Header, secondBody, applicationID, environmentID)
	require.NoError(t, err)
	require.Equal(t, firstBody, secondBody)
	require.Equal(t, firstClaim.PayloadSHA256, secondClaim.PayloadSHA256)
	require.NotEqual(t, firstClaim.LeaseID, secondClaim.LeaseID)
	t.Logf("lease reclaim: status=%d event_id=%s payload_sha256=%x new_lease_id=%s body_bytes=%d bytes_identical=true",
		second.StatusCode, secondClaim.EventID, secondClaim.PayloadSHA256, secondClaim.LeaseID, len(secondBody))
	// PollOnce issues its own claim request, so release the inspected lease before
	// the receiver starts. The client must receive a current lease to exercise ACK.
	waitT31LeaseExpiry(t, ctx, secondClaim.LeaseExpiresAt)

	// The consumer commits its own inbox/effect and sends ACK through the real
	// HTTPS path. Drop only the first successful ACK response to simulate a lost
	// reply after GIS commit; PollOnce then replays the exact ACK idempotently.
	dropper := &t31DropFirstAckResponse{next: transport}
	consumer := &SessionEventClient{
		BaseURL: baseURL, Credential: credential, ApplicationID: applicationID,
		EnvironmentID: environmentID, Store: store,
		HTTPClient: &http.Client{Transport: dropper, Timeout: 10 * time.Second},
	}
	applied, firstErr := consumer.PollOnce(ctx)
	require.Error(t, firstErr, "first ACK response is intentionally lost after its upstream commit")
	require.False(t, applied)
	assertT31ReceiverRows(t, ctx, pool, applicationID, environmentID, eventID, 1, 1, false)
	t.Logf("injected lost ACK response: receiver inbox_rows=1 effect_rows=1 acknowledged=false upstream_ack_responses=%d", dropper.ackResponses.Load())

	applied, err = consumer.PollOnce(ctx)
	require.NoError(t, err, "pending exact ACK replay must complete")
	require.False(t, applied, "ACK replay completes before the client claims another event")
	require.EqualValues(t, 2, dropper.ackResponses.Load())
	assertT31ReceiverRows(t, ctx, pool, applicationID, environmentID, eventID, 1, 1, true)
	t.Logf("exact ACK replay: upstream_ack_responses=%d receiver inbox_rows=1 effect_rows=1 acknowledged=true", dropper.ackResponses.Load())
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
