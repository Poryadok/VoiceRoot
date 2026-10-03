package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/protocol"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresMediaIssuerDerivesCanonicalRouteAndEnforcesMTLSRoleAndCompleteTuple(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	public, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca, serverCert, operatorCert, nodeCert, foreignCert, voiceCert := q11Certificates(t)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "issuer", Issuer: "master", Environment: "sandbox"}
	node, space, otherSpace := uuid.NewString(), uuid.NewString(), uuid.NewString()
	require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://canonical-node.test", q11Fingerprint(nodeCert.Leaf)))
	nodeCredential, err := store.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	for _, id := range []string{space, otherSpace} {
		require.NoError(t, store.place(ctx, node, id))
	}
	mapping := HostedResourceMapping{ResourceID: uuid.NewString(), ResourceType: "voice_room", SpaceID: space, HomeNodeID: node, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"voice"}, RoomName: "canonical-room-" + uuid.NewString()}
	_, err = store.registerHostedResource(ctx, mapping)
	require.NoError(t, err)
	request := mediaauthority.Request{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SpaceID: space, ResourceID: mapping.ResourceID, RoomName: mapping.RoomName, RoutingGeneration: 1, SessionEpoch: 4,
		ApplicationID: uuid.NewString(), EnvironmentID: uuid.NewString(), BindingID: uuid.NewString(), InstallationID: uuid.NewString()}
	publish := func(revision int64, allow bool) Snapshot {
		permissions := []Permission{}
		if allow {
			permissions = append(permissions, Permission{AccountID: request.AccountID, ProfileID: request.ProfileID, ResourceID: request.ResourceID, SessionEpoch: request.SessionEpoch, Actions: []string{"media"}, RoutingGeneration: request.RoutingGeneration, RoomName: request.RoomName, ApplicationID: request.ApplicationID, EnvironmentID: request.EnvironmentID, BindingID: request.BindingID, InstallationID: request.InstallationID})
		}
		snapshot := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: revision, ValidUntil: time.Now().Add(1900 * time.Millisecond).UnixMilli(), Permissions: permissions}
		require.NoError(t, store.publish(ctx, node, space, snapshot))
		return snapshot
	}
	snapshot := publish(1, true)
	server := httptest.NewUnstartedServer(&authorityAPI{Store: store, Operators: map[string]bool{q11Fingerprint(operatorCert.Leaf): true}, MediaIssuers: map[string]bool{q11Fingerprint(voiceCert.Leaf): true}})
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientCAs: ca, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	t.Cleanup(server.Close)
	client := func(cert tls.Certificate) *http.Client {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: ca, Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}}, Timeout: 2 * time.Second}
	}
	voiceClient := client(voiceCert)
	discovery := mediaauthority.RouteRequest{AccountID: request.AccountID, ProfileID: request.ProfileID, SpaceID: space, ResourceID: request.ResourceID, RoomName: request.RoomName, SessionEpoch: request.SessionEpoch}
	response, resolvedRaw := q11Request(t, ctx, voiceClient, server.URL, "POST", "/internal/v1/media-routes", discovery, uuid.NewString(), "")
	require.Equal(t, 200, response.StatusCode)
	var resolved mediaauthority.RouteResult
	require.NoError(t, json.Unmarshal(resolvedRaw, &resolved))
	require.True(t, resolved.Hosted)
	require.Equal(t, request, *resolved.Request, "only the current master route/policy may supply application binding and routing generation")
	foreignSpace := discovery
	foreignSpace.SpaceID = otherSpace
	response, _ = q11Request(t, ctx, voiceClient, server.URL, "POST", "/internal/v1/media-routes", foreignSpace, uuid.NewString(), "")
	require.Equal(t, 403, response.StatusCode, "global canonical resource mapped to another Space is denied, never not-hosted fallback")
	call := func(c *http.Client, input any) (int, []byte) {
		response, raw := q11Request(t, ctx, c, server.URL, "POST", "/internal/v1/media-grants", input, uuid.NewString(), "")
		return response.StatusCode, raw
	}
	status, raw := call(voiceClient, request)
	require.Equal(t, 200, status)
	var issued mediaauthority.Result
	require.NoError(t, json.Unmarshal(raw, &issued))
	verifier := mediaauthority.Verifier{Issuer: store.Issuer, Environment: store.Environment, NodeID: node, Keys: map[string]ed25519.PublicKey{"issuer": public}}
	grant, err := verifier.Verify(issued.Credential, mapping.RoomName, request.ProfileID, time.Now(), 0)
	require.NoError(t, err)
	require.Equal(t, node, grant.NodeID)
	require.Equal(t, request.ApplicationID, grant.ApplicationID)
	require.Equal(t, request.EnvironmentID, grant.EnvironmentID)
	require.Equal(t, request.BindingID, grant.BindingID)
	require.Equal(t, request.InstallationID, grant.InstallationID)
	require.Equal(t, request.RoutingGeneration, grant.RoutingGeneration)
	require.Equal(t, "https://canonical-node.test", issued.NodeEndpoint)
	require.LessOrEqual(t, grant.ExpiresAt-grant.IssuedAt, mediaauthority.MaxGrantValidity.Milliseconds())
	_, err = uuid.Parse(grant.Nonce)
	require.NoError(t, err)
	status, secondRaw := call(voiceClient, request)
	require.Equal(t, 200, status)
	var second mediaauthority.Result
	require.NoError(t, json.Unmarshal(secondRaw, &second))
	secondGrant, err := verifier.Verify(second.Credential, mapping.RoomName, request.ProfileID, time.Now(), 0)
	require.NoError(t, err)
	require.NotEqual(t, grant.Nonce, secondGrant.Nonce)
	manifest, err := store.issue(ctx, node, space, q11Fingerprint(nodeCert.Leaf), nodeCredential.Secret, nil)
	require.NoError(t, err)
	page, err := store.issuePage(ctx, node, space, q11Fingerprint(nodeCert.Leaf), nodeCredential.Secret, 0)
	require.NoError(t, err)
	lease, err := store.issue(ctx, node, space, q11Fingerprint(nodeCert.Leaf), nodeCredential.Secret, &leaseRequest{Revision: 1, Hash: protocol.SnapshotDigest(snapshot), Nonce: uuid.NewString()})
	require.NoError(t, err)
	registry, err := mediaauthority.NewRegistry(verifier, 250*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, registry.Apply(mediaauthority.Bundle{Scope: grant.Scope(), Manifest: manifest, Pages: []protocol.Envelope{page}, Lease: lease}, time.Now()))
	_, err = registry.Admit(issued.Credential, mapping.RoomName, request.ProfileID, time.Now())
	require.NoError(t, err, "actual master-issued private grant must admit against actual signed policy")
	for _, cert := range []tls.Certificate{operatorCert, nodeCert, foreignCert} {
		status, _ := call(client(cert), request)
		require.Equal(t, 403, status, "operator/node/unregistered certificate cannot mint user media")
		response, _ := q11Request(t, ctx, client(cert), server.URL, "POST", "/internal/v1/media-routes", discovery, uuid.NewString(), "")
		require.Equal(t, 403, response.StatusCode, "same separated role protects canonical route discovery")
	}
	for name, change := range map[string]func(*mediaauthority.Request){
		"account":      func(r *mediaauthority.Request) { r.AccountID = uuid.NewString() },
		"profile":      func(r *mediaauthority.Request) { r.ProfileID = uuid.NewString() },
		"space":        func(r *mediaauthority.Request) { r.SpaceID = otherSpace },
		"resource":     func(r *mediaauthority.Request) { r.ResourceID = uuid.NewString() },
		"room":         func(r *mediaauthority.Request) { r.RoomName += "-substituted" },
		"route":        func(r *mediaauthority.Request) { r.RoutingGeneration++ },
		"session":      func(r *mediaauthority.Request) { r.SessionEpoch++ },
		"application":  func(r *mediaauthority.Request) { r.ApplicationID = uuid.NewString() },
		"environment":  func(r *mediaauthority.Request) { r.EnvironmentID = uuid.NewString() },
		"binding":      func(r *mediaauthority.Request) { r.BindingID = uuid.NewString() },
		"installation": func(r *mediaauthority.Request) { r.InstallationID = uuid.NewString() },
		"unscoped downgrade": func(r *mediaauthority.Request) {
			r.ApplicationID, r.EnvironmentID, r.BindingID, r.InstallationID = "", "", "", ""
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := request
			change(&changed)
			status, _ := call(voiceClient, changed)
			require.Equal(t, 403, status)
		})
	}
	unknown := map[string]any{}
	require.NoError(t, json.Unmarshal(mustJSON(t, request), &unknown))
	unknown["node_id"] = uuid.NewString()
	status, _ = call(voiceClient, unknown)
	require.Equal(t, 400, status, "caller cannot choose signing node or other unknown claims")
	publishEscalation := map[string]any{}
	require.NoError(t, json.Unmarshal(mustJSON(t, request), &publishEscalation))
	publishEscalation["can_publish"] = true
	status, _ = call(voiceClient, publishEscalation)
	require.Equal(t, 403, status, "listen-only projection must reject a publishing credential")
	mapping.RoutingGeneration, mapping.LifecycleState = 2, "frozen"
	_, err = store.registerHostedResource(ctx, mapping)
	require.NoError(t, err)
	status, _ = call(voiceClient, request)
	require.Equal(t, 403, status)
	request.RoutingGeneration = 2
	status, _ = call(voiceClient, request)
	require.Equal(t, 403, status, "even current route cannot mint for a frozen resource")
	publish(2, false)
	mapping.RoutingGeneration, mapping.LifecycleState = 3, "active"
	_, err = store.registerHostedResource(ctx, mapping)
	require.NoError(t, err)
	request.RoutingGeneration = 3
	status, _ = call(voiceClient, request)
	require.Equal(t, 403, status, "current route cannot mint from a complete policy without the permission")
	publish(3, true)
	status, _ = call(voiceClient, request)
	require.Equal(t, 200, status)
	legacyPolicy := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 4, ValidUntil: time.Now().Add(1900 * time.Millisecond).UnixMilli(), Permissions: []Permission{{AccountID: request.AccountID, ProfileID: request.ProfileID, ResourceID: request.ResourceID, SessionEpoch: request.SessionEpoch, Actions: []string{"media"}}}}
	require.NoError(t, store.publish(ctx, node, space, legacyPolicy))
	status, _ = call(voiceClient, request)
	require.Equal(t, 403, status, "legacy allowlist cannot mint a routed/app-scoped grant")
	publish(5, true)
	badRoleServer := httptest.NewUnstartedServer(&authorityAPI{Store: store, Operators: map[string]bool{q11Fingerprint(operatorCert.Leaf): true}, MediaIssuers: map[string]bool{q11Fingerprint(operatorCert.Leaf): true, q11Fingerprint(nodeCert.Leaf): true}})
	badRoleServer.TLS = server.TLS.Clone()
	badRoleServer.StartTLS()
	t.Cleanup(badRoleServer.Close)
	for _, cert := range []tls.Certificate{operatorCert, nodeCert} {
		response, _ := q11Request(t, ctx, client(cert), badRoleServer.URL, "POST", "/internal/v1/media-grants", request, uuid.NewString(), "")
		require.Equal(t, 403, response.StatusCode, "even configured overlap cannot turn operator/node into a media issuer")
	}
	missing := discovery
	missing.ResourceID = uuid.NewString()
	response, raw = q11Request(t, ctx, voiceClient, server.URL, "POST", "/internal/v1/media-routes", missing, uuid.NewString(), "")
	require.Equal(t, 200, response.StatusCode)
	var notHosted mediaauthority.RouteResult
	require.NoError(t, json.Unmarshal(raw, &notHosted))
	require.Equal(t, mediaauthority.RouteResult{Version: 1, Hosted: false}, notHosted)
	current := publish(6, true)
	otherBinding := current.Permissions[0]
	otherBinding.BindingID = uuid.NewString()
	current.Permissions = append(current.Permissions, otherBinding)
	current.Revision = 7
	require.NoError(t, store.publish(ctx, node, space, current))
	response, _ = q11Request(t, ctx, voiceClient, server.URL, "POST", "/internal/v1/media-routes", discovery, uuid.NewString(), "")
	require.Equal(t, 403, response.StatusCode, "lookup cannot choose an arbitrary binding from ambiguous policy")
	_, err = store.changeNode(ctx, node, "suspend", "", "operator")
	require.NoError(t, err)
	status, _ = call(voiceClient, request)
	require.Equal(t, 403, status)
}

func TestPostgresMediaIssuerRechecksClockAfterBlockedCanonicalRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "clock", Issuer: "master", Environment: "sandbox"}
	node, space := uuid.NewString(), uuid.NewString()
	require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://canonical-node.test", digest([]byte(node))))
	_, err = store.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	require.NoError(t, store.place(ctx, node, space))
	mapping := HostedResourceMapping{ResourceID: uuid.NewString(), ResourceType: "voice_room", SpaceID: space, HomeNodeID: node, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"voice"}, RoomName: "blocked-canonical-room"}
	_, err = store.registerHostedResource(ctx, mapping)
	require.NoError(t, err)
	request := mediaauthority.Request{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SpaceID: space, ResourceID: mapping.ResourceID, RoomName: mapping.RoomName, RoutingGeneration: 1, SessionEpoch: 1}
	until := time.Now().Add(1900 * time.Millisecond)
	snapshot := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: until.UnixMilli(), Permissions: []Permission{{AccountID: request.AccountID, ProfileID: request.ProfileID, ResourceID: request.ResourceID, SessionEpoch: 1, RoutingGeneration: 1, RoomName: request.RoomName, Actions: []string{"media"}}}}
	require.NoError(t, store.publish(ctx, node, space, snapshot))
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	var locked string
	require.NoError(t, blocker.QueryRow(ctx, `SELECT resource_id::text FROM federation_hosted_resources WHERE resource_id=$1 FOR UPDATE`, mapping.ResourceID).Scan(&locked))
	result := make(chan error, 1)
	go func() { _, err := store.issueMediaGrant(ctx, request); result <- err }()
	require.Eventually(t, func() bool {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FROM federation_hosted_resources%FOR UPDATE%')`).Scan(&blocked)
		return err == nil && blocked
	}, time.Second, 10*time.Millisecond)
	time.Sleep(max(0, time.Until(until.Add(50*time.Millisecond))))
	require.NoError(t, blocker.Commit(ctx))
	select {
	case err := <-result:
		require.ErrorIs(t, err, errForbidden, "source elapsed while canonical route read waited; no credential can be signed")
	case <-time.After(2 * time.Second):
		t.Fatal("media issuer did not finish after route lock released")
	}
}
