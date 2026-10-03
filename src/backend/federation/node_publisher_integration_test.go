package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/nodepublisher"
	"voice/backend/federation/protocol"
	"voice/backend/pkg/integrationtest"
)

type publisherAcceptanceSink struct {
	mu      sync.Mutex
	bundles map[string]mediaauthority.Bundle
}

func (s *publisherAcceptanceSink) Publish(_ context.Context, space string, b mediaauthority.Bundle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bundles[space] = b
	return nil
}

func TestPostgresNodePublisherConsumesActualMTLSAuthorityAPIAndRejectsSuspension(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	public, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca, serverCert, _, nodeCert, _, _ := q11Certificates(t)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "publisher", Issuer: "master", Environment: "sandbox"}
	node, spaceA, spaceB := uuid.NewString(), uuid.NewString(), uuid.NewString()
	require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://node.test", q11Fingerprint(nodeCert.Leaf)))
	credential, err := store.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	permission := protocol.Permission{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 1, Actions: []string{"media"}}
	for _, space := range []string{spaceA, spaceB} {
		require.NoError(t, store.place(ctx, node, space))
		require.NoError(t, store.publish(ctx, node, space, Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: time.Now().Add(1900 * time.Millisecond).UnixMilli(), Permissions: []Permission{permission}}))
	}
	server := httptest.NewUnstartedServer(&authorityAPI{Store: store, Operators: map[string]bool{}})
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: ca, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: ca, Certificates: []tls.Certificate{nodeCert}, MinVersion: tls.VersionTLS13}}}
	sink := &publisherAcceptanceSink{bundles: map[string]mediaauthority.Bundle{}}
	verifier := mediaauthority.Verifier{Issuer: store.Issuer, Environment: store.Environment, NodeID: node, Keys: map[string]ed25519.PublicKey{"publisher": public}}
	registry, err := mediaauthority.NewBootRegistry(verifier, 250*time.Millisecond)
	require.NoError(t, err)
	directory := t.TempDir()
	heartbeat, err := mediaauthority.NewBootHeartbeat(registry, directory)
	require.NoError(t, err)
	defer heartbeat.Close()
	require.NoError(t, heartbeat.Pulse(time.Now()))
	controller, err := nodepublisher.New(nodepublisher.Config{MasterURL: server.URL, Issuer: store.Issuer, Environment: store.Environment, NodeID: node, Credential: credential.Secret, Keys: map[string]ed25519.PublicKey{"publisher": public}, Spaces: []string{spaceA, spaceB}, Client: client, Sink: sink, Interval: 100 * time.Millisecond, BootRequestDirectory: directory})
	require.NoError(t, err)
	for _, space := range []string{spaceA, spaceB} {
		require.NoError(t, controller.Refresh(ctx, space))
		require.NoError(t, registry.Apply(sink.bundles[space], time.Now()))
	}
	restarted, err := mediaauthority.NewBootRegistry(verifier, 250*time.Millisecond)
	require.NoError(t, err)
	require.ErrorIs(t, restarted.Apply(sink.bundles[spaceA], time.Now()), mediaauthority.ErrDenied, "saved unexpired bundle cannot activate a new boot")
	restartedHeartbeat, err := mediaauthority.NewBootHeartbeat(restarted, directory)
	require.NoError(t, err)
	defer restartedHeartbeat.Close()
	require.NoError(t, restartedHeartbeat.Pulse(time.Now()))
	require.NoError(t, controller.Refresh(ctx, spaceA), "new receiver needs actual mTLS master ACK")
	require.NoError(t, restarted.Apply(sink.bundles[spaceA], time.Now()))
	before := sink.bundles[spaceA]
	_, err = store.changeNode(ctx, node, "suspend", "", "operator")
	require.NoError(t, err)
	require.ErrorIs(t, controller.Refresh(ctx, spaceA), nodepublisher.ErrUnavailable)
	require.Equal(t, before, sink.bundles[spaceA], "suspension cannot replace authority with unsigned/error output")
	var leases int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_lease_nonces WHERE node_id=$1`, node).Scan(&leases))
	require.Equal(t, 3, leases, "two Space ACKs and a fresh boot-bound master round trip")
}
