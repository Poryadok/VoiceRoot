package s2s

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
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

const gameResourceMappingAuthorizationPath = "/internal/v1/game-integrations/resource-mappings/authorize-chat"
const messagingServiceURISAN = "spiffe://voice/service/messaging"

type GameResourceMappingAuthorizationConfig struct {
	Endpoint, TLSCertFile, TLSKeyFile, CAFile string
	WorkloadKeyBase64                         string
}

type GameResourceMappingAuthorizationClient struct {
	endpoint    *url.URL
	client      *http.Client
	workloadKey []byte
	now         func() time.Time
}

type authorizeGameChatMappingRequest struct {
	ApplicationID string `json:"application_id"`
	EnvironmentID string `json:"environment_id"`
	BindingID     string `json:"binding_id"`
	ChatID        string `json:"chat_id"`
}

type authorizeGameChatMappingResponse struct {
	Allowed         bool  `json:"allowed"`
	MappingRevision int64 `json:"mapping_revision,omitempty"`
}

func NewGameResourceMappingAuthorizationClient(config GameResourceMappingAuthorizationConfig) (*GameResourceMappingAuthorizationClient, error) {
	endpoint, err := url.Parse(strings.TrimSpace(config.Endpoint))
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" ||
		endpoint.Fragment != "" || endpoint.Path != gameResourceMappingAuthorizationPath || endpoint.RawPath != "" ||
		config.TLSCertFile == "" || config.TLSKeyFile == "" || config.CAFile == "" {
		return nil, errors.New("GIS resource mapping authorization requires the fixed HTTPS endpoint and mTLS credentials")
	}
	certificate, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil || len(certificate.Certificate) == 0 {
		return nil, errors.New("GIS resource mapping client certificate is invalid")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil || !containsMessagingURISAN(leaf) {
		return nil, errors.New("GIS resource mapping client certificate must have the Messaging service URI SAN")
	}
	caPEM, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, errors.New("GIS resource mapping CA is unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("GIS resource mapping CA has no certificates")
	}
	currentKey, err := decodeResourceMappingWorkloadKey(config.WorkloadKeyBase64)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{certificate},
		ServerName:   endpoint.Hostname(),
	}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("GIS resource mapping redirects are forbidden")
	}}
	return &GameResourceMappingAuthorizationClient{
		endpoint: endpoint, client: client, workloadKey: currentKey,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func decodeResourceMappingWorkloadKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("GIS resource mapping workload key must be a base64-encoded 32-byte key")
	}
	return key, nil
}

func containsMessagingURISAN(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	for _, uri := range cert.URIs {
		if uri != nil && uri.String() == messagingServiceURISAN {
			return true
		}
	}
	return false
}

// AuthorizeAppBindingChat implements the fail-closed mapping seam used before
// permit issue. GIS independently checks the exact tuple against its registry.
func (c *GameResourceMappingAuthorizationClient) AuthorizeAppBindingChat(ctx context.Context,
	authority gameprotocol.DeviceAuthority, message gameprotocol.Message) error {
	if c == nil || c.endpoint == nil || c.client == nil || len(c.workloadKey) != 32 || c.now == nil ||
		authority.ApplicationID == uuid.Nil || authority.EnvironmentID == uuid.Nil || authority.BindingID == uuid.Nil ||
		message.ChatID == uuid.Nil || message.ApplicationID != authority.ApplicationID ||
		message.EnvironmentID != authority.EnvironmentID || message.BindingID != authority.BindingID {
		return errors.New("GIS resource mapping authority is unavailable or mismatched")
	}
	body, err := json.Marshal(authorizeGameChatMappingRequest{
		ApplicationID: authority.ApplicationID.String(), EnvironmentID: authority.EnvironmentID.String(),
		BindingID: authority.BindingID.String(), ChatID: message.ChatID.String(),
	})
	if err != nil {
		return errors.New("GIS resource mapping request could not be encoded")
	}
	now := c.now().UTC()
	timestamp := fmt.Sprintf("%d", now.Unix())
	nonce := uuid.NewString()
	path := c.endpoint.EscapedPath()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return errors.New("GIS resource mapping request could not be created")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Voice-Workload", "messaging")
	request.Header.Set("X-Voice-Timestamp", timestamp)
	request.Header.Set("X-Voice-Nonce", nonce)
	request.Header.Set("X-Voice-Signature", workloadBodySignature(c.workloadKey, http.MethodPost, path, timestamp, nonce, body))
	response, err := c.client.Do(request)
	if err != nil {
		return errors.New("GIS resource mapping authorization request failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/json" ||
		!strings.EqualFold(response.Header.Get("Cache-Control"), "no-store") {
		return errors.New("GIS resource mapping authorization response is invalid")
	}
	responseTimestamp, err := uniqueResourceMappingHeader(response.Header, "X-Voice-Response-Timestamp")
	if err != nil || responseTimestamp != timestamp {
		return errors.New("GIS resource mapping response timestamp is invalid")
	}
	responseNonce, err := uniqueResourceMappingHeader(response.Header, "X-Voice-Response-Nonce")
	if err != nil || responseNonce != nonce {
		return errors.New("GIS resource mapping response nonce is invalid")
	}
	signature, err := uniqueResourceMappingHeader(response.Header, "X-Voice-Response-Signature")
	if err != nil {
		return errors.New("GIS resource mapping response signature is invalid")
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1025))
	if err != nil || len(responseBody) > 1024 || !c.verifyResponse(path, timestamp, nonce, responseBody, signature) {
		return errors.New("GIS resource mapping response proof is invalid")
	}
	var result authorizeGameChatMappingResponse
	if err := decodeCanonicalResourceMappingResponse(responseBody, &result); err != nil {
		return errors.New("GIS resource mapping response body is invalid")
	}
	if !result.Allowed {
		return fmt.Errorf("GIS resource mapping denied: allowed=%t mapping_revision=%d", result.Allowed, result.MappingRevision)
	}
	if result.MappingRevision <= 0 {
		return fmt.Errorf("GIS resource mapping revision is invalid: allowed=%t mapping_revision=%d", result.Allowed, result.MappingRevision)
	}
	return nil
}

func (c *GameResourceMappingAuthorizationClient) verifyResponse(path, timestamp, nonce string, body []byte, signature string) bool {
	return hmac.Equal([]byte(signature), []byte(workloadResponseSignature(
		c.workloadKey, http.StatusOK, path, timestamp, nonce, body)))
}

func workloadBodySignature(key []byte, method, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := "v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func workloadResponseSignature(key []byte, status int, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := fmt.Sprintf("v1\n%d\n%s\n%s\n%s\n%s", status, path, timestamp, nonce, hex.EncodeToString(digest[:]))
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func uniqueResourceMappingHeader(header http.Header, name string) (string, error) {
	values := header.Values(name)
	if len(values) != 1 || values[0] == "" {
		return "", errors.New("required unique response header is missing or duplicated")
	}
	return values[0], nil
}

func decodeCanonicalResourceMappingResponse(body []byte, target *authorizeGameChatMappingResponse) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("GIS resource mapping response contains trailing data")
	}
	canonical, err := json.Marshal(target)
	if err != nil {
		return err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(canonical, body) {
		return errors.New("GIS resource mapping response does not use the exact canonical body")
	}
	return nil
}

func (c *GameResourceMappingAuthorizationClient) Close() {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
}
