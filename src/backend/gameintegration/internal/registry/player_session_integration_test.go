package registry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPlayerSessionSnapshotBindsAccountProfileCurrentRosterAndLease(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	accountID, profileID, foreignProfile := uuid.New(), uuid.New(), uuid.New()
	sessionID, chatID := uuid.New(), uuid.New()
	_, err := store.Pool.Exec(ctx, `INSERT INTO gis_sessions
(id,application_id,environment_id,kind,external_key,session_status,stage,roster_revision,roster_complete,members,chat_id,lease_expires_at)
VALUES($1,$2,$3,'match','player-session','active','active',9,true,$4,$5,clock_timestamp()+interval '60 seconds')`,
		sessionID, principal.ApplicationID, principal.EnvironmentID, []uuid.UUID{profileID}, chatID)
	require.NoError(t, err)
	insertPlayerSessionBinding(t, ctx, store, principal.ApplicationID, principal.EnvironmentID, accountID, profileID, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	insertPlayerSessionBinding(t, ctx, store, principal.ApplicationID, principal.EnvironmentID, accountID, foreignProfile, "1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	snapshot, err := store.GetPlayerSessionSnapshot(ctx, accountID, profileID, sessionID)
	require.NoError(t, err)
	require.Equal(t, sessionID, snapshot.SessionID)
	require.Equal(t, "match", snapshot.Kind)
	require.Equal(t, int64(9), snapshot.RosterRevision)
	require.Equal(t, &chatID, snapshot.ChatID)

	for _, identity := range []struct{ account, profile uuid.UUID }{
		{uuid.New(), profileID},
		{accountID, foreignProfile},
	} {
		_, err = store.GetPlayerSessionSnapshot(ctx, identity.account, identity.profile, sessionID)
		require.ErrorIs(t, err, ErrPlayerSessionDenied, "foreign and unknown session membership are indistinguishable")
	}
	_, err = store.GetPlayerSessionSnapshot(ctx, accountID, profileID, uuid.New())
	require.ErrorIs(t, err, ErrPlayerSessionDenied)

	rosterOnlyProfile := uuid.New()
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET roster_revision=10,members=$2 WHERE id=$1`, sessionID, []uuid.UUID{rosterOnlyProfile})
	require.NoError(t, err)
	_, err = store.GetPlayerSessionSnapshot(ctx, accountID, profileID, sessionID)
	require.ErrorIs(t, err, ErrPlayerSessionDenied, "the replaced roster revokes the old participant's snapshot access")
	_, err = store.GetPlayerSessionSnapshot(ctx, accountID, rosterOnlyProfile, sessionID)
	require.ErrorIs(t, err, ErrPlayerSessionDenied, "roster membership without a matching Auth account binding is not authority")
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET members=$2 WHERE id=$1`, sessionID, []uuid.UUID{profileID})
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE player_bindings SET status='revoked' WHERE account_id=$1 AND profile_id=$2`, accountID, profileID)
	require.NoError(t, err)
	_, err = store.GetPlayerSessionSnapshot(ctx, accountID, profileID, sessionID)
	require.ErrorIs(t, err, ErrPlayerSessionDenied, "a revoked game binding cannot read the session")
	_, err = store.Pool.Exec(ctx, `UPDATE player_bindings SET status='active' WHERE account_id=$1 AND profile_id=$2`, accountID, profileID)
	require.NoError(t, err)

	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET lease_expires_at=clock_timestamp() WHERE id=$1`, sessionID)
	require.NoError(t, err)
	_, err = store.GetPlayerSessionSnapshot(ctx, accountID, profileID, sessionID)
	require.ErrorIs(t, err, ErrPlayerSessionDenied, "the lease is denied at equality")
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET lease_expires_at=$2 WHERE id=$1`, sessionID, time.Now().Add(-time.Second))
	require.NoError(t, err)
	_, err = store.GetPlayerSessionSnapshot(ctx, accountID, profileID, sessionID)
	require.ErrorIs(t, err, ErrPlayerSessionDenied, "the session lease fails closed at expiry")
}

func insertPlayerSessionBinding(t *testing.T, ctx context.Context, store *Store, app, env, account, profile uuid.UUID, digest string) {
	t.Helper()
	_, err := store.Pool.Exec(ctx, `INSERT INTO player_bindings
(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
VALUES($1,$2,$3,'provider','hmac-sha256-v1:test:'||$4,$5,$5,$6,$7,'active',1)`,
		uuid.New(), app, env, digest, account, profile, uuid.New())
	require.NoError(t, err)
}
