package s2s

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"voice/backend/messaging/internal/gameprotocol"
)

const gameMessageExecutionPermitPath = "/api/v1/auth/sdk/game-message/execution-permits"

type GameMessageExecutionPermitConfig struct {
	Endpoint, TLSCertFile, TLSKeyFile, CAFile string
}

type GameMessageExecutionPermitClient struct {
	endpoint *url.URL
	client   *http.Client
}

type gameMessageExecutionPermitRequest struct {
	OperationID   string `json:"operation_id"`
	RequestSHA256 string `json:"request_sha256"`
}

type gameMessageExecutionPermitResponse struct {
	PermitJWS string `json:"permit_jws"`
}

type gameMessageExecutionPermitCompletionRequest struct {
	OperationID string `json:"operation_id"`
	Outcome     string `json:"outcome"`
}

type gameMessageExecutionPermitCompletionResponse struct {
	PermitID    string `json:"permit_id"`
	OperationID string `json:"operation_id"`
	Outcome     string `json:"outcome"`
	Status      string `json:"status"`
}

func NewGameMessageExecutionPermitClient(config GameMessageExecutionPermitConfig) (*GameMessageExecutionPermitClient, error) {
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Path != gameMessageExecutionPermitPath || config.TLSCertFile == "" || config.TLSKeyFile == "" || config.CAFile == "" {
		return nil, errors.New("auth execution permit requires the fixed HTTPS endpoint and mTLS credentials")
	}
	certificate, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, errors.New("auth execution permit client certificate is invalid")
	}
	caPEM, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, errors.New("auth execution permit trust file is unavailable")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("auth execution permit trust file has no certificates")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("auth execution permit redirects are forbidden")
	}}
	return &GameMessageExecutionPermitClient{endpoint: endpoint, client: client}, nil
}

func (c *GameMessageExecutionPermitClient) Issue(ctx context.Context, authority gameprotocol.DeviceAuthority, operationID uuid.UUID, mutationBytes []byte) (string, error) {
	if c == nil || c.endpoint == nil || c.client == nil || authority.AssertionJWS == "" || operationID == uuid.Nil || len(mutationBytes) == 0 || len(mutationBytes) > 128*1024 {
		return "", errors.New("auth execution permit request is unavailable or incomplete")
	}
	digest := sha256.Sum256(mutationBytes)
	body, err := json.Marshal(gameMessageExecutionPermitRequest{OperationID: operationID.String(), RequestSHA256: hex.EncodeToString(digest[:])})
	if err != nil {
		return "", errors.New("auth execution permit request could not be encoded")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return "", errors.New("auth execution permit request could not be created")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-store")
	request.Header.Set("X-Voice-Device-Authority", authority.AssertionJWS)
	response, err := c.client.Do(request)
	if err != nil {
		return "", errors.New("auth execution permit request failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth execution permit denied with status %d", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/json" || !strings.EqualFold(response.Header.Get("Cache-Control"), "no-store") {
		return "", errors.New("auth execution permit response headers are invalid")
	}
	var record gameMessageExecutionPermitResponse
	if err := decodeStrictJSONBody(response.Body, 20*1024, &record, "permit_jws"); err != nil || record.PermitJWS == "" || len(record.PermitJWS) > 16*1024 {
		return "", errors.New("auth execution permit response is malformed")
	}
	return record.PermitJWS, nil
}

func (c *GameMessageExecutionPermitClient) Complete(ctx context.Context, permitJTI, gisPermitID, operationID uuid.UUID, outcome string) error {
	if c == nil || c.endpoint == nil || c.client == nil || permitJTI == uuid.Nil || gisPermitID == uuid.Nil || operationID == uuid.Nil || (outcome != "committed" && outcome != "aborted") {
		return errors.New("auth execution permit completion is unavailable or invalid")
	}
	target := *c.endpoint
	target.Path = gameMessageExecutionPermitPath + "/" + permitJTI.String() + "/completion"
	body, err := json.Marshal(gameMessageExecutionPermitCompletionRequest{OperationID: operationID.String(), Outcome: outcome})
	if err != nil {
		return errors.New("auth execution permit completion could not be encoded")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return errors.New("auth execution permit completion request could not be created")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-store")
	response, err := c.client.Do(request)
	if err != nil {
		return errors.New("auth execution permit completion request failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("auth execution permit completion denied with status %d", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/json" || !strings.EqualFold(response.Header.Get("Cache-Control"), "no-store") {
		return errors.New("auth execution permit completion response headers are invalid")
	}
	var receipt gameMessageExecutionPermitCompletionResponse
	if err := decodeStrictJSONBody(response.Body, 4*1024, &receipt, "permit_id", "operation_id", "outcome", "status"); err != nil || receipt.Status != "completed" || receipt.Outcome != outcome || !canonicalIDEqual(receipt.PermitID, gisPermitID) || !canonicalIDEqual(receipt.OperationID, operationID) {
		return errors.New("auth execution permit completion receipt is malformed or mismatched")
	}
	return nil
}

func decodeStrictJSONBody(reader io.Reader, limit int64, target any, expectedFields ...string) error {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil || int64(len(data)) > limit {
		return errors.New("JSON response exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("JSON response must be an object")
	}
	fields := make(map[string]json.RawMessage, len(expectedFields))
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok {
			return errors.New("JSON response has an invalid field")
		}
		if _, duplicate := fields[key]; duplicate {
			return errors.New("JSON response contains a duplicate field")
		}
		known := false
		for _, field := range expectedFields {
			known = known || key == field
		}
		if !known {
			return errors.New("JSON response contains an unknown field")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		fields[key] = value
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	if closing != json.Delim('}') {
		return errors.New("JSON response object is unterminated")
	}
	if len(fields) != len(expectedFields) {
		return errors.New("JSON response omits a required field")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("JSON response contains trailing data")
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(canonical, target); err != nil {
		return err
	}
	return nil
}

func canonicalIDEqual(value string, expected uuid.UUID) bool {
	id, err := uuid.Parse(value)
	return err == nil && id == expected && id.String() == value
}

func (c *GameMessageExecutionPermitClient) Close() {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
}
