package grpcsvc

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/profilemutation"
	"voice/backend/user/internal/store"
)

type profileMutationEvents struct {
	UserEventsPublisher
	created atomic.Int32
}

func (e *profileMutationEvents) PublishProfileCreated(context.Context, string, string) error {
	e.created.Add(1)
	return nil
}

func TestProfileMutationLimitIntegrationSharedBudgetAndNoDeniedWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	account, primary := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary)
 VALUES($1,$2,'rate_limit_primary','0001','Main',true)`, primary, account)
	require.NoError(t, err)
	server := miniredis.RunT(t)
	start := time.Unix(1700000000, 0).UTC()
	server.SetTime(start)
	client := func() *redis.Client {
		c := redis.NewClient(&redis.Options{Addr: server.Addr()})
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	events := &profileMutationEvents{}
	service := func() *UserGRPC {
		return &UserGRPC{
			Profiles: store.NewProfileStore(pool), Privacy: store.NewPrivacyStore(pool),
			ProfileMutations: profilemutation.New(client()), Events: events,
		}
	}
	first, second := service(), service()
	caller := metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderUserID, account.String(), authctx.HeaderProfileID, primary.String()))
	lastID := ""
	for i := 0; i < profilemutation.MaxAttempts; i++ {
		svc := first
		if i%2 != 0 {
			svc = second
		}
		response, err := svc.CreateProfile(caller, &userv1.CreateProfileRequest{DisplayName: fmt.Sprintf("Alt %d", i)})
		require.NoError(t, err)
		lastID = response.GetProfile().GetId()
		_, err = svc.DeleteProfile(caller, &userv1.DeleteProfileRequest{ProfileId: lastID})
		require.NoError(t, err)
	}
	snapshot := func() [2]int64 {
		var rows, active int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE deleted_at IS NULL) FROM profiles WHERE account_id=$1`, account).Scan(&rows, &active))
		return [2]int64{rows, active}
	}
	before := snapshot()
	require.Equal(t, [2]int64{11, 1}, before)
	_, err = second.CreateProfile(caller, &userv1.CreateProfileRequest{DisplayName: "Denied"})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
	_, err = first.DeleteProfile(caller, &userv1.DeleteProfileRequest{ProfileId: lastID})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
	require.Equal(t, before, snapshot())
	require.EqualValues(t, 10, events.created.Load(), "denied create must not publish")
	server.Close()
	_, err = first.CreateProfile(caller, &userv1.CreateProfileRequest{DisplayName: "Redis unavailable"})
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = second.DeleteProfile(caller, &userv1.DeleteProfileRequest{ProfileId: primary.String()})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, before, snapshot())
	require.EqualValues(t, 10, events.created.Load())
}
