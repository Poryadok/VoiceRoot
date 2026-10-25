package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/store"
)

func TestProfileDeletionStatesUsesOnlyDeletedAtInPostgres(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "deletionstates", "")
	applyUserPrivacyMigrations(t, ctx, pool)
	live, deleted, frozen, missing := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for i, id := range []uuid.UUID{live, deleted, frozen} {
		seedScheduledPresenceProfile(t, ctx, pool, uuid.New(), id, false, []string{"state-live", "state-deleted", "state-frozen"}[i])
	}
	_, err := pool.Exec(ctx, `UPDATE profiles SET deleted_at=now() WHERE id=$1`, deleted)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE profiles SET frozen_at=now() WHERE id=$1`, frozen)
	require.NoError(t, err)
	// No Auth/privacy/presence dependencies: their absence must not alter deletion facts.
	svc := &MessagingScheduledPresenceGRPC{User: &UserGRPC{Profiles: store.NewProfileStore(pool)}}
	req := &userv1.GetProfileDeletionStatesRequest{ProfileIds: []string{missing.String(), frozen.String(), deleted.String(), live.String()}}
	resp, err := svc.GetProfileDeletionStates(deletionStatesContext(t, req), req)
	require.NoError(t, err)
	require.Len(t, resp.States, 4)
	want := []userv1.ProfileDeletionState{userv1.ProfileDeletionState_PROFILE_DELETION_STATE_UNKNOWN, userv1.ProfileDeletionState_PROFILE_DELETION_STATE_NOT_DELETED, userv1.ProfileDeletionState_PROFILE_DELETION_STATE_SOFT_DELETED, userv1.ProfileDeletionState_PROFILE_DELETION_STATE_NOT_DELETED}
	for i, entry := range resp.States {
		require.Equal(t, req.ProfileIds[i], entry.ProfileId)
		require.Equal(t, want[i], entry.State)
	}
	_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at=NULL WHERE id=$1`, deleted)
	require.NoError(t, err)
	resp, err = svc.GetProfileDeletionStates(deletionStatesContext(t, req), req)
	require.NoError(t, err)
	require.Equal(t, userv1.ProfileDeletionState_PROFILE_DELETION_STATE_NOT_DELETED, resp.States[2].State, "restoration is visible immediately")
	maximum := &userv1.GetProfileDeletionStatesRequest{ProfileIds: make([]string, 500)}
	for i := range maximum.ProfileIds {
		maximum.ProfileIds[i] = uuid.NewString()
	}
	resp, err = svc.GetProfileDeletionStates(deletionStatesContext(t, maximum), maximum)
	require.NoError(t, err)
	require.Len(t, resp.States, 500, "the documented maximum batch is accepted completely")
	for i, entry := range resp.States {
		require.Equal(t, maximum.ProfileIds[i], entry.ProfileId)
		require.Equal(t, userv1.ProfileDeletionState_PROFILE_DELETION_STATE_UNKNOWN, entry.State)
	}
	_, err = pool.Exec(ctx, `DROP TABLE profiles CASCADE`)
	require.NoError(t, err)
	resp, err = svc.GetProfileDeletionStates(deletionStatesContext(t, req), req)
	require.Nil(t, resp)
	require.Equal(t, codes.Unavailable, status.Code(err), "storage failure never fabricates deletion/unknown states")
}
