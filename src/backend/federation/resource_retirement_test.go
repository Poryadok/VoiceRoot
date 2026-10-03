package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/protocol"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresHistoricalPurgeFenceDeniesPreviouslyRevivedRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "history", Issuer: "master", Environment: "sandbox"}
	node, space, pin := uuid.NewString(), uuid.NewString(), digest([]byte("history-node"))
	require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://node.test", pin))
	credential, err := store.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	require.NoError(t, store.place(ctx, node, space))
	for index, state := range []string{"purging", "tombstoned"} {
		t.Run(state, func(t *testing.T) {
			resource := HostedResourceMapping{ResourceID: uuid.NewString(), ResourceType: "voice_room", SpaceID: space, HomeNodeID: node, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"voice"}, RoomName: "history-" + state}
			_, err := store.registerHostedResource(ctx, resource)
			require.NoError(t, err)
			resource.RoutingGeneration = 2
			resource.LifecycleState = state
			_, err = store.registerHostedResource(ctx, resource)
			require.NoError(t, err)
			// Simulate an immutable pre-repair history and saved complete projection.
			// INSERT is allowed; no original route or retirement evidence is rewritten.
			resource.RoutingGeneration = 3
			resource.LifecycleState = "active"
			_, err = pool.Exec(ctx, `INSERT INTO federation_hosted_resources(resource_id,routing_generation,resource_type,space_id,home_node_id,lifecycle_state,capabilities,room_name) VALUES($1,3,'voice_room',$2,$3,'active',$4,$5)`, resource.ResourceID, space, node, resource.Capabilities, resource.RoomName)
			require.NoError(t, err)
			request := mediaauthority.Request{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SpaceID: space, ResourceID: resource.ResourceID, RoomName: resource.RoomName, SessionEpoch: 1, RoutingGeneration: 3}
			policy := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: int64(index*2 + 1), ValidUntil: time.Now().Add(1900 * time.Millisecond).UnixMilli(), Permissions: []Permission{{AccountID: request.AccountID, ProfileID: request.ProfileID, ResourceID: request.ResourceID, SessionEpoch: 1, RoutingGeneration: 3, RoomName: resource.RoomName, Actions: []string{"media"}}}}
			raw, err := json.Marshal(policy)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `UPDATE federation_placements SET revision=$3,snapshot=$4,snapshot_hash=$5,valid_until=$6 WHERE space_id=$1 AND node_id=$2`, space, node, policy.Revision, raw, digest(raw), time.UnixMilli(policy.ValidUntil))
			require.NoError(t, err)
			_, err = store.issueMediaGrant(ctx, request)
			require.ErrorIs(t, err, errForbidden)
			_, err = store.resolveMediaRoute(ctx, mediaauthority.RouteRequest{AccountID: request.AccountID, ProfileID: request.ProfileID, SpaceID: space, ResourceID: request.ResourceID, RoomName: request.RoomName, SessionEpoch: 1})
			require.ErrorIs(t, err, errForbidden)
			_, err = store.issue(ctx, node, space, pin, credential.Secret, &leaseRequest{Revision: policy.Revision, Hash: digest(raw), Nonce: uuid.NewString()})
			require.ErrorIs(t, err, errForbidden)
			_, err = store.issue(ctx, node, space, pin, credential.Secret, nil)
			require.ErrorIs(t, err, errForbidden)
			_, err = store.issuePage(ctx, node, space, pin, credential.Secret, 0)
			require.ErrorIs(t, err, errForbidden)
			_, err = store.issueRevisionStream(ctx, node, space, pin, credential.Secret, policy.Revision)
			require.ErrorIs(t, err, errForbidden)
			_, err = store.registerHostedResource(ctx, resource)
			require.ErrorIs(t, err, errConflict, "exact pre-repair active retry cannot bypass an older permanent fence")
			resource.RoutingGeneration++
			_, err = store.registerHostedResource(ctx, resource)
			require.ErrorIs(t, err, errConflict)
			policy.Revision++
			policy.ValidUntil = time.Now().Add(1900 * time.Millisecond).UnixMilli()
			require.ErrorIs(t, store.publish(ctx, node, space, policy), errForbidden)
			// A legacy generation-zero content permission is still subject to the
			// permanent resource fence; omission cannot restore retired authority.
			policy.Permissions[0].RoutingGeneration = 0
			policy.Permissions[0].RoomName = ""
			policy.Permissions[0].Actions = []string{"read"}
			require.ErrorIs(t, store.publish(ctx, node, space, policy), errForbidden)
			legacy, err := json.Marshal(policy)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `UPDATE federation_placements SET revision=$3,snapshot=$4,snapshot_hash=$5,valid_until=$6 WHERE space_id=$1 AND node_id=$2`, space, node, policy.Revision, legacy, digest(legacy), time.UnixMilli(policy.ValidUntil))
			require.NoError(t, err)
			_, err = store.issue(ctx, node, space, pin, credential.Secret, &leaseRequest{Revision: policy.Revision, Hash: digest(legacy), Nonce: uuid.NewString()})
			require.ErrorIs(t, err, errForbidden)
			_, err = store.issue(ctx, node, space, pin, credential.Secret, nil)
			require.ErrorIs(t, err, errForbidden)
			_, err = store.issuePage(ctx, node, space, pin, credential.Secret, 0)
			require.ErrorIs(t, err, errForbidden)
			_, err = store.issueRevisionStream(ctx, node, space, pin, credential.Secret, policy.Revision)
			require.ErrorIs(t, err, errForbidden)
			policy.Revision++
			policy.Permissions = []Permission{}
			require.NoError(t, store.publish(ctx, node, space, policy))
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_hosted_resources WHERE resource_id=$1`, resource.ResourceID).Scan(&count))
			require.Equal(t, 3, count)
		})
	}
}

func TestPostgresRetiredResourceCannotReopenOrRenewItsStaleProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "retirement", Issuer: "master", Environment: "sandbox"}
	node, space, pin := uuid.NewString(), uuid.NewString(), digest([]byte("retirement-node"))
	require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://node.test", pin))
	credential, err := store.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	require.NoError(t, store.place(ctx, node, space))
	resource := HostedResourceMapping{ResourceID: uuid.NewString(), ResourceType: "voice_room", SpaceID: space, HomeNodeID: node, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"voice"}, RoomName: "permanent-room"}
	_, err = store.registerHostedResource(ctx, resource)
	require.NoError(t, err)
	request := mediaauthority.Request{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SpaceID: space, ResourceID: resource.ResourceID, RoomName: resource.RoomName, SessionEpoch: 1, RoutingGeneration: 1}
	policy := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: time.Now().Add(1900 * time.Millisecond).UnixMilli(), Permissions: []Permission{{AccountID: request.AccountID, ProfileID: request.ProfileID, ResourceID: request.ResourceID, SessionEpoch: 1, RoutingGeneration: 1, RoomName: resource.RoomName, Actions: []string{"media"}}}}
	require.NoError(t, store.publish(ctx, node, space, policy))
	lease := func(revision int64, hash string) error {
		_, e := store.issue(ctx, node, space, pin, credential.Secret, &leaseRequest{Revision: revision, Hash: hash, Nonce: uuid.NewString()})
		return e
	}
	require.NoError(t, lease(1, protocol.SnapshotDigest(policy)))
	resource.RoutingGeneration = 2
	resource.LifecycleState = "purging"
	_, err = store.registerHostedResource(ctx, resource)
	require.NoError(t, err)
	_, err = store.issueMediaGrant(ctx, request)
	require.ErrorIs(t, err, errForbidden)
	require.ErrorIs(t, lease(1, protocol.SnapshotDigest(policy)), errForbidden, "a changed registry cannot renew a stale complete projection")
	require.NoError(t, store.publish(ctx, node, space, policy), "exact saved publication remains an inert historical retry")
	require.ErrorIs(t, lease(1, protocol.SnapshotDigest(policy)), errForbidden, "historical replay cannot undo registry invalidation")
	for _, state := range []string{"active", "frozen"} {
		revived := resource
		revived.RoutingGeneration++
		revived.LifecycleState = state
		_, err = store.registerHostedResource(ctx, revived)
		require.ErrorIs(t, err, errConflict, "purging is irreversible")
	}
	resource.RoutingGeneration = 3
	resource.LifecycleState = "tombstoned"
	tombstone, err := store.registerHostedResource(ctx, resource)
	require.NoError(t, err)
	retry, err := store.registerHostedResource(ctx, resource)
	require.NoError(t, err)
	require.Equal(t, tombstone.CreatedAt, retry.CreatedAt)
	for _, state := range []string{"active", "frozen", "purging", "tombstoned"} {
		revived := resource
		revived.RoutingGeneration++
		revived.LifecycleState = state
		_, err = store.registerHostedResource(ctx, revived)
		require.ErrorIs(t, err, errConflict, "terminal evidence cannot append a replacement generation")
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_hosted_resources WHERE resource_id=$1`, resource.ResourceID).Scan(&count))
	require.Equal(t, 3, count)
	policy.Revision = 2
	policy.ValidUntil = time.Now().Add(1900 * time.Millisecond).UnixMilli()
	require.ErrorIs(t, store.publish(ctx, node, space, policy), errForbidden, "a fresh revision cannot republish a retired route grant")
	policy.Permissions = []Permission{}
	require.NoError(t, store.publish(ctx, node, space, policy))
	require.NoError(t, lease(2, protocol.SnapshotDigest(policy)), "complete reconciled deny policy may renew authority")

	// Freeze is reversible before purge. Every transition still requires a new
	// explicit route generation and a new complete reconciled source revision.
	other := resource
	other.ResourceID = uuid.NewString()
	other.RoomName += "-restorable"
	other.RoutingGeneration = 1
	other.LifecycleState = "active"
	_, err = store.registerHostedResource(ctx, other)
	require.NoError(t, err)
	other.RoutingGeneration = 2
	other.LifecycleState = "frozen"
	_, err = store.registerHostedResource(ctx, other)
	require.NoError(t, err)
	other.RoutingGeneration = 3
	other.LifecycleState = "active"
	_, err = store.registerHostedResource(ctx, other)
	require.NoError(t, err)
	policy.Revision = 3
	policy.ValidUntil = time.Now().Add(1900 * time.Millisecond).UnixMilli()
	policy.Permissions = []Permission{{AccountID: request.AccountID, ProfileID: request.ProfileID, ResourceID: other.ResourceID, SessionEpoch: 1, RoutingGeneration: 3, RoomName: other.RoomName, Actions: []string{"media"}}}
	require.NoError(t, store.publish(ctx, node, space, policy))
	require.NoError(t, lease(3, protocol.SnapshotDigest(policy)))
}

func TestPostgresNodeLeaseCannotSignAnElapsedSourceAfterPlacementLockWait(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "clock", Issuer: "master", Environment: "sandbox"}
	node, space, pin := uuid.NewString(), uuid.NewString(), digest([]byte("clock-node"))
	require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://node.test", pin))
	credential, err := store.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	require.NoError(t, store.place(ctx, node, space))
	for index, kind := range []string{"lease", "manifest", "page", "revisions"} {
		t.Run(kind, func(t *testing.T) {
			until := time.Now().Add(1900 * time.Millisecond)
			policy := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: int64(index + 1), ValidUntil: until.UnixMilli(), Permissions: []Permission{}}
			require.NoError(t, store.publish(ctx, node, space, policy))
			blocker, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer blocker.Rollback(ctx)
			var locked string
			require.NoError(t, blocker.QueryRow(ctx, `SELECT space_id::text FROM federation_placements WHERE space_id=$1 FOR UPDATE`, space).Scan(&locked))
			result := make(chan error, 1)
			go func() {
				var e error
				switch kind {
				case "lease":
					_, e = store.issue(ctx, node, space, pin, credential.Secret, &leaseRequest{Revision: policy.Revision, Hash: protocol.SnapshotDigest(policy), Nonce: uuid.NewString()})
				case "manifest":
					_, e = store.issue(ctx, node, space, pin, credential.Secret, nil)
				case "page":
					_, e = store.issuePage(ctx, node, space, pin, credential.Secret, 0)
				case "revisions":
					_, e = store.issueRevisionStream(ctx, node, space, pin, credential.Secret, 0)
				}
				result <- e
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND query LIKE '%FROM federation_placements%FOR UPDATE%')`).Scan(&waiting)
				return e == nil && waiting
			}, time.Second, 10*time.Millisecond)
			time.Sleep(time.Until(until.Add(50 * time.Millisecond)))
			require.NoError(t, blocker.Commit(ctx))
			require.ErrorIs(t, <-result, errForbidden, "DB source expired during placement wait; no signed envelope may be returned")
		})
	}
}
