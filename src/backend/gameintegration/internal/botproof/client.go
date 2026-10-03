package botproof

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"voice/backend/pkg/workloadproof"
)

var (
	ErrDenied       = errors.New("bot owner authority denied")
	ErrInvalidProof = errors.New("invalid Bot authority proof")
	ErrUnavailable  = errors.New("bot authority proof unavailable")
)

const maxResponseBytes = 1024

type ownerProofRequest struct {
	ApplicationOwnerAccountID string `json:"application_owner_account_id"`
}

type ownerProofResponse struct {
	BotID          string `json:"bot_id"`
	OwnerAccountID string `json:"owner_account_id"`
	Status         string `json:"status"`
}

// Client calls Bot's narrow T11 authority proof endpoint. It carries no
// forwarded client identity and accepts only a response authenticated under
// the dedicated GIS↔Bot workload key.
type Client struct {
	BaseURL  string
	Key      []byte
	HTTP     *http.Client
	Now      func() time.Time
	NewNonce func() string
}

func DecodeWorkloadKey(encoded string) ([]byte, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64 must be a base64-encoded 32-byte key")
	}
	return key, nil
}

func (c *Client) VerifyGameIntegrationBot(ctx context.Context, botID, ownerID uuid.UUID) error {
	if c == nil || len(c.Key) != 32 || c.Now == nil || c.NewNonce == nil || botID == uuid.Nil || ownerID == uuid.Nil {
		return ErrUnavailable
	}
	base, err := validateBaseURL(c.BaseURL)
	if err != nil {
		return ErrUnavailable
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("bot authority redirects forbidden")
		}}
	} else {
		copy := *client
		copy.CheckRedirect = func(*http.Request, []*http.Request) error {
			return errors.New("bot authority redirects forbidden")
		}
		client = &copy
	}
	path := "/internal/v1/game-integrations/bots/" + botID.String() + "/authority"
	endpoint := base + path
	body, err := json.Marshal(ownerProofRequest{ApplicationOwnerAccountID: ownerID.String()})
	if err != nil {
		return ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	nonce := c.NewNonce()
	if parsed, parseErr := uuid.Parse(nonce); parseErr != nil || parsed == uuid.Nil || parsed.String() != nonce {
		return ErrUnavailable
	}
	workloadproof.SignRequest(request, c.Key, "gameintegration", "bot", c.Now().UTC(), nonce, body)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusForbidden {
			return ErrDenied
		}
		return fmt.Errorf("%w: Bot returned status %d", ErrUnavailable, response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/json" || len(response.Header.Values("Content-Type")) != 1 {
		return ErrInvalidProof
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(responseBody) == 0 || len(responseBody) > maxResponseBytes {
		return ErrInvalidProof
	}
	if _, err := workloadproof.VerifyResponse(c.Key, response.StatusCode, path,
		request.Header.Get("X-Voice-Timestamp"), nonce, responseBody, response.Header); err != nil {
		return ErrInvalidProof
	}
	var result ownerProofResponse
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil || decoder.Decode(new(any)) != io.EOF {
		return ErrInvalidProof
	}
	canonicalBody, err := json.Marshal(result)
	if err != nil || !bytes.Equal(canonicalBody, responseBody) {
		return ErrInvalidProof
	}
	resultBotID, botErr := uuid.Parse(result.BotID)
	resultOwnerID, ownerErr := uuid.Parse(result.OwnerAccountID)
	if botErr != nil || ownerErr != nil || resultBotID == uuid.Nil || resultOwnerID == uuid.Nil ||
		resultBotID.String() != result.BotID || resultOwnerID.String() != result.OwnerAccountID {
		return ErrInvalidProof
	}
	if resultBotID != botID || resultOwnerID != ownerID || result.Status != "live" {
		return ErrDenied
	}
	return nil
}

func validateBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", ErrUnavailable
	}
	return strings.TrimSuffix(parsed.String(), "/"), nil
}
