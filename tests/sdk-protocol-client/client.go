package sdkprotocolclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

// Client is a small test-only HTTP client for the Auth SDK device-key API.
// It never stores a provider bearer or device key beyond the caller's scope.
type Client struct {
	base *url.URL
	http *http.Client
}

type DeviceKeyChallengeRequest struct {
	Purpose          string  `json:"purpose"`
	ApplicationID    string  `json:"applicationId"`
	EnvironmentID    string  `json:"environmentId"`
	PublicJWK        string  `json:"publicJwk"`
	ReplacesDeviceID *string `json:"replacesDeviceId,omitempty"`
}

type DeviceKeyChallenge struct {
	ChallengeID      string `json:"challengeId"`
	Nonce            string `json:"nonce"`
	ClientID         string `json:"clientId"`
	ExpiresAt        string `json:"expiresAt"`
	Purpose          string `json:"purpose"`
	DeviceID         string `json:"deviceId"`
	ReplacesDeviceID string `json:"replacesDeviceId"`
}

type RotateDeviceKeyRequest struct {
	ChallengeID     string `json:"challengeId"`
	ProviderToken   string `json:"providerToken"`
	CurrentKeyProof string `json:"currentKeyProof"`
	NewKeyProof     string `json:"newKeyProof"`
	RequestID       string `json:"requestId"`
}

type RecoverDeviceKeyRequest struct {
	ChallengeID   string `json:"challengeId"`
	ProviderToken string `json:"providerToken"`
	NewKeyProof   string `json:"newKeyProof"`
	RequestID     string `json:"requestId"`
}

type RevokeDeviceRequest struct {
	ProviderToken   string `json:"providerToken"`
	CurrentKeyProof string `json:"currentKeyProof"`
	RequestID       string `json:"requestId"`
}

type DeviceKeyResult struct {
	DeviceID          string `json:"deviceId"`
	KeyID             string `json:"keyId"`
	Generation        int64  `json:"generation"`
	AuthorityRevision int64  `json:"authorityRevision"`
}

func NewClient(baseURL string, transport *http.Client) (*Client, error) {
	base, err := url.Parse(baseURL)
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || base.Path != "" {
		return nil, errors.New("Auth base URL must be an origin")
	}
	if base.Scheme != "https" && !(base.Scheme == "http" && isLoopbackHost(base.Hostname())) {
		return nil, errors.New("Auth base URL must use HTTPS (HTTP is allowed only on loopback)")
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	if transport != nil {
		*httpClient = *transport
	}
	// Device proofs and provider credentials must never follow an HTTP redirect.
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{base: base, http: httpClient}, nil
}

func (c *Client) PostJSON(ctx context.Context, path, bearer string, request any) ([]byte, error) {
	return c.doJSON(ctx, http.MethodPost, path, bearer, request)
}

func (c *Client) CreateDeviceKeyChallenge(ctx context.Context, bearer string, request DeviceKeyChallengeRequest) (DeviceKeyChallenge, error) {
	var response DeviceKeyChallenge
	err := c.doJSONInto(ctx, http.MethodPost, "/api/v1/auth/sdk/device-keys/challenges", bearer, request, &response)
	return response, err
}

func (c *Client) RotateDeviceKey(ctx context.Context, request RotateDeviceKeyRequest) (DeviceKeyResult, error) {
	var response DeviceKeyResult
	err := c.doJSONInto(ctx, http.MethodPost, "/api/v1/auth/sdk/device-keys/rotate", "", request, &response)
	return response, err
}

func (c *Client) RecoverDeviceKey(ctx context.Context, request RecoverDeviceKeyRequest) (DeviceKeyResult, error) {
	var response DeviceKeyResult
	err := c.doJSONInto(ctx, http.MethodPost, "/api/v1/auth/sdk/device-keys/recover", "", request, &response)
	return response, err
}

func (c *Client) RevokeDevice(ctx context.Context, deviceID string, request RevokeDeviceRequest) (DeviceKeyResult, error) {
	var response DeviceKeyResult
	path := "/api/v1/auth/sdk/devices/" + deviceID
	err := c.doJSONInto(ctx, http.MethodDelete, path, "", request, &response)
	return response, err
}

func (c *Client) doJSONInto(ctx context.Context, method, path, bearer string, request, response any) error {
	body, err := c.doJSON(ctx, method, path, bearer, request)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(response); err != nil {
		return fmt.Errorf("decode Auth response: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("Auth response must contain one JSON value")
	}
	return nil
}

func (c *Client) doJSON(ctx context.Context, method, path, bearer string, request any) ([]byte, error) {
	if c == nil || c.base == nil || c.http == nil {
		return nil, errors.New("Auth client is not configured")
	}
	parsed, err := url.ParseRequestURI(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(parsed.Path, "..") || !allowedRoute(method, parsed.Path) {
		return nil, errors.New("request path must be a relative Auth SDK API path")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode Auth request: %w", err)
	}
	target := *c.base
	target.Path = parsed.Path
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Auth API request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Auth response: %w", err)
	}
	if len(responseBody) > maxResponseBytes {
		return nil, errors.New("Auth response exceeds 1 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Auth API returned HTTP %d", response.StatusCode)
	}
	return responseBody, nil
}

func allowedRoute(method, path string) bool {
	if method == http.MethodPost {
		return path == "/api/v1/auth/sdk/device-keys/challenges" ||
			path == "/api/v1/auth/sdk/device-keys/rotate" ||
			path == "/api/v1/auth/sdk/device-keys/recover"
	}
	if method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/auth/sdk/devices/") {
		id := strings.TrimPrefix(path, "/api/v1/auth/sdk/devices/")
		return id != "" && !strings.Contains(id, "/") && id != "." && id != ".."
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
