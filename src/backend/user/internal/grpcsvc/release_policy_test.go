package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/releasepolicy"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"
)

func TestReleasePolicyCustomStatusOwnerEffects(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	var pool *pgxpool.Pool
	if dsn := os.Getenv("USER_RELEASE_TEST_DATABASE_URL"); dsn != "" {
		var err error
		pool, err = pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		t.Cleanup(pool.Close)
	} else {
		pool = integrationtest.StartPostgres(t, ctx, "userdb", "")
	}
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))
	account, profile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary,custom_status) VALUES($1,$2,'releaseowner','7732','Before',true,'saved')`, profile, account)
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	events := &customStatusEventsRecorder{}
	svc := &UserGRPC{Profiles: store.NewProfileStore(pool), Presence: store.NewPresenceStore(rdb), Events: events}
	premium := metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderUserID, account.String(), authctx.HeaderProfileID, profile.String(), authctx.HeaderSubscriptionTier, "premium"))
	_, err = svc.UpdatePresence(premium, &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: proto.String("saved")})
	require.NoError(t, err)
	before := mr.Dump()
	beforeEvents := events.presence
	data, err := os.ReadFile("../../../pkg/releasepolicy/testdata/free-invite-alpha.json")
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	svc.ReleasePolicy, err = releasepolicy.Parse(data, hex.EncodeToString(sum[:]), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	_, err = svc.UpdatePresence(premium, &userv1.UpdatePresenceRequest{Status: "dnd", CustomStatus: proto.String("blocked")})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Equal(t, "capability_unavailable", status.Convert(err).Message())
	require.Equal(t, before, mr.Dump())
	require.Equal(t, beforeEvents, events.presence)
	_, err = svc.UpdateProfile(premium, &userv1.UpdateProfileRequest{ProfileId: profile.String(), DisplayName: proto.String("Blocked"), CustomStatus: proto.String("blocked")})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Equal(t, "capability_unavailable", status.Convert(err).Message())
	var name, custom string
	require.NoError(t, pool.QueryRow(ctx, `SELECT display_name,custom_status FROM profiles WHERE id=$1`, profile).Scan(&name, &custom))
	require.Equal(t, "Before", name)
	require.Equal(t, "saved", custom)
	require.Zero(t, events.profile)
	// An alpha policy is not authority for a foreign profile.
	_, err = svc.UpdateProfile(premium, &userv1.UpdateProfileRequest{ProfileId: uuid.NewString(), CustomStatus: proto.String("blocked")})
	require.Equal(t, codes.NotFound, status.Code(err))
	// Actual positive controls write plain presence, explicit clears and free edits.
	_, err = svc.UpdatePresence(premium, &userv1.UpdatePresenceRequest{Status: "idle"})
	require.NoError(t, err)
	snap, err := svc.Presence.Get(ctx, profile)
	require.NoError(t, err)
	require.Equal(t, "saved", snap.CustomStatus)
	require.Equal(t, "idle", snap.Status)
	_, err = svc.UpdatePresence(premium, &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: proto.String("")})
	require.NoError(t, err)
	snap, err = svc.Presence.Get(ctx, profile)
	require.NoError(t, err)
	require.Empty(t, snap.CustomStatus)
	_, err = svc.UpdateProfile(premium, &userv1.UpdateProfileRequest{ProfileId: profile.String(), DisplayName: proto.String("Allowed"), CustomStatus: proto.String("")})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT display_name,custom_status FROM profiles WHERE id=$1`, profile).Scan(&name, &custom))
	require.Equal(t, "Allowed", name)
	require.Empty(t, custom)
	require.Equal(t, 1, events.profile)
}
