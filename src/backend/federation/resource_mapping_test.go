package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresHostedResourceRouteIsStableAndPlacementBound(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "test", Issuer: "master", Environment: "sandbox"}
	nodeID, nextNodeID, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	require.NoError(t, store.enroll(ctx, nodeID, uuid.NewString(), "https://node.test", digest([]byte("node-cert"))))
	_, err = store.changeNode(ctx, nodeID, "approve", "", "operator")
	require.NoError(t, err)
	require.NoError(t, store.place(ctx, nodeID, spaceID))
	require.NoError(t, store.enroll(ctx, nextNodeID, uuid.NewString(), "https://next-node.test", digest([]byte("next-node-cert"))))
	_, err = store.changeNode(ctx, nextNodeID, "approve", "", "operator")
	require.NoError(t, err)
	foreign := HostedResourceMapping{ResourceID: uuid.NewString(), ResourceType: "chat", SpaceID: spaceID, HomeNodeID: nextNodeID, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"messaging"}}
	_, err = store.registerHostedResource(ctx, foreign)
	require.ErrorIs(t, err, errForbidden, "an active node cannot register a resource for another node's Space placement")
	var foreignRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_hosted_resources WHERE resource_id=$1`, foreign.ResourceID).Scan(&foreignRows))
	require.Zero(t, foreignRows)

	requested := HostedResourceMapping{
		ResourceID: uuid.NewString(), ResourceType: "chat", SpaceID: spaceID, HomeNodeID: nodeID,
		RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"messaging", "file", "search"},
	}
	created, err := store.registerHostedResource(ctx, requested)
	require.NoError(t, err)
	require.EqualValues(t, 1, created.RoutingGeneration)
	require.Equal(t, []string{"file", "messaging", "search"}, created.Capabilities)

	replayed, err := store.registerHostedResource(ctx, requested)
	require.NoError(t, err)
	require.Equal(t, created, replayed, "an exact registration retry is inert")

	changed := requested
	changed.RoutingGeneration = 2
	moved := changed
	moved.HomeNodeID = nextNodeID
	_, err = store.registerHostedResource(ctx, moved)
	require.ErrorIs(t, err, errForbidden, "registration requires the exact current Space placement")
	changed.LifecycleState = "frozen"
	changed.Capabilities = []string{"file", "messaging", "search", "voice"}
	updated, err := store.registerHostedResource(ctx, changed)
	require.NoError(t, err)
	require.EqualValues(t, 2, updated.RoutingGeneration)
	require.Equal(t, nodeID, updated.HomeNodeID)

	conflictingGeneration := changed
	conflictingGeneration.Capabilities = []string{"file", "messaging", "search"}
	_, err = store.registerHostedResource(ctx, conflictingGeneration)
	require.ErrorIs(t, err, errConflict, "an immutable canonical ID cannot acquire a different route")

	foreignPlacement := requested
	foreignPlacement.HomeNodeID = uuid.NewString()
	_, err = store.registerHostedResource(ctx, foreignPlacement)
	require.ErrorIs(t, err, errForbidden, "a resource cannot be routed outside the Space placement")

	operatorCert := &x509.Certificate{Raw: []byte("operator-cert"), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	nodeCert := &x509.Certificate{Raw: []byte("node-cert"), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	api := &authorityAPI{Store: store, Operators: map[string]bool{digest(operatorCert.Raw): true}}
	apiCall := func(cert *x509.Certificate, routeNode, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "https://master/v1/nodes/"+routeNode+"/spaces/"+spaceID+"/resources/"+requested.ResourceID, strings.NewReader(body))
		req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}, VerifiedChains: [][]*x509.Certificate{{cert}}}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	body, err := json.Marshal(struct {
		ResourceType string   `json:"resource_type"`
		Generation   int64    `json:"routing_generation"`
		State        string   `json:"lifecycle_state"`
		Capabilities []string `json:"capabilities"`
	}{changed.ResourceType, changed.RoutingGeneration, changed.LifecycleState, changed.Capabilities})
	require.NoError(t, err)
	operatorResponse := apiCall(operatorCert, nodeID, string(body))
	require.Equal(t, 200, operatorResponse.Code, operatorResponse.Body.String())
	nodeResponse := apiCall(nodeCert, nodeID, string(body))
	require.Equal(t, 403, nodeResponse.Code, nodeResponse.Body.String(), "nodes cannot assign their own resource routes")

	var mappingCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_hosted_resources WHERE resource_id=$1`, requested.ResourceID).Scan(&mappingCount))
	require.Equal(t, 2, mappingCount, "each immutable route generation remains available for stale-route rejection")
	_, err = pool.Exec(ctx, `UPDATE federation_hosted_resources SET lifecycle_state='tombstoned' WHERE resource_id=$1 AND routing_generation=1`, requested.ResourceID)
	require.Error(t, err, "a saved route generation cannot be rewritten")
	_, err = pool.Exec(ctx, `DELETE FROM federation_hosted_resources WHERE resource_id=$1 AND routing_generation=1`, requested.ResourceID)
	require.Error(t, err, "a saved route generation cannot be removed")

	concurrentID := uuid.NewString()
	first := HostedResourceMapping{ResourceID: concurrentID, ResourceType: "chat", SpaceID: spaceID,
		HomeNodeID: nodeID, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"messaging"}}
	second := first
	second.ResourceType = "file"
	second.HomeNodeID = nextNodeID
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, candidate := range []HostedResourceMapping{first, second} {
		wait.Add(1)
		go func(candidate HostedResourceMapping) {
			defer wait.Done()
			<-start
			_, registerErr := store.registerHostedResource(ctx, candidate)
			results <- registerErr
		}(candidate)
	}
	close(start)
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for registerErr := range results {
		if registerErr == nil {
			successes++
		} else if errors.Is(registerErr, errForbidden) {
			conflicts++
		} else {
			t.Fatalf("concurrent route registration: %v", registerErr)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func TestHostedVoiceRoomRequiresExplicitCanonicalRoom(t *testing.T) {
	valid := HostedResourceMapping{ResourceID: uuid.NewString(), ResourceType: "voice_room", SpaceID: uuid.NewString(), HomeNodeID: uuid.NewString(), RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"voice"}, RoomName: "rtc-explicit-room"}
	_, err := normalizeHostedResource(valid)
	require.NoError(t, err)
	for name, change := range map[string]func(*HostedResourceMapping){
		"missing":                  func(m *HostedResourceMapping) { m.RoomName = "" },
		"whitespace":               func(m *HostedResourceMapping) { m.RoomName = " room" },
		"control":                  func(m *HostedResourceMapping) { m.RoomName = "room\nname" },
		"oversized":                func(m *HostedResourceMapping) { m.RoomName = strings.Repeat("r", 257) },
		"invalid UTF8":             func(m *HostedResourceMapping) { m.RoomName = string([]byte{0xff}) },
		"wrong type":               func(m *HostedResourceMapping) { m.ResourceType = "chat" },
		"missing voice capability": func(m *HostedResourceMapping) { m.Capabilities = []string{"messaging"} },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			change(&candidate)
			_, err := normalizeHostedResource(candidate)
			require.ErrorIs(t, err, errInvalidHostedResource)
		})
	}
}

func TestPostgresHostedVoiceRoomBindingIsExplicitImmutableAndNodeScoped(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	require.NoError(t, migrate(ctx, pool), "additive migration is replayable")
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "room", Issuer: "master", Environment: "sandbox"}
	node, spaceA, spaceB := uuid.NewString(), uuid.NewString(), uuid.NewString()
	require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://node.test", digest([]byte("room-node"))))
	_, err = store.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	for _, space := range []string{spaceA, spaceB} {
		require.NoError(t, store.place(ctx, node, space))
	}
	mapping := HostedResourceMapping{ResourceID: uuid.NewString(), ResourceType: "voice_room", SpaceID: spaceA, HomeNodeID: node, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"voice"}, RoomName: "explicit-" + uuid.NewString()}
	created, err := store.registerHostedResource(ctx, mapping)
	require.NoError(t, err)
	require.Equal(t, mapping.RoomName, created.RoomName)
	restarted := *store
	replayed, err := restarted.registerHostedResource(ctx, mapping)
	require.NoError(t, err)
	require.Equal(t, created, replayed)
	foreign := mapping
	foreign.ResourceID, foreign.SpaceID = uuid.NewString(), spaceB
	_, err = store.registerHostedResource(ctx, foreign)
	require.ErrorIs(t, err, errConflict, "a room on one SFU cannot represent another Space/resource")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_hosted_resources WHERE resource_id=$1`, foreign.ResourceID).Scan(&count))
	require.Zero(t, count, "collision cannot append a route")
	changed := mapping
	changed.RoutingGeneration++
	changed.LifecycleState = "frozen"
	updated, err := restarted.registerHostedResource(ctx, changed)
	require.NoError(t, err)
	require.Equal(t, mapping.RoomName, updated.RoomName)
	changed.RoutingGeneration++
	changed.RoomName += "-substituted"
	_, err = store.registerHostedResource(ctx, changed)
	require.ErrorIs(t, err, errConflict, "a higher route generation cannot substitute an RTC room")
	_, err = pool.Exec(ctx, `UPDATE federation_voice_room_bindings SET room_name='substituted' WHERE resource_id=$1`, mapping.ResourceID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM federation_voice_room_bindings WHERE resource_id=$1`, mapping.ResourceID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO federation_hosted_resources(resource_id,routing_generation,resource_type,space_id,home_node_id,lifecycle_state,capabilities) VALUES($1,1,'chat',$2,$3,'active',ARRAY['messaging'])`, uuid.NewString(), uuid.NewString(), node)
	require.Error(t, err, "database also enforces exact placement")

	// Legacy room-less evidence remains immutable. Only a new owner-supplied
	// generation can attach its canonical room; no name is inferred from UUIDs.
	legacy := mapping
	legacy.ResourceID, legacy.RoomName = uuid.NewString(), "legacy-explicit-"+uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO federation_hosted_resources(resource_id,routing_generation,resource_type,space_id,home_node_id,lifecycle_state,capabilities) VALUES($1,1,'voice_room',$2,$3,'active',ARRAY['voice'])`, legacy.ResourceID, spaceA, node)
	require.NoError(t, err)
	legacy.RoutingGeneration = 2
	_, err = restarted.registerHostedResource(ctx, legacy)
	require.NoError(t, err)
	var originalRoom string
	require.NoError(t, pool.QueryRow(ctx, `SELECT room_name FROM federation_hosted_resources WHERE resource_id=$1 AND routing_generation=1`, legacy.ResourceID).Scan(&originalRoom))
	require.Empty(t, originalRoom, "historical route evidence stays intact")

	operator := &x509.Certificate{Raw: []byte("room-operator"), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	api := &authorityAPI{Store: store, Operators: map[string]bool{digest(operator.Raw): true}}
	call := func(input HostedResourceMapping) *httptest.ResponseRecorder {
		body, err := json.Marshal(input)
		require.NoError(t, err)
		// Registration accepts only its own wire fields, never caller supplied
		// canonical IDs/home node/creation timestamp in the request body.
		var fields map[string]any
		require.NoError(t, json.Unmarshal(body, &fields))
		for _, name := range []string{"resource_id", "space_id", "home_node_id", "created_at"} {
			delete(fields, name)
		}
		body, err = json.Marshal(fields)
		require.NoError(t, err)
		request := httptest.NewRequest("POST", "https://master/v1/nodes/"+node+"/spaces/"+spaceA+"/resources/"+input.ResourceID, strings.NewReader(string(body)))
		request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{operator}, VerifiedChains: [][]*x509.Certificate{{operator}}}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		return response
	}
	require.Equal(t, 200, call(updated).Code)
	missingRoom := mapping
	missingRoom.ResourceID, missingRoom.RoomName = uuid.NewString(), ""
	require.Equal(t, 400, call(missingRoom).Code, "invalid explicit room returns a caller error")
}
