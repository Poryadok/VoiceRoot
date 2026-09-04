package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	socialv1 "voice.app/voice/social/v1"
)

type accountProfilesResolverFunc func(context.Context, uuid.UUID) ([]uuid.UUID, error)

func (f accountProfilesResolverFunc) ProfileIDsForAccount(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	return f(ctx, accountID)
}

func seedFriendship(t *testing.T, ctx context.Context, pool *pgxpool.Pool, requester, target uuid.UUID, state string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO friendships (id, requester_profile_id, target_profile_id, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())`, uuid.New(), requester, target, state)
	require.NoError(t, err)
}

func countFriendshipsBetween(t *testing.T, ctx context.Context, pool *pgxpool.Pool, setA, setB []uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(ctx, `
SELECT count(*) FROM friendships
WHERE (
  requester_profile_id = ANY($1::uuid[]) AND target_profile_id = ANY($2::uuid[])
) OR (
  requester_profile_id = ANY($2::uuid[]) AND target_profile_id = ANY($1::uuid[])
)`, setA, setB).Scan(&count)
	require.NoError(t, err)
	return count
}

func requireNoDirectedBlock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blocker, blocked uuid.UUID) {
	t.Helper()
	var count int
	err := pool.QueryRow(ctx, `
SELECT count(*) FROM blocks WHERE blocker_account_id = $1 AND blocked_account_id = $2`, blocker, blocked).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
}

// TestBlockAccount_RemovesAcceptedFriendship documents block cascade: accepted friendship rows are removed.
func TestBlockAccount_RemovesAcceptedFriendship(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {profB},
		}
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, profA), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: profB.String(),
	})
	require.NoError(t, err)
	_, err = client.AcceptFriendInvitation(withProfileCtx(ctx, profB), &socialv1.AcceptFriendInvitationRequest{
		RequesterProfileId: profA.String(),
	})
	require.NoError(t, err)

	friends, err := client.ListFriends(withProfileCtx(ctx, profA), &socialv1.ListFriendsRequest{})
	require.NoError(t, err)
	require.Len(t, friends.GetFriendList().GetFriends(), 1)

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{
		BlockedAccountId: accB.String(),
	})
	require.NoError(t, err)

	friendsAfter, err := client.ListFriends(withProfileCtx(ctx, profA), &socialv1.ListFriendsRequest{})
	require.NoError(t, err)
	require.Empty(t, friendsAfter.GetFriendList().GetFriends())

	af, err := client.AreFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: profA.String(),
		ProfileIdB: profB.String(),
	})
	require.NoError(t, err)
	require.False(t, af.GetFriends())
}

// TestBlockAccount_ClearsPendingOutgoingFriendRequest documents block cascade: pending outgoing invites are cancelled.
func TestBlockAccount_ClearsPendingOutgoingFriendRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {profB},
		}
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, profA), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: profB.String(),
	})
	require.NoError(t, err)

	outBefore, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, outBefore.GetFriendRequestList().GetOutgoing(), 1)

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{
		BlockedAccountId: accB.String(),
	})
	require.NoError(t, err)

	outAfter, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, outAfter.GetFriendRequestList().GetOutgoing())

	inAfter, err := client.ListFriendRequests(withProfileCtx(ctx, profB), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, inAfter.GetFriendRequestList().GetIncoming())
}

// TestBlockAccount_ClearsPendingIncomingFriendRequest documents block cascade: pending incoming invites are declined/cancelled.
func TestBlockAccount_ClearsPendingIncomingFriendRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {profB},
		}
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, profB), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: profA.String(),
	})
	require.NoError(t, err)

	inBefore, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, inBefore.GetFriendRequestList().GetIncoming(), 1)

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{
		BlockedAccountId: accB.String(),
	})
	require.NoError(t, err)

	inAfter, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, inAfter.GetFriendRequestList().GetIncoming())
}

// TestBlockAccount_ResolverUnavailable_LeavesBlockAndFriendshipsUntouched documents
// the fail-closed account-level cascade: an unwired resolver is a configuration
// failure, while a live resolver error is a transient dependency failure. Neither
// may create the account block or sever only part of the friendship graph.
func TestBlockAccount_ResolverUnavailable_LeavesBlockAndFriendshipsUntouched(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()

	for _, tc := range []struct {
		name     string
		resolver AccountProfilesResolver
		wantCode codes.Code
	}{
		{name: "unwired", resolver: nil, wantCode: codes.FailedPrecondition},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := startSocialPostgresForTest(t, ctx)
			applySocialMigration(t, ctx, pool)

			accA, accB := uuid.New(), uuid.New()
			profA, profB := uuid.New(), uuid.New()
			seedFriendship(t, ctx, pool, profA, profB, "accepted")

			client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
				s.AccountProfiles = tc.resolver
			})
			t.Cleanup(cleanup)

			_, err := client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
			require.Error(t, err)
			require.Equal(t, tc.wantCode, status.Code(err))
			requireNoDirectedBlock(t, ctx, pool, accA, accB)
			require.Equal(t, 1, countFriendshipsBetween(t, ctx, pool, []uuid.UUID{profA}, []uuid.UUID{profB}))
		})
	}
}

// TestBlockAccount_SecondResolverResultInvalid_LeavesBlockAndFriendshipsUntouched
// proves resolution is completed before any local mutation: the blocker's profile
// lookup succeeds, then the blocked account's result is unusable.
func TestBlockAccount_SecondResolverResultInvalid_LeavesBlockAndFriendshipsUntouched(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()

	for _, tc := range []struct {
		name     string
		second   func() ([]uuid.UUID, error)
		wantCode codes.Code
	}{
		{name: "runtime_error", second: func() ([]uuid.UUID, error) { return nil, errors.New("user s2s unavailable") }, wantCode: codes.Unavailable},
		{name: "empty_profiles", second: func() ([]uuid.UUID, error) { return []uuid.UUID{}, nil }, wantCode: codes.FailedPrecondition},
		{name: "invalid_profile_id", second: func() ([]uuid.UUID, error) { return []uuid.UUID{uuid.Nil}, nil }, wantCode: codes.FailedPrecondition},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := startSocialPostgresForTest(t, ctx)
			applySocialMigration(t, ctx, pool)
			accA, accB := uuid.New(), uuid.New()
			profA, profB := uuid.New(), uuid.New()
			seedFriendship(t, ctx, pool, profA, profB, "accepted")

			client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
				s.AccountProfiles = accountProfilesResolverFunc(func(_ context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
					if accountID == accA {
						return []uuid.UUID{profA}, nil
					}
					require.Equal(t, accB, accountID)
					return tc.second()
				})
			})
			t.Cleanup(cleanup)

			_, err := client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
			require.Error(t, err)
			require.Equal(t, tc.wantCode, status.Code(err))
			requireNoDirectedBlock(t, ctx, pool, accA, accB)
			require.Equal(t, 1, countFriendshipsBetween(t, ctx, pool, []uuid.UUID{profA}, []uuid.UUID{profB}))
		})
	}
}

// TestBlockAccount_FirstResolverRuntimeError_LeavesBlockAndFriendshipsUntouched
// proves a transient failure resolving the blocking account also prevents every
// local mutation.
func TestBlockAccount_FirstResolverRuntimeError_LeavesBlockAndFriendshipsUntouched(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA, accB := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	seedFriendship(t, ctx, pool, profA, profB, "accepted")
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = accountProfilesResolverFunc(func(_ context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
			require.Equal(t, accA, accountID)
			return nil, errors.New("user s2s unavailable")
		})
	})
	t.Cleanup(cleanup)

	_, err := client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
	requireNoDirectedBlock(t, ctx, pool, accA, accB)
	require.Equal(t, 1, countFriendshipsBetween(t, ctx, pool, []uuid.UUID{profA}, []uuid.UUID{profB}))
}

// TestBlockAccount_RemovesEveryFriendshipStatusAcrossAllAccountProfiles documents
// that account-scoped blocking deletes accepted, pending, and declined rows for
// every cross-account profile pair in one cascade.
func TestBlockAccount_RemovesEveryFriendshipStatusAcrossAllAccountProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA, accB := uuid.New(), uuid.New()
	profilesA := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	profilesB := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	statuses := []string{"accepted", "pending", "declined"}
	for a, profileA := range profilesA {
		for b, profileB := range profilesB {
			if (a+b)%2 == 0 {
				seedFriendship(t, ctx, pool, profileA, profileB, statuses[(a*len(profilesB)+b)%len(statuses)])
			} else {
				seedFriendship(t, ctx, pool, profileB, profileA, statuses[(a*len(profilesB)+b)%len(statuses)])
			}
		}
	}
	require.Equal(t, len(profilesA)*len(profilesB), countFriendshipsBetween(t, ctx, pool, profilesA, profilesB))

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{accA: profilesA, accB: profilesB}
	})
	t.Cleanup(cleanup)

	_, err := client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)
	require.Zero(t, countFriendshipsBetween(t, ctx, pool, profilesA, profilesB))
}

// TestBlockAccount_PostBlockAcceptCannotCreateAcceptedFriendship exercises an
// accept after BlockAccount has removed a real pending invitation. The accept
// must observe the account block before treating the missing pending row as a
// normal not-found outcome.
func TestBlockAccount_PostBlockAcceptCannotCreateAcceptedFriendship(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA, accB := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{accA: {profA}, accB: {profB}}
		s.ProfileAccounts = stubProfileAccounts{profA: accA, profB: accB}
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withProfileAndAccountCtx(ctx, profA, accA), &socialv1.SendFriendInvitationRequest{TargetProfileId: profB.String()})
	require.NoError(t, err)
	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)

	_, err = client.AcceptFriendInvitation(withProfileAndAccountCtx(ctx, profB, accB), &socialv1.AcceptFriendInvitationRequest{RequesterProfileId: profA.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	areFriends, err := client.AreFriends(ctx, &socialv1.AreFriendsRequest{ProfileIdA: profA.String(), ProfileIdB: profB.String()})
	require.NoError(t, err)
	require.False(t, areFriends.GetFriends())
	require.Zero(t, countFriendshipsBetween(t, ctx, pool, []uuid.UUID{profA}, []uuid.UUID{profB}))
}

// TestBlockAccount_ConcurrentSendCannotCreateFriendshipAfterCommittedBlock
// holds SendInvitation inside its real SQL insert point. A correct pair lock
// lets Send finish before Block or makes it observe the block; in both orders
// no friendship row survives once BlockAccount returns.
func TestBlockAccount_ConcurrentSendCannotCreateFriendshipAfterCommittedBlock(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA, accB := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{accA: {profA}, accB: {profB}}
		s.ProfileAccounts = stubProfileAccounts{profA: accA, profB: accB}
	})
	t.Cleanup(cleanup)

	const pauseLockID int64 = 51051
	lockConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	t.Cleanup(lockConn.Release)
	_, err = lockConn.Exec(ctx, "SELECT pg_advisory_lock($1)", pauseLockID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = lockConn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", pauseLockID) })

	_, err = pool.Exec(ctx, `
CREATE OR REPLACE FUNCTION pause_friendship_insert_for_block_test() RETURNS trigger AS $$
BEGIN
  PERFORM pg_advisory_xact_lock(51051);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER pause_friendship_insert_for_block_test
BEFORE INSERT ON friendships
FOR EACH ROW EXECUTE FUNCTION pause_friendship_insert_for_block_test();`)
	require.NoError(t, err)

	sendDone := make(chan error, 1)
	go func() {
		_, sendErr := client.SendFriendInvitation(withProfileAndAccountCtx(ctx, profA, accA), &socialv1.SendFriendInvitationRequest{TargetProfileId: profB.String()})
		sendDone <- sendErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM pg_stat_activity
WHERE datname = current_database() AND wait_event_type = 'Lock' AND wait_event = 'advisory')`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)

	blockDone := make(chan error, 1)
	go func() {
		_, blockErr := client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
		blockDone <- blockErr
	}()

	_, err = lockConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", pauseLockID)
	require.NoError(t, err)
	require.NoError(t, <-sendDone)
	require.NoError(t, <-blockDone)
	require.Zero(t, countFriendshipsBetween(t, ctx, pool, []uuid.UUID{profA}, []uuid.UUID{profB}))
}
