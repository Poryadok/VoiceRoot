// Package nodepublisher materializes master-signed policy for the node SFU.
package nodepublisher

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/nodecache"
	"voice/backend/federation/protocol"
)

const MaxBundleBytes = 16 << 20

var ErrUnavailable = errors.New("node authority refresh unavailable")
var ErrConfig = errors.New("invalid node authority controller configuration")

type Sink interface {
	Publish(context.Context, string, mediaauthority.Bundle) error
}
type Config struct {
	MasterURL, Issuer, Environment, NodeID, Credential string
	Keys                                               map[string]ed25519.PublicKey
	Spaces                                             []string
	Client                                             *http.Client
	Sink                                               Sink
	Interval                                           time.Duration
	BootRequestDirectory                               string
}
type Controller struct {
	config   Config
	registry *mediaauthority.Registry
	spaces   map[string]*spaceState
	now      func() time.Time
}

type spaceState struct {
	mu       sync.Mutex
	floor    appliedFloor
	hasFloor bool
}

type appliedFloor struct {
	scope    protocol.Scope
	revision int64
	hash     string
}

func New(config Config) (*Controller, error) {
	endpoint, err := url.Parse(config.MasterURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.RawPath != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") || len(config.MasterURL) > 2048 || config.Client == nil || config.Sink == nil || len(config.Spaces) == 0 || len(config.Spaces) > 4096 || config.Interval < 50*time.Millisecond || config.Interval > 500*time.Millisecond {
		return nil, ErrConfig
	}
	credential, err := base64.RawURLEncoding.DecodeString(config.Credential)
	if err != nil || len(credential) != 32 || base64.RawURLEncoding.EncodeToString(credential) != config.Credential {
		return nil, ErrConfig
	}
	transport, ok := config.Client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil || transport.TLSClientConfig.InsecureSkipVerify || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 || len(transport.TLSClientConfig.Certificates) == 0 || transport.DialTLSContext != nil || transport.DialTLS != nil {
		return nil, ErrConfig
	}
	verifier := mediaauthority.Verifier{Issuer: config.Issuer, Environment: config.Environment, NodeID: config.NodeID, Keys: config.Keys}
	registry, err := mediaauthority.NewRegistry(verifier, nodecache.MaxClockUncertainty)
	if err != nil {
		return nil, ErrConfig
	}
	if config.BootRequestDirectory != "" {
		if _, err := mediaauthority.ActiveBootNonces(config.BootRequestDirectory, config.Issuer, config.Environment, config.NodeID, time.Now()); err != nil {
			return nil, ErrConfig
		}
	}
	copied := *config.Client
	isolated := transport.Clone()
	isolated.TLSClientConfig.RootCAs = transport.TLSClientConfig.RootCAs.Clone()
	copied.Transport = isolated
	copied.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copied.Timeout = 2 * time.Second
	config.Client = &copied
	config.MasterURL = strings.TrimSuffix(config.MasterURL, "/")
	config.Keys = make(map[string]ed25519.PublicKey, len(verifier.Keys))
	for id, key := range verifier.Keys {
		config.Keys[id] = append(ed25519.PublicKey(nil), key...)
	}
	config.Spaces = append([]string(nil), config.Spaces...)
	sort.Strings(config.Spaces)
	spaces := map[string]*spaceState{}
	for _, space := range config.Spaces {
		id, err := uuid.Parse(space)
		if err != nil || id == uuid.Nil || id.String() != space || spaces[space] != nil {
			return nil, ErrConfig
		}
		spaces[space] = &spaceState{}
	}
	return &Controller{config: config, registry: registry, spaces: spaces, now: time.Now}, nil
}

// Refresh does not acknowledge a partial policy. Fetch/verification errors retain
// the last file. A sink error after rename can mean a complete replacement with
// uncertain durability; signed expiry and the SFU watchdog remain authoritative.
func (c *Controller) Refresh(parent context.Context, space string) error {
	if c == nil || c.spaces[space] == nil {
		return ErrConfig
	}
	state := c.spaces[space]
	state.mu.Lock()
	defer state.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	remaining := MaxBundleBytes
	manifest, err := c.fetch(ctx, space, "snapshot", nil, &remaining)
	if err != nil {
		return err
	}
	claims, err := protocol.VerifyEnvelope(c.config.Keys[manifest.KeyID], manifest, c.now())
	if err != nil || claims.Kind != "snapshot_manifest" || claims.Issuer != c.config.Issuer || claims.Environment != c.config.Environment || claims.NodeID != c.config.NodeID || claims.SpaceID != space {
		return ErrUnavailable
	}
	scope := protocol.Scope{Issuer: claims.Issuer, Environment: claims.Environment, NodeID: claims.NodeID, SpaceID: space, Generation: claims.Generation, Epoch: claims.Epoch}
	candidate := nodecache.New(scope, c.config.Keys)
	if candidate.StageManifest(manifest, c.now()) != nil {
		return ErrUnavailable
	}
	bundle := mediaauthority.Bundle{Scope: scope, Manifest: manifest}
	for page := 0; page < claims.Manifest.PageCount; page++ {
		envelope, err := c.fetch(ctx, space, "snapshot/pages/"+strconv.Itoa(page), nil, &remaining)
		if err != nil || candidate.StagePage(envelope, c.now()) != nil {
			return ErrUnavailable
		}
		bundle.Pages = append(bundle.Pages, envelope)
	}
	applied := candidate.Current()
	if applied.Revision != claims.Revision || protocol.SnapshotDigest(applied) != claims.Hash {
		return ErrUnavailable
	}
	// Never acknowledge a policy which cannot advance the node's verified
	// floor. A fresh signature does not permit generation/epoch/revision rollback.
	if previous := state.floor; state.hasFloor {
		if scope.Generation < previous.scope.Generation || scope.Epoch < previous.scope.Epoch {
			return ErrUnavailable
		}
		if scope == previous.scope && (claims.Revision < previous.revision || (claims.Revision == previous.revision && claims.Hash != previous.hash)) {
			return ErrUnavailable
		}
	}
	ack := protocol.AppliedRevisionAck{Revision: applied.Revision, Hash: claims.Hash, Nonce: uuid.NewString()}
	if c.config.BootRequestDirectory != "" {
		var err error
		ack.ReceiverBootNonces, err = mediaauthority.ActiveBootNonces(c.config.BootRequestDirectory, c.config.Issuer, c.config.Environment, c.config.NodeID, c.now())
		if err != nil || len(ack.ReceiverBootNonces) == 0 {
			return ErrUnavailable
		}
	}
	lease, err := c.fetch(ctx, space, "lease", &ack, &remaining)
	if err != nil || candidate.AcceptLease(lease, c.now()) != nil {
		return ErrUnavailable
	}
	leaseClaims, err := protocol.VerifyEnvelope(c.config.Keys[lease.KeyID], lease, c.now())
	if err != nil || !slices.Equal(leaseClaims.ReceiverBootNonces, ack.ReceiverBootNonces) {
		return ErrUnavailable
	}
	bundle.Lease = lease
	// The shared verifier preserves process-local generation/epoch/revision
	// floors and rejects replay conflicts before files visible to the SFU change.
	if c.registry.Apply(bundle, c.now()) != nil {
		return ErrUnavailable
	}
	state.floor = appliedFloor{scope: scope, revision: claims.Revision, hash: claims.Hash}
	state.hasFloor = true
	raw, err := json.Marshal(bundle)
	if err != nil || len(raw) > MaxBundleBytes {
		return ErrUnavailable
	}
	if ctx.Err() != nil || c.config.Sink.Publish(ctx, space, bundle) != nil {
		return ErrUnavailable
	}
	return nil
}

func (c *Controller) fetch(ctx context.Context, space, suffix string, ack *protocol.AppliedRevisionAck, remaining *int) (protocol.Envelope, error) {
	var body io.Reader
	method := http.MethodGet
	if ack != nil {
		raw, err := json.Marshal(ack)
		if err != nil {
			return protocol.Envelope{}, ErrUnavailable
		}
		body = bytes.NewReader(raw)
		method = http.MethodPost
	}
	request, err := http.NewRequestWithContext(ctx, method, c.config.MasterURL+"/v1/nodes/"+c.config.NodeID+"/spaces/"+space+"/"+suffix, body)
	if err != nil {
		return protocol.Envelope{}, ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+c.config.Credential)
	request.Header.Set("X-Request-ID", uuid.NewString())
	if ack != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.config.Client.Do(request)
	if err != nil {
		return protocol.Envelope{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" || *remaining <= 0 {
		return protocol.Envelope{}, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, int64(*remaining+1)))
	if err != nil || len(raw) > *remaining {
		return protocol.Envelope{}, ErrUnavailable
	}
	*remaining -= len(raw)
	var envelope protocol.Envelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&envelope) != nil || decoder.Decode(new(any)) != io.EOF {
		return protocol.Envelope{}, ErrUnavailable
	}
	return envelope, nil
}

// Run gives each Space an independent refresh loop. One unavailable Space
// cannot prevent another's lease from being renewed. report receives only the
// Space ID and a generic error, never credentials, URLs or policy bytes.
func (c *Controller) Run(ctx context.Context, report func(string, error)) {
	var wait sync.WaitGroup
	for _, space := range c.config.Spaces {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ticker := time.NewTicker(c.config.Interval)
			defer ticker.Stop()
			for {
				err := c.Refresh(ctx, space)
				if report != nil {
					report(space, err)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	wait.Wait()
}
