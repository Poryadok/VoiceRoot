package gameconsent

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrUnavailable = errors.New("game notification consent authority unavailable")

func ConfigFromEnv(getenv func(string) string) (*Client, bool, error) {
	if getenv == nil {
		return nil, false, fmt.Errorf("game notification consent environment is unavailable")
	}
	baseURL := strings.TrimSpace(getenv("GAME_INTEGRATION_CONSENT_URL"))
	rawKey := strings.TrimSpace(getenv("GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64"))
	if baseURL == "" && rawKey == "" {
		return nil, false, nil
	}
	if baseURL == "" || rawKey == "" {
		return nil, false, fmt.Errorf("GIS consent URL and workload key must be configured together")
	}
	key, err := base64.StdEncoding.Strict().DecodeString(rawKey)
	if err != nil || len(key) != 32 {
		return nil, false, fmt.Errorf("invalid GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64")
	}
	client, err := New(baseURL, key)
	if err != nil {
		return nil, false, err
	}
	return client, true, nil
}

type Client struct {
	BaseURL  string
	Key      []byte
	HTTP     *http.Client
	Now      func() time.Time
	NewNonce func() string
}

func New(baseURL string, key []byte) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil || len(key) != 32 {
		return nil, fmt.Errorf("game notification consent configuration is invalid")
	}
	return &Client{BaseURL: baseURL, Key: append([]byte(nil), key...), HTTP: &http.Client{Timeout: 2 * time.Second}, Now: time.Now, NewNonce: func() string { return uuid.NewString() }}, nil
}

func (c *Client) AllowsGamePush(ctx context.Context, profileID, appID, envID uuid.UUID, category string) (bool, error) {
	if c == nil || len(c.Key) != 32 || c.HTTP == nil || c.Now == nil || c.NewNonce == nil ||
		profileID == uuid.Nil || appID == uuid.Nil || envID == uuid.Nil || !validCategory(category) {
		return false, ErrUnavailable
	}
	path := "/internal/v1/notification-consents/" + appID.String() + "/" + envID.String() + "/" + profileID.String() + "/" + url.PathEscape(category)
	body, err := c.getSigned(ctx, path)
	if err != nil {
		return false, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var result struct {
		Allowed *bool `json:"allowed"`
	}
	if decoder.Decode(&result) != nil || result.Allowed == nil || decoder.Decode(new(any)) != io.EOF {
		return false, ErrUnavailable
	}
	return *result.Allowed, nil
}

func (c *Client) getSigned(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	timestamp := strconv.FormatInt(c.Now().Unix(), 10)
	nonce := c.NewNonce()
	req.Header.Set("X-Voice-Workload", "notification")
	req.Header.Set("X-Voice-Timestamp", timestamp)
	req.Header.Set("X-Voice-Nonce", nonce)
	req.Header.Set("X-Voice-Signature", requestSignature(c.Key, req.Method, req.URL.EscapedPath(), timestamp, nonce))
	response, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 || response.StatusCode != http.StatusOK ||
		response.Header.Get("X-Voice-Response-Timestamp") != timestamp || response.Header.Get("X-Voice-Response-Nonce") != nonce ||
		!hmac.Equal([]byte(response.Header.Get("X-Voice-Response-Signature")), []byte(responseSignature(c.Key, response.StatusCode, req.URL.EscapedPath(), timestamp, nonce, body))) {
		return nil, ErrUnavailable
	}
	return body, nil
}

// ResolveGamePushChat resolves whether a chat notification is game-managed and
// whether the recipient currently allows its category on push.
func (c *Client) ResolveGamePushChat(ctx context.Context, profileID, chatID uuid.UUID, category string) (bool, bool, error) {
	if c == nil || len(c.Key) != 32 || c.HTTP == nil || c.Now == nil || c.NewNonce == nil ||
		profileID == uuid.Nil || chatID == uuid.Nil || !validCategory(category) {
		return false, false, ErrUnavailable
	}
	path := "/internal/v1/notification-consents/chat/" + profileID.String() + "/" + chatID.String() + "/" + url.PathEscape(category) + "/push"
	body, err := c.getSigned(ctx, path)
	if err != nil {
		return false, false, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var result struct {
		GameScoped *bool `json:"game_scoped"`
		Allowed    *bool `json:"allowed"`
	}
	if decoder.Decode(&result) != nil || result.GameScoped == nil || result.Allowed == nil || decoder.Decode(new(any)) != io.EOF {
		return false, false, ErrUnavailable
	}
	return *result.GameScoped, *result.Allowed, nil
}

func validCategory(category string) bool {
	switch category {
	case "game_activity", "game_social", "game_actions":
		return true
	default:
		return false
	}
}

func requestSignature(key []byte, method, path, timestamp, nonce string) string {
	empty := sha256.Sum256(nil)
	message := "v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(empty[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func responseSignature(key []byte, status int, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := "v1\n" + strconv.Itoa(status) + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
