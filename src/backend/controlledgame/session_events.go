package controlledgame

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	sessionEventsClaimPath = "/api/v1/session-events/claim"
	maxSessionEventBytes   = 64 << 10
)

var errSessionEventAckConflict = errors.New("GIS rejected a stale or conflicting session-event ACK")

// SessionEventClient polls GIS using one app/environment-scoped game-server
// credential. Its HTTP client uses standard certificate and hostname checks.
type SessionEventClient struct {
	BaseURL       string
	Credential    string
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	Store         *PostgresStore
	HTTPClient    *http.Client
	Now           func() time.Time
}

// RunSessionEventPoller repeatedly invokes the HTTPS pull client. Empty polls
// use the contract's one-second Retry-After; transient transport or receiver
// failures are also retried without acknowledging uncommitted work.
func RunSessionEventPoller(ctx context.Context, client *SessionEventClient, onError func(error)) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if _, err := client.PollOnce(ctx); err != nil && onError != nil {
			onError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// PollOnce claims at most one event, commits its receiver inbox and visible
// activation effect in the controlled game DB, then ACKs that committed digest.
// Any error before the ACK response is safe to retry; expired leases are
// reclaimed by the next claim and the inbox makes duplicate effects inert.
func (client *SessionEventClient) PollOnce(ctx context.Context) (bool, error) {
	if client == nil || client.Store == nil || client.ApplicationID == uuid.Nil || client.EnvironmentID == uuid.Nil {
		return false, errors.New("controlled game session-event client is not configured")
	}
	base, err := url.Parse(strings.TrimSpace(client.BaseURL))
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil ||
		base.Path != "" && base.Path != "/" || base.RawQuery != "" || base.Fragment != "" {
		return false, errors.New("GIS session-event base URL must be an HTTPS origin")
	}
	if client.Credential == "" || strings.TrimSpace(client.Credential) != client.Credential || strings.ContainsAny(client.Credential, "\r\n") {
		return false, errors.New("GIS game-server credential is required")
	}
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	requestClient := *httpClient
	requestClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	replayed, err := client.retryPendingSessionEventAcks(ctx, *base, &requestClient)
	if err != nil {
		return false, err
	}
	if replayed {
		// A prior receiver commit is now confirmed at GIS. Finish this poll here;
		// claiming new work belongs to the next scheduled poll.
		return false, nil
	}
	claimURL := *base
	claimURL.Path = strings.TrimRight(base.Path, "/") + sessionEventsClaimPath
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, claimURL.String(), http.NoBody)
	if err != nil {
		return false, err
	}
	request.Header.Set("Authorization", "Bearer "+client.Credential)
	response, err := requestClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("claim GIS session event: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		if _, err = io.Copy(io.Discard, io.LimitReader(response.Body, 1)); err != nil {
			return false, err
		}
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GIS claim returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSessionEventBytes+1))
	if err != nil {
		return false, err
	}
	if len(body) == 0 || len(body) > maxSessionEventBytes {
		return false, errors.New("GIS session-event body is empty or too large")
	}
	claim, err := parseSessionEventClaim(response.Header, body, client.ApplicationID, client.EnvironmentID)
	if err != nil {
		return false, err
	}
	now := time.Now
	if client.Now != nil {
		now = client.Now
	}
	if !claim.LeaseExpiresAt.After(now().UTC()) {
		return false, errors.New("GIS session-event claim lease is already expired")
	}
	committed, err := client.Store.ConsumeSessionActiveEventWithLease(ctx, client.ApplicationID, client.EnvironmentID,
		claim.EventID, claim.LeaseID, body, claim.PayloadSHA256, func(tx pgx.Tx) error {
			_, effectErr := tx.Exec(ctx, `INSERT INTO session_activation_effects(application_id,environment_id,event_id,session_id,operation_id,kind,active_at,committed_at)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, client.ApplicationID, client.EnvironmentID, claim.EventID,
				claim.Event.SessionID, claim.Event.OperationID, claim.Event.Kind, claim.Event.ActiveAt.UTC(), now().UTC())
			return effectErr
		})
	if err != nil {
		return false, fmt.Errorf("commit controlled game session-event effect: %w", err)
	}
	pending := pendingSessionEventAck{
		applicationID: client.ApplicationID, environmentID: client.EnvironmentID,
		eventID: claim.EventID, leaseID: claim.LeaseID, digest: claim.PayloadSHA256,
	}
	if err := client.sendSessionEventAck(ctx, *base, &requestClient, pending); err != nil {
		return false, fmt.Errorf("ACK GIS session event after receiver commit: %w", err)
	}
	if err := client.Store.markSessionEventAcked(ctx, pending); err != nil {
		return false, fmt.Errorf("record GIS session-event ACK locally: %w", err)
	}
	return committed, nil
}

func (client *SessionEventClient) retryPendingSessionEventAcks(ctx context.Context, base url.URL, httpClient *http.Client) (bool, error) {
	pending, err := client.Store.pendingSessionEventAcks(ctx, client.ApplicationID, client.EnvironmentID)
	if err != nil {
		return false, fmt.Errorf("load pending GIS session-event ACKs: %w", err)
	}
	replayed := false
	for _, item := range pending {
		if err = client.sendSessionEventAck(ctx, base, httpClient, item); errors.Is(err, errSessionEventAckConflict) {
			// A lost lease is refreshed by the next claim. The durable receiver row
			// keeps the effect inert and will save the replacement lease on replay.
			continue
		} else if err != nil {
			return replayed, err
		}
		if err = client.Store.markSessionEventAcked(ctx, item); err != nil {
			return replayed, fmt.Errorf("record replayed GIS session-event ACK locally: %w", err)
		}
		replayed = true
	}
	return replayed, nil
}

func (client *SessionEventClient) sendSessionEventAck(ctx context.Context, base url.URL, httpClient *http.Client, item pendingSessionEventAck) error {
	ackBody, err := json.Marshal(struct {
		LeaseID     string `json:"lease_id"`
		PayloadHash string `json:"payload_sha256"`
	}{LeaseID: item.leaseID.String(), PayloadHash: hex.EncodeToString(item.digest)})
	if err != nil {
		return err
	}
	ackURL := base
	ackURL.Path = strings.TrimRight(base.Path, "/") + "/api/v1/session-events/" + item.eventID.String() + "/ack"
	ackRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, ackURL.String(), bytes.NewReader(ackBody))
	if err != nil {
		return err
	}
	ackRequest.Header.Set("Authorization", "Bearer "+client.Credential)
	ackRequest.Header.Set("Content-Type", "application/json")
	ackResponse, err := httpClient.Do(ackRequest)
	if err != nil {
		return fmt.Errorf("ACK GIS session event: %w", err)
	}
	defer ackResponse.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(ackResponse.Body, 4096))
	switch ackResponse.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusConflict:
		return errSessionEventAckConflict
	default:
		return fmt.Errorf("GIS session-event ACK returned HTTP %d", ackResponse.StatusCode)
	}
}

type parsedSessionEventClaim struct {
	Event          SessionActiveEvent
	EventID        uuid.UUID
	PayloadSHA256  []byte
	LeaseID        uuid.UUID
	LeaseExpiresAt time.Time
}

func parseSessionEventClaim(headers http.Header, body []byte, applicationID, environmentID uuid.UUID) (parsedSessionEventClaim, error) {
	var claim parsedSessionEventClaim
	if headers.Get("Content-Encoding") != "identity" {
		return claim, errors.New("GIS session-event response must preserve identity encoding")
	}
	contentTypes := headers.Values("Content-Type")
	if len(contentTypes) != 1 {
		return claim, errors.New("GIS session-event response content type is missing or repeated")
	}
	mediaType, _, contentTypeErr := mime.ParseMediaType(contentTypes[0])
	if contentTypeErr != nil || mediaType != "application/json" {
		return claim, errors.New("GIS session-event response must be application/json")
	}
	if err := validateSessionEventJSON(body); err != nil {
		return claim, err
	}
	if err := json.Unmarshal(body, &claim.Event); err != nil {
		return claim, fmt.Errorf("decode GIS session event: %w", err)
	}
	if !validSessionEventUUID(claim.Event.EventID) || !validSessionEventUUID(claim.Event.ApplicationID) ||
		!validSessionEventUUID(claim.Event.EnvironmentID) || !validSessionEventUUID(claim.Event.SessionID) ||
		!validSessionEventUUID(claim.Event.OperationID) || claim.Event.Kind != "party" && claim.Event.Kind != "match" && claim.Event.Kind != "fleet" {
		return claim, errors.New("GIS session event has invalid identity or kind")
	}
	if claim.Event.ApplicationID != applicationID.String() || claim.Event.EnvironmentID != environmentID.String() {
		return claim, errors.New("GIS session event is outside the configured app/environment")
	}
	_, offset := claim.Event.ActiveAt.Zone()
	if offset != 0 || claim.Event.ActiveAt.IsZero() {
		return claim, errors.New("GIS session event active_at must be UTC")
	}
	var ok bool
	var raw string
	if raw, ok = singleDeliveryHeader(headers, "X-Voice-Event-Id"); !ok || !validSessionEventUUID(raw) || raw != claim.Event.EventID {
		return claim, errors.New("GIS session-event ID header does not match body")
	}
	claim.EventID, _ = uuid.Parse(raw)
	if raw, ok = singleDeliveryHeader(headers, "X-Voice-Payload-SHA256"); !ok || len(raw) != 64 || raw != strings.ToLower(raw) {
		return claim, errors.New("GIS session-event digest header is invalid")
	}
	claim.PayloadSHA256, _ = hex.DecodeString(raw)
	digest := sha256.Sum256(body)
	if !bytes.Equal(claim.PayloadSHA256, digest[:]) {
		return claim, errors.New("GIS session-event body digest does not match header")
	}
	if raw, ok = singleDeliveryHeader(headers, "X-Voice-Claim-Lease-Id"); !ok || !validSessionEventUUID(raw) {
		return claim, errors.New("GIS session-event lease ID header is invalid")
	}
	claim.LeaseID, _ = uuid.Parse(raw)
	if raw, ok = singleDeliveryHeader(headers, "X-Voice-Claim-Lease-Expires-At"); !ok {
		return claim, errors.New("GIS session-event lease expiry header is missing")
	}
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return claim, errors.New("GIS session-event lease expiry header is invalid")
	}
	_, expiryOffset := expiresAt.Zone()
	if expiryOffset != 0 {
		return claim, errors.New("GIS session-event lease expiry must be UTC")
	}
	claim.LeaseExpiresAt = expiresAt.UTC()
	return claim, nil
}

func singleDeliveryHeader(headers http.Header, name string) (string, bool) {
	values := headers.Values(name)
	if len(values) != 1 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

func validSessionEventUUID(value string) bool {
	if !canonicalUUID(value) {
		return false
	}
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil
}

func validateSessionEventJSON(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("GIS session event must be a JSON object")
	}
	seen := make(map[string]struct{}, 7)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("GIS session event has an invalid key")
		}
		if _, exists := seen[key]; exists {
			return errors.New("GIS session event contains a duplicate key")
		}
		seen[key] = struct{}{}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return err
		}
	}
	if len(seen) != 7 {
		return errors.New("GIS session event must contain exactly seven fields")
	}
	for _, key := range []string{"event_id", "application_id", "environment_id", "session_id", "operation_id", "kind", "active_at"} {
		if _, ok := seen[key]; !ok {
			return errors.New("GIS session event is missing a required field")
		}
	}
	if _, err = decoder.Token(); err != nil {
		return err
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("GIS session event has trailing JSON")
	}
	return nil
}
