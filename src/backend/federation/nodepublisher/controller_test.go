package nodepublisher

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/protocol"
)

type recordedSink struct {
	mu      sync.Mutex
	bundles map[string]mediaauthority.Bundle
	fail    bool
	writes  map[string]int
}

func (s *recordedSink) Publish(_ context.Context, space string, b mediaauthority.Bundle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return ErrUnavailable
	}
	s.bundles[space] = b
	s.writes[space]++
	return nil
}

type authorityFixture struct {
	mu                         sync.Mutex
	key                        ed25519.PrivateKey
	config                     Config
	bundles                    map[string]mediaauthority.Bundle
	revisions                  map[string]int64
	badPage, foreign, redirect string
	acks                       map[string]int
	badLease                   bool
	badBoot                    bool
}

func (f *authorityFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.TLS == nil || len(r.TLS.PeerCertificates) != 1 || r.Header.Get("Authorization") != "Bearer "+f.config.Credential {
		w.WriteHeader(403)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 6 || parts[2] != f.config.NodeID {
		w.WriteHeader(404)
		return
	}
	space := parts[4]
	if f.redirect == space {
		w.Header().Set("Location", "https://example.invalid/")
		w.WriteHeader(307)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	var envelope protocol.Envelope
	if parts[5] == "snapshot" && len(parts) == 6 {
		f.revisions[space]++
		scope := protocol.Scope{Issuer: f.config.Issuer, Environment: f.config.Environment, NodeID: f.config.NodeID, SpaceID: space, Generation: 1, Epoch: 1}
		if f.foreign == space {
			scope.SpaceID = uuid.NewString()
		}
		now := time.Now()
		snapshot := protocol.Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: f.revisions[space], ValidUntil: now.Add(1900 * time.Millisecond).UnixMilli(), Permissions: []protocol.Permission{}}
		pages, err := protocol.SignSnapshotPages(f.key, "master", scope, snapshot, now)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		manifest := protocol.SnapshotManifest{Version: 1, Complete: true, PageCount: 1, Revision: snapshot.Revision, ValidUntil: snapshot.ValidUntil, TotalHash: protocol.SnapshotDigest(snapshot)}
		signed, err := protocol.SignSnapshotManifest(f.key, "master", scope, manifest, now)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		lease, err := protocol.SignEnvelope(f.key, "master", protocol.Claims{Version: 1, Kind: "lease", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment, NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: 1, Epoch: 1, Revision: snapshot.Revision, IssuedAt: now.UnixMilli(), ExpiresAt: snapshot.ValidUntil, Hash: manifest.TotalHash})
		if err != nil {
			w.WriteHeader(500)
			return
		}
		f.bundles[space] = mediaauthority.Bundle{Scope: scope, Manifest: signed, Pages: pages, Lease: lease}
		envelope = signed
	} else if parts[5] == "snapshot" && len(parts) == 8 {
		if f.badPage == space {
			w.WriteHeader(503)
			return
		}
		envelope = f.bundles[space].Pages[0]
	} else if parts[5] == "lease" && r.Method == http.MethodPost {
		var ack protocol.AppliedRevisionAck
		if json.NewDecoder(r.Body).Decode(&ack) != nil {
			w.WriteHeader(400)
			return
		}
		claims, err := protocol.VerifyEnvelope(f.key.Public().(ed25519.PublicKey), f.bundles[space].Manifest, time.Now())
		if err != nil || ack.Revision != claims.Revision || ack.Hash != claims.Hash {
			w.WriteHeader(409)
			return
		}
		if id, err := uuid.Parse(ack.Nonce); err != nil || id == uuid.Nil {
			w.WriteHeader(400)
			return
		}
		f.acks[space]++
		envelope = f.bundles[space].Lease
		if len(ack.ReceiverBootNonces) > 0 {
			lease, err := protocol.VerifyEnvelope(f.key.Public().(ed25519.PublicKey), envelope, time.Now())
			if err != nil {
				w.WriteHeader(409)
				return
			}
			lease.ReceiverBootNonces = ack.ReceiverBootNonces
			if f.badBoot {
				lease.ReceiverBootNonces = []string{uuid.NewString()}
			}
			envelope, err = protocol.SignEnvelope(f.key, "master", lease)
			if err != nil {
				w.WriteHeader(500)
				return
			}
		}
		if f.badLease {
			envelope.Signature = base64.RawURLEncoding.EncodeToString(make([]byte, 64))
		}
	} else {
		w.WriteHeader(404)
		return
	}
	_ = json.NewEncoder(w).Encode(envelope)
}

func publisherFixture(t *testing.T) (*Controller, *authorityFixture, *recordedSink) {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	credential := make([]byte, 32)
	_, err = rand.Read(credential)
	require.NoError(t, err)
	sink := &recordedSink{bundles: map[string]mediaauthority.Bundle{}, writes: map[string]int{}}
	config := Config{Issuer: "master", Environment: "test", NodeID: uuid.NewString(), Credential: base64.RawURLEncoding.EncodeToString(credential), Spaces: []string{uuid.NewString(), uuid.NewString()}, Keys: map[string]ed25519.PublicKey{"master": public}, Sink: sink, Interval: 100 * time.Millisecond}
	fixture := &authorityFixture{key: key, config: config, bundles: map[string]mediaauthority.Bundle{}, revisions: map[string]int64{}, acks: map[string]int{}}
	server := httptest.NewUnstartedServer(fixture)
	server.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	client := server.Client()
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	transport.TLSClientConfig.Certificates = server.TLS.Certificates
	config.MasterURL = server.URL
	config.Client = client
	controller, err := New(config)
	require.NoError(t, err)
	return controller, fixture, sink
}

func TestControllerAcknowledgesOnlyCompleteSignedPolicyAndKeepsLastFile(t *testing.T) {
	c, f, s := publisherFixture(t)
	space := c.config.Spaces[0]
	require.NoError(t, c.Refresh(context.Background(), space))
	before := s.bundles[space]
	f.badPage = space
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Equal(t, 1, f.acks[space], "partial snapshot cannot mint a lease")
	require.Equal(t, before, s.bundles[space], "partial policy never replaces the last complete file")
	f.badPage = ""
	f.badLease = true
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Equal(t, before, s.bundles[space], "invalid lease never publishes")
	f.badLease = false
	s.fail = true
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Equal(t, before, s.bundles[space])
	s.fail = false
	require.NoError(t, c.Refresh(context.Background(), space))
	require.Equal(t, 2, s.writes[space])
}

func TestControllerRejectsForeignScopeAndRedirectWithoutAcknowledgement(t *testing.T) {
	c, f, s := publisherFixture(t)
	space := c.config.Spaces[0]
	f.foreign = space
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Zero(t, f.acks[space])
	require.Zero(t, s.writes[space])
	f.foreign = ""
	f.redirect = space
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Zero(t, f.acks[space])
	require.Zero(t, s.writes[space])
	config := c.config
	config.MasterURL = "http://127.0.0.1:1"
	_, err := New(config)
	require.ErrorIs(t, err, ErrConfig)
	config = c.config
	config.Spaces = append(config.Spaces, config.Spaces[0])
	_, err = New(config)
	require.ErrorIs(t, err, ErrConfig)
}

func TestControllerIndependentSpaceRefreshSurvivesAnotherSpaceFailure(t *testing.T) {
	c, f, s := publisherFixture(t)
	bad, good := c.config.Spaces[0], c.config.Spaces[1]
	f.badPage = bad
	ctx, cancel := context.WithTimeout(context.Background(), 450*time.Millisecond)
	defer cancel()
	c.Run(ctx, nil)
	s.mu.Lock()
	defer s.mu.Unlock()
	require.Zero(t, s.writes[bad])
	require.GreaterOrEqual(t, s.writes[good], 3)
}

func TestControllerRejectsFreshlySignedRevisionRollbackBeforeFileReplacement(t *testing.T) {
	c, f, s := publisherFixture(t)
	space := c.config.Spaces[0]
	require.NoError(t, c.Refresh(context.Background(), space))
	require.NoError(t, c.Refresh(context.Background(), space))
	before := s.bundles[space]
	f.revisions[space] = 0
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Equal(t, before, s.bundles[space])
	require.Equal(t, 2, s.writes[space], "a fresh signature cannot lower applied revision")
	require.Equal(t, 2, f.acks[space], "rolled-back policy cannot be acknowledged as applied")
}

func TestControllerRequiresLiveBootRequestAndExactSignedAcknowledgement(t *testing.T) {
	c, f, s := publisherFixture(t)
	c.config.BootRequestDirectory = t.TempDir()
	space := c.config.Spaces[0]
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Zero(t, f.acks[space], "no live receiver means no master lease request")
	require.Zero(t, s.writes[space])
	registry, err := mediaauthority.NewBootRegistry(mediaauthority.Verifier{Issuer: c.config.Issuer, Environment: c.config.Environment, NodeID: c.config.NodeID, Keys: c.config.Keys}, 250*time.Millisecond)
	require.NoError(t, err)
	heartbeat, err := mediaauthority.NewBootHeartbeat(registry, c.config.BootRequestDirectory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, heartbeat.Close()) })
	require.NoError(t, heartbeat.Pulse(time.Now()))
	f.badBoot = true
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Equal(t, 1, f.acks[space])
	require.Zero(t, s.writes[space], "valid signed lease for another boot cannot replace authority")
	f.badBoot = false
	require.NoError(t, c.Refresh(context.Background(), space))
	require.Equal(t, 1, s.writes[space])
	lease, err := protocol.VerifyEnvelope(c.config.Keys[s.bundles[space].Lease.KeyID], s.bundles[space].Lease, time.Now())
	require.NoError(t, err)
	require.Equal(t, []string{registry.BootNonce()}, lease.ReceiverBootNonces)
	require.NoError(t, heartbeat.Close())
	require.ErrorIs(t, c.Refresh(context.Background(), space), ErrUnavailable)
	require.Equal(t, 2, f.acks[space], "closed receiver must not receive fresh authority")
	require.Equal(t, 1, s.writes[space])
}
