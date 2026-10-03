package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresRoomBindingUpgradePreservesLegacyEvidenceAndFencesForeignRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	_, err := pool.Exec(ctx, `CREATE TABLE federation_schema_versions(version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	require.NoError(t, err)
	for index, sql := range []string{migrationSQL, auditMigrationSQL, hostedResourcesMigrationSQL, snapshotRevisionsMigrationSQL} {
		_, err = pool.Exec(ctx, sql)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO federation_schema_versions(version) VALUES($1)`, index+1)
		require.NoError(t, err)
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &authorityStore{Pool: pool, Key: key, KeyID: "upgrade", Issuer: "master", Environment: "sandbox"}
	nodeA, nodeB, space := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, node := range []string{nodeA, nodeB} {
		require.NoError(t, store.enroll(ctx, node, uuid.NewString(), "https://node.test", digest([]byte(node))))
		_, err = store.changeNode(ctx, node, "approve", "", "operator")
		require.NoError(t, err)
	}
	require.NoError(t, store.place(ctx, nodeA, space))
	legacyRoom, legacyForeign := uuid.NewString(), uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO federation_hosted_resources(resource_id,routing_generation,resource_type,space_id,home_node_id,lifecycle_state,capabilities) VALUES($1,1,'voice_room',$2,$3,'active',ARRAY['voice']),($4,1,'chat',$2,$5,'active',ARRAY['messaging'])`, legacyRoom, space, nodeA, legacyForeign, nodeB)
	require.NoError(t, err, "reproduce pre-fix foreign route admitted by schema v4")
	var before string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(r) ORDER BY resource_id)::text FROM federation_hosted_resources r`).Scan(&before))
	require.NoError(t, migrate(ctx, pool))
	require.NoError(t, migrate(ctx, pool))
	var after string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(r)-'room_name' ORDER BY resource_id)::text FROM federation_hosted_resources r`).Scan(&after))
	require.JSONEq(t, before, after, "migration cannot rewrite immutable historical evidence")
	_, err = store.registerHostedResource(ctx, HostedResourceMapping{ResourceID: legacyForeign, ResourceType: "chat", SpaceID: space, HomeNodeID: nodeB, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"messaging"}})
	require.ErrorIs(t, err, errForbidden, "legacy foreign route cannot gain authority through exact retry")
	_, err = pool.Exec(ctx, `INSERT INTO federation_hosted_resources(resource_id,routing_generation,resource_type,space_id,home_node_id,lifecycle_state,capabilities) VALUES($1,2,'chat',$2,$3,'active',ARRAY['messaging'])`, legacyForeign, space, nodeB)
	require.Error(t, err, "new generations must meet exact placement even for a historical bad route")
	_, err = store.registerHostedResource(ctx, HostedResourceMapping{ResourceID: legacyRoom, ResourceType: "voice_room", SpaceID: space, HomeNodeID: nodeA, RoutingGeneration: 1, LifecycleState: "active", Capabilities: []string{"voice"}})
	require.ErrorIs(t, err, errInvalidHostedResource)
	room := "owner-canonical-" + uuid.NewString()
	_, err = store.registerHostedResource(ctx, HostedResourceMapping{ResourceID: legacyRoom, ResourceType: "voice_room", SpaceID: space, HomeNodeID: nodeA, RoutingGeneration: 2, LifecycleState: "active", Capabilities: []string{"voice"}, RoomName: room})
	require.NoError(t, err)
	var original, bound string
	require.NoError(t, pool.QueryRow(ctx, `SELECT room_name FROM federation_hosted_resources WHERE resource_id=$1 AND routing_generation=1`, legacyRoom).Scan(&original))
	require.Empty(t, original)
	require.NoError(t, pool.QueryRow(ctx, `SELECT room_name FROM federation_voice_room_bindings WHERE resource_id=$1`, legacyRoom).Scan(&bound))
	require.Equal(t, room, bound)
}
