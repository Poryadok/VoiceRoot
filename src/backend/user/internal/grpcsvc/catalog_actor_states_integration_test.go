package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"voice/backend/pkg/integrationtest"
	catalogv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/user/internal/store"
)

func TestCatalogActorStatesExactOwnershipDeletedAtPostgres(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "catalogactors", "")
	applyUserPrivacyMigrations(t, ctx, pool)
	accounts := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	live, deleted, frozen, wrongOwner, missing := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for i, id := range []uuid.UUID{live, deleted, frozen, wrongOwner} {
		seedScheduledPresenceProfile(t, ctx, pool, accounts[i], id, false, []string{"catalog-live", "catalog-deleted", "catalog-frozen", "catalog-wrong"}[i])
	}
	_, err := pool.Exec(ctx, `UPDATE profiles SET deleted_at=now() WHERE id=$1`, deleted)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE profiles SET frozen_at=now() WHERE id=$1`, frozen)
	require.NoError(t, err)
	req := &catalogv1.GetCatalogActorStatesRequest{ProtocolVersion: 1}
	for i, id := range []uuid.UUID{live, deleted, frozen, wrongOwner, missing} {
		req.Actors = append(req.Actors, &catalogv1.CatalogActor{AccountId: accounts[i].String(), ProfileId: id.String()})
	}
	req.Actors[3].AccountId = uuid.NewString()
	svc := &CatalogActorGRPC{Profiles: store.NewProfileStore(pool)}
	var before string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM profiles p`).Scan(&before))
	for _, issuer := range []string{"chat", "file"} {
		response, err := svc.GetCatalogActorStates(catalogContext(t, issuer, req), req)
		require.NoError(t, err)
		require.EqualValues(t, 1, response.ProtocolVersion)
		require.Len(t, response.States, 5)
		want := []catalogv1.CatalogActorState{catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED, catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_SOFT_DELETED, catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED, catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_UNKNOWN, catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_UNKNOWN}
		for i, entry := range response.States {
			require.Equal(t, req.Actors[i].AccountId, entry.Actor.AccountId)
			require.Equal(t, req.Actors[i].ProfileId, entry.Actor.ProfileId)
			require.Equal(t, want[i], entry.State)
		}
	}
	var after string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM profiles p`).Scan(&after))
	require.Equal(t, before, after, "snapshot must never mutate rows")
	maximum := &catalogv1.GetCatalogActorStatesRequest{ProtocolVersion: 1}
	for i := 0; i < 100; i++ {
		maximum.Actors = append(maximum.Actors, catalogRequest().Actors[0])
	}
	response, err := svc.GetCatalogActorStates(catalogContext(t, "chat", maximum), maximum)
	require.NoError(t, err)
	require.Len(t, response.States, 100)
	_, err = pool.Exec(ctx, `DROP TABLE profiles CASCADE`)
	require.NoError(t, err)
	response, err = svc.GetCatalogActorStates(catalogContext(t, "chat", req), req)
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
