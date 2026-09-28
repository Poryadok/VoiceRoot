package authbinding

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
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	exchangePath   = "/internal/v1/auth/game-bindings/handoffs/exchange"
	claimPath      = "/internal/v1/auth/game-bindings/handoffs/claim"
	completionPath = "/internal/v1/auth/game-bindings/handoffs/completion"
	revokePath     = "/internal/v1/auth/game-bindings/handoffs/revoke"
	maxResponse    = 16 << 10
)

var ErrUnavailable = errors.New("Auth game-binding authority unavailable")

var requestHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var codePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var verifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
var compactJWS = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

type ExchangeRequest struct {
	ChallengeID  uuid.UUID `json:"challenge_id"`
	OperationID  uuid.UUID `json:"operation_id"`
	Code         string    `json:"code"`
	CodeVerifier string    `json:"code_verifier"`
	DeviceProof  string    `json:"device_proof"`
}

type ExchangeResponse struct {
	HandoffJWS string `json:"handoff_jws"`
}

type ClaimRequest struct {
	OperationID   uuid.UUID `json:"operation_id"`
	RequestSHA256 string    `json:"request_sha256"`
	HandoffJWS    string    `json:"handoff_jws"`
	DeviceProof   string    `json:"device_proof"`
}

type ClaimReceipt struct {
	ClaimID      uuid.UUID `json:"claim_id"`
	OperationID  uuid.UUID `json:"operation_id"`
	AssertionJTI uuid.UUID `json:"assertion_jti"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type CompletionRequest struct {
	ClaimID     uuid.UUID `json:"claim_id"`
	OperationID uuid.UUID `json:"operation_id"`
	Outcome     string    `json:"outcome"`
	BindingID   uuid.UUID `json:"binding_id,omitempty"`
}

func (r CompletionRequest) MarshalJSON() ([]byte, error) {
	var bindingID *uuid.UUID
	if r.BindingID != uuid.Nil {
		bindingID = &r.BindingID
	}
	return json.Marshal(struct {
		ClaimID     uuid.UUID  `json:"claim_id"`
		OperationID uuid.UUID  `json:"operation_id"`
		Outcome     string     `json:"outcome"`
		BindingID   *uuid.UUID `json:"binding_id,omitempty"`
	}{r.ClaimID, r.OperationID, r.Outcome, bindingID})
}

type CompletionReceipt struct {
	ClaimID     uuid.UUID `json:"claim_id"`
	OperationID uuid.UUID `json:"operation_id"`
	Outcome     string    `json:"outcome"`
	BindingID   uuid.UUID `json:"binding_id,omitempty"`
	Status      string    `json:"status"`
}

type RevokeRequest struct {
	OperationID uuid.UUID `json:"operation_id"`
}

type RevokeReceipt struct {
	OperationID       uuid.UUID `json:"operation_id"`
	Status            string    `json:"status"`
	AuthorityRevision int64     `json:"authority_revision"`
}

type Client struct {
	baseURL *url.URL
	http    *http.Client
}

func NewClient(baseURL, caFile, certFile, keyFile string) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	caFile, certFile, keyFile = strings.TrimSpace(caFile), strings.TrimSpace(certFile), strings.TrimSpace(keyFile)
	if baseURL == "" || caFile == "" || certFile == "" || keyFile == "" {
		return nil, ErrUnavailable
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" ||
		base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return nil, ErrUnavailable
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read Auth CA: %w", ErrUnavailable)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, ErrUnavailable
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load Auth client certificate: %w", ErrUnavailable)
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12,
		RootCAs: roots, Certificates: []tls.Certificate{certificate}}, TLSHandshakeTimeout: 2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second, DisableKeepAlives: false}
	return &Client{baseURL: base, http: &http.Client{Transport: transport, Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) Claim(ctx context.Context, request ClaimRequest) (ClaimReceipt, error) {
	if request.OperationID == uuid.Nil || !requestHashPattern.MatchString(request.RequestSHA256) || request.HandoffJWS == "" ||
		request.DeviceProof == "" || len(request.HandoffJWS) > 16<<10 || len(request.DeviceProof) > 8<<10 {
		return ClaimReceipt{}, ErrUnavailable
	}
	var response ClaimReceipt
	if err := c.post(ctx, claimPath, request, &response); err != nil {
		return ClaimReceipt{}, err
	}
	if response.ClaimID == uuid.Nil || response.AssertionJTI == uuid.Nil || response.OperationID != request.OperationID ||
		response.ExpiresAt.IsZero() {
		return ClaimReceipt{}, ErrUnavailable
	}
	return response, nil
}

// Exchange consumes a T14 approval code on Auth's private GIS-authenticated
// route and receives the Auth-only delivery assertion over mTLS.
func (c *Client) Exchange(ctx context.Context, request ExchangeRequest) (ExchangeResponse, error) {
	if request.ChallengeID == uuid.Nil || request.OperationID == uuid.Nil || !codePattern.MatchString(request.Code) ||
		!verifierPattern.MatchString(request.CodeVerifier) || request.DeviceProof == "" || len(request.DeviceProof) > 8<<10 {
		return ExchangeResponse{}, ErrUnavailable
	}
	var response ExchangeResponse
	if err := c.post(ctx, exchangePath, request, &response); err != nil {
		return ExchangeResponse{}, err
	}
	if !compactJWS.MatchString(response.HandoffJWS) || len(response.HandoffJWS) > 8<<10 {
		return ExchangeResponse{}, ErrUnavailable
	}
	return response, nil
}

func (c *Client) Complete(ctx context.Context, request CompletionRequest) (CompletionReceipt, error) {
	if request.ClaimID == uuid.Nil || request.OperationID == uuid.Nil ||
		(request.Outcome != "succeeded" && request.Outcome != "failed") ||
		(request.Outcome == "succeeded" && request.BindingID == uuid.Nil) ||
		(request.Outcome == "failed" && request.BindingID != uuid.Nil) {
		return CompletionReceipt{}, ErrUnavailable
	}
	var response CompletionReceipt
	if err := c.post(ctx, completionPath, request, &response); err != nil {
		return CompletionReceipt{}, err
	}
	if response.ClaimID != request.ClaimID || response.OperationID != request.OperationID ||
		response.Outcome != request.Outcome || response.BindingID != request.BindingID || response.Status != "completed" {
		return CompletionReceipt{}, ErrUnavailable
	}
	return response, nil
}

// Revoke stops new Auth claims and returns either a durable pending drain
// status or a completed revoke receipt. GIS retries the same operation ID.
func (c *Client) Revoke(ctx context.Context, operationID uuid.UUID) (RevokeReceipt, error) {
	if operationID == uuid.Nil {
		return RevokeReceipt{}, ErrUnavailable
	}
	var response RevokeReceipt
	if err := c.post(ctx, revokePath, RevokeRequest{OperationID: operationID}, &response); err != nil {
		return RevokeReceipt{}, err
	}
	if response.OperationID != operationID || response.AuthorityRevision < 0 ||
		(response.Status != "revoking" && response.Status != "revoked") {
		return RevokeReceipt{}, ErrUnavailable
	}
	return response, nil
}

func (c *Client) post(ctx context.Context, path string, input, output any) error {
	if c == nil || c.http == nil || c.baseURL == nil || ctx == nil {
		return ErrUnavailable
	}
	endpoint := *c.baseURL
	endpoint.Path = path
	body, err := json.Marshal(input)
	if err != nil {
		return ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" ||
		!strings.EqualFold(response.Header.Get("Content-Type"), "application/json") {
		return ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil || len(raw) == 0 || len(raw) > maxResponse {
		return ErrUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil || decoder.Decode(new(any)) != io.EOF {
		return ErrUnavailable
	}
	return nil
}
