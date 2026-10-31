//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

// This fixture initializes only the seeded legacy spaces used by the
// concurrency, delayed-capture and roster proofs, after the real native50 transition.
func TestAccountBanFixtureConcurrencyEnrollment50(t *testing.T) {
	accountBanFixtureEnrollment50(t, []int{8, 9, 11, 12})
}

// The clock fixture uses a separate database and enrolls only its roster space.
func TestAccountBanFixtureRosterEnrollment50(t *testing.T) {
	accountBanFixtureEnrollment50(t, []int{8})
}

func accountBanFixtureEnrollment50(t *testing.T, spaces []int) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned native50 fixture required")
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	st := &SpaceStore{Pool: owner}
	before, err := st.InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	for _, n := range spaces {
		space := nativeBanSpace(n)
		var count int
		require.NoError(t, owner.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=ANY($2::uuid[])`, space, []uuid.UUID{nativeBanProfile(1), nativeBanProfile(2), nativeBanProfile(3)}).Scan(&count))
		require.Equal(t, 3, count)
		require.NoError(t, st.InitializeNativeMembership(ctx, space))
		var complete bool
		require.NoError(t, owner.QueryRow(ctx, `SELECT state='COMPLETE' FROM native_membership_guard.spaces WHERE space_id=$1`, space).Scan(&complete))
		require.True(t, complete)
	}
	after, err := st.InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	require.Equal(t, before, after, "fixture enrollment must preserve the source seal")
}

func TestAccountBanIntentConcurrency50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	serving := projectionServingLogin(t, ctx, owner)
	space, operation := nativeBanSpace(9), uuid.New()
	st := &SpaceStore{Pool: serving}
	st.AccountCaptures = nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		err := nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error {
			t.Error("active same-intent probe must precede new Role check")
			return nil
		})
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		details := status.Convert(err).Details()
		require.Len(t, details, 1)
		info, ok := details[0].(*errdetails.ErrorInfo)
		require.True(t, ok)
		require.Equal(t, "voice.space.moderation.v1", info.Domain)
		require.Equal(t, "INTENT_IN_PROGRESS", info.Reason)
		hint := nativeBanProfile(6)
		err = nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, &hint, func(context.Context) error {
			t.Error("changed binding must conflict before new Role check")
			return nil
		})
		require.Equal(t, codes.AlreadyExists, status.Code(err))
		return nativeBanOwnerFixture(r), nil
	})
	require.NoError(t, nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error { return nil }))
	err = nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(6), operation, nil, func(context.Context) error {
		return status.Error(codes.PermissionDenied, "current actor has no moderation permission")
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "another verified actor has an independent root, no saved-outcome oracle")
	var count int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE client_operation_id=$1`, operation).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&count))
	require.Equal(t, 1, count)
}

func TestAccountBanLockWaitExhaustsOriginalBudget50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	serving := projectionServingLogin(t, ctx, owner)
	space, operation := nativeBanSpace(11), uuid.New()
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		return nativeBanOwnerFixture(r), nil
	})}
	blocker, err := serving.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(space))
	require.NoError(t, err)
	r := &spacev1.BanMemberRequest{SpaceId: space.String(), AccountId: nativeBanAccount(2).String()}
	nativeBanIntentField(t, r, operation)
	verified, call := nativeBanCall(t, r, nativeBanProfile(1))
	result := make(chan error, 1)
	go func() {
		result <- st.BanAccount(verified, AccountBanInput{SpaceID: space, AccountID: nativeBanAccount(2), ActorProfileID: nativeBanProfile(1), InitiatingCall: call, Request: r, Authorize: func(context.Context) error {
			t.Error("expired actor must not reach a fresh Role observation")
			return nil
		}})
	}()
	// The actual DB fence is held across the original opaque ingress budget.
	// No fake request timestamp, renewed proof or transaction-start DB time.
	select {
	case err = <-result:
		t.Fatalf("call returned before held fence was released: %v", err)
	case <-time.After(5100 * time.Millisecond):
	}
	require.NoError(t, blocker.Commit(ctx))
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("expired call failed to leave the released DB fence")
	}
	nativeBanAssertNoEffect(t, serving, space)
	var roots int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE space_id=$1`, space).Scan(&roots))
	require.Zero(t, roots)
	require.NoError(t, nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error { return nil }), "later authenticated retry gets a new bounded phase")
}
