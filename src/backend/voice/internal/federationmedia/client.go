// Package federationmedia exchanges only master-signed narrow media authority
// at the registered node. Master user credentials and node SFU secrets stay in
// their own trust domains.
package federationmedia

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"voice/backend/federation/mediaauthority"
)

var ErrDenied = errors.New("federated media denied")
var ErrUnavailable = errors.New("federated media unavailable")
var ErrNotHosted = errors.New("resource is not hosted on a node")

type Config struct {
	MasterURL, Issuer, Environment string
	Keys                           map[string]ed25519.PublicKey
	MasterClient, NodeClient       *http.Client
}
type Client struct {
	config Config
}

func rootURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "" && u.Opaque == ""
}
func safeClient(source *http.Client, master bool) (*http.Client, error) {
	if source == nil || source.Timeout <= 0 || source.Timeout > 2*time.Second {
		return nil, ErrUnavailable
	}
	t, ok := source.Transport.(*http.Transport)
	if !ok || t.TLSClientConfig == nil || t.TLSClientConfig.RootCAs == nil || t.TLSClientConfig.InsecureSkipVerify || t.DialTLS != nil || t.DialTLSContext != nil || t.TLSClientConfig.GetClientCertificate != nil || (master && len(t.TLSClientConfig.Certificates) == 0) || (!master && len(t.TLSClientConfig.Certificates) != 0) { //nolint:staticcheck // Deny deprecated custom TLS dialers as well as context-aware ones.
		return nil, ErrUnavailable
	}
	transport := t.Clone()
	transport.TLSClientConfig = t.TLSClientConfig.Clone()
	transport.TLSClientConfig.RootCAs = t.TLSClientConfig.RootCAs.Clone()
	transport.TLSClientConfig.MinVersion = 0x0304
	client := *source
	client.Transport = transport
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client, nil
}
func New(config Config) (*Client, error) {
	if !rootURL(config.MasterURL) || config.Issuer == "" || config.Environment == "" || len(config.Keys) == 0 {
		return nil, ErrUnavailable
	}
	keys := map[string]ed25519.PublicKey{}
	for id, key := range config.Keys {
		if id == "" || len(key) != ed25519.PublicKeySize {
			return nil, ErrUnavailable
		}
		keys[id] = append(ed25519.PublicKey(nil), key...)
	}
	config.Keys = keys
	var err error
	config.MasterClient, err = safeClient(config.MasterClient, true)
	if err != nil {
		return nil, err
	}
	config.NodeClient, err = safeClient(config.NodeClient, false)
	if err != nil {
		return nil, err
	}
	return &Client{config: config}, nil
}
func post(ctx context.Context, client *http.Client, address string, input, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return ErrUnavailable
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(raw))
	if err != nil {
		return ErrUnavailable
	}
	r.Header.Set("Content-Type", "application/json")
	response, err := client.Do(r)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == 403 || response.StatusCode == 401 {
		return ErrDenied
	}
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" {
		return ErrUnavailable
	}
	raw, err = io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return ErrUnavailable
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(output) != nil || d.Decode(new(any)) != io.EOF {
		return ErrUnavailable
	}
	return nil
}

func (c *Client) JoinToken(ctx context.Context, input mediaauthority.RouteRequest) (mediaauthority.ExchangeResult, error) {
	if c == nil || input.Validate() != nil {
		return mediaauthority.ExchangeResult{}, ErrDenied
	}
	var route mediaauthority.RouteResult
	if err := post(ctx, c.config.MasterClient, c.config.MasterURL+"/internal/v1/media-routes", input, &route); err != nil {
		return mediaauthority.ExchangeResult{}, err
	}
	if route.Version != 1 || (route.Hosted && route.Request == nil) || (!route.Hosted && route.Request != nil) {
		return mediaauthority.ExchangeResult{}, ErrUnavailable
	}
	if !route.Hosted {
		return mediaauthority.ExchangeResult{}, ErrNotHosted
	}
	request := *route.Request
	if request.Validate() != nil || request.AccountID != input.AccountID || request.ProfileID != input.ProfileID || request.SpaceID != input.SpaceID || request.ResourceID != input.ResourceID || request.SessionEpoch != input.SessionEpoch || request.RoomName != input.RoomName || request.CanPublish != input.CanPublish {
		return mediaauthority.ExchangeResult{}, ErrDenied
	}
	var issued mediaauthority.Result
	if err := post(ctx, c.config.MasterClient, c.config.MasterURL+"/internal/v1/media-grants", request, &issued); err != nil {
		return mediaauthority.ExchangeResult{}, err
	}
	node, err := uuid.Parse(issued.NodeID)
	if err != nil || node == uuid.Nil || node.String() != issued.NodeID || !rootURL(issued.NodeEndpoint) || issued.SpaceID != request.SpaceID || issued.ResourceID != request.ResourceID || issued.RoomName != request.RoomName || issued.RoutingGeneration != request.RoutingGeneration {
		return mediaauthority.ExchangeResult{}, ErrDenied
	}
	verifier := mediaauthority.Verifier{Issuer: c.config.Issuer, Environment: c.config.Environment, NodeID: issued.NodeID, Keys: c.config.Keys}
	grant, err := verifier.Verify(issued.Credential, request.RoomName, request.ProfileID, time.Now(), 250*time.Millisecond)
	if err != nil || grant.ExpiresAt != issued.ExpiresAt || grant.AccountID != request.AccountID || grant.SpaceID != request.SpaceID || grant.ResourceID != request.ResourceID || grant.SessionEpoch != request.SessionEpoch || grant.RoutingGeneration != request.RoutingGeneration || grant.CanPublish != request.CanPublish || grant.ApplicationID != request.ApplicationID || grant.EnvironmentID != request.EnvironmentID || grant.BindingID != request.BindingID || grant.InstallationID != request.InstallationID {
		return mediaauthority.ExchangeResult{}, ErrDenied
	}
	var result mediaauthority.ExchangeResult
	if err := post(ctx, c.config.NodeClient, issued.NodeEndpoint+"/v1/media/token", mediaauthority.ExchangeRequest{Credential: issued.Credential, RoomName: grant.RoomName, ProfileID: grant.ProfileID}, &result); err != nil {
		return mediaauthority.ExchangeResult{}, err
	}
	u, err := url.Parse(result.LivekitURL)
	if err != nil || u.Scheme != "wss" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || result.JWT == "" || len(result.JWT) > 65536 || result.ExpiresAt > grant.ExpiresAt || result.ExpiresAt <= time.Now().Add(250*time.Millisecond).UnixMilli() {
		return mediaauthority.ExchangeResult{}, ErrDenied
	}
	return result, nil
}
