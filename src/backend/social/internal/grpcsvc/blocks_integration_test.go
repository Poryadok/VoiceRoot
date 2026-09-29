package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/integrationtest"

	"voice/backend/social/internal/authctx"

	commonv1 "voice.app/voice/common/v1"
	socialv1 "voice.app/voice/social/v1"
)

func withAccountCtx(ctx context.Context, accountID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
}

func startSocialPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "socialdb", "")
}

func applySocialMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := repoRoot(t)
	for _, name := range []string{"000001_init.up.sql", "000002_contacts.up.sql", "000003_blocked_profile_identity.up.sql", "000004_friend_accept_outbox.up.sql"} {
		migrationPath := filepath.Join(root, "src", "backend", "migrations", "social_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

func TestBlockFlow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {uuid.New()},
		}
	})
	t.Cleanup(cleanup)

	// Missing account in metadata
	_, err := client.BlockAccount(withProfileCtx(ctx, profA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// Self block
	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accA.String()})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	// A blocks B
	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)
	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)

	lb, err := client.ListBlocked(withAccountCtx(ctx, accA), &socialv1.ListBlockedRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, lb.GetBlockedList().GetBlocked(), 1)
	require.Equal(t, accB.String(), lb.GetBlockedList().GetBlocked()[0].GetBlockedAccountId())

	_, err = client.UnblockAccount(withAccountCtx(ctx, accA), &socialv1.UnblockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)
	_, err = client.UnblockAccount(withAccountCtx(ctx, accA), &socialv1.UnblockAccountRequest{BlockedAccountId: accB.String()})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))

	lb2, err := client.ListBlocked(withAccountCtx(ctx, accA), &socialv1.ListBlockedRequest{})
	require.NoError(t, err)
	require.Empty(t, lb2.GetBlockedList().GetBlocked())

	// Internal IsBlocked: ordered pair (blocker, blocked); no user metadata required.
	rNo, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accA.String(),
		AccountIdB: accB.String(),
	})
	require.NoError(t, err)
	require.False(t, rNo.GetBlocked())

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)
	rAB, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accA.String(),
		AccountIdB: accB.String(),
	})
	require.NoError(t, err)
	require.True(t, rAB.GetBlocked())
	rBA, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: accB.String(),
		AccountIdB: accA.String(),
	})
	require.NoError(t, err)
	require.False(t, rBA.GetBlocked())
}

func TestListBlocked_InvalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	acc := uuid.New()
	_, err := client.ListBlocked(withAccountCtx(ctx, acc), &socialv1.ListBlockedRequest{
		Page: &commonv1.CursorPageRequest{Cursor: "garbage", PageSize: 5},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestIsBlocked_SelfPair_NotBlocked(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	acc := uuid.New()
	r, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: acc.String(),
		AccountIdB: acc.String(),
	})
	require.NoError(t, err)
	require.False(t, r.GetBlocked())
}

func TestIsBlocked_InvalidUUIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: "",
		AccountIdB: uuid.New().String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListBlocked_PaginationRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	acc := uuid.New()
	b1 := uuid.New()
	b2 := uuid.New()
	b3 := uuid.New()
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			acc: {uuid.New()},
			b1:  {uuid.New()},
			b2:  {uuid.New()},
			b3:  {uuid.New()},
		}
	})
	t.Cleanup(cleanup)

	for _, blocked := range []uuid.UUID{b1, b2, b3} {
		_, err := client.BlockAccount(withAccountCtx(ctx, acc), &socialv1.BlockAccountRequest{BlockedAccountId: blocked.String()})
		require.NoError(t, err)
	}

	page1, err := client.ListBlocked(withAccountCtx(ctx, acc), &socialv1.ListBlockedRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 1},
	})
	require.NoError(t, err)
	require.Len(t, page1.GetBlockedList().GetBlocked(), 1)
	require.NotEmpty(t, page1.GetBlockedList().GetNextCursor())

	page2, err := client.ListBlocked(withAccountCtx(ctx, acc), &socialv1.ListBlockedRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 1, Cursor: page1.GetBlockedList().GetNextCursor()},
	})
	require.NoError(t, err)
	require.Len(t, page2.GetBlockedList().GetBlocked(), 1)
	require.NotEqual(t, page1.GetBlockedList().GetBlocked()[0].GetBlockedAccountId(),
		page2.GetBlockedList().GetBlocked()[0].GetBlockedAccountId())

	all, err := client.ListBlocked(withAccountCtx(ctx, acc), &socialv1.ListBlockedRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, all.GetBlockedList().GetBlocked(), 3)
}

type stubBlockedProfiles map[uuid.UUID]BlockedProfile

func (m stubBlockedProfiles) Profile(_ context.Context, profileID uuid.UUID) (BlockedProfile, error) {
	return m[profileID], nil
}

type blockedProfileResolverFunc func(context.Context, uuid.UUID) (BlockedProfile, error)

func (f blockedProfileResolverFunc) Profile(ctx context.Context, profileID uuid.UUID) (BlockedProfile, error) {
	return f(ctx, profileID)
}

// A block applies to an account, but its list may only reveal the profile the
// blocker actually selected. Sibling and primary profiles remain undisclosed.
func TestListBlocked_PersistsSelectedProfileSnapshotAndLegacyFallback(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	blocker := uuid.New()
	blocked := uuid.New()
	legacyBlocked := uuid.New()
	primary := uuid.New()
	selected := uuid.New()
	profiles := stubBlockedProfiles{
		primary:  {AccountID: blocked, DisplayName: "Private Primary", Username: "private", Discriminator: "1111"},
		selected: {AccountID: blocked, DisplayName: "Known Gaming", Username: "known", Discriminator: "2222"},
	}
	configure := func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			blocker:       {uuid.New()},
			blocked:       {primary, selected},
			legacyBlocked: {uuid.New()},
		}
		s.BlockedProfiles = profiles
	}
	client, cleanup := startSocialGRPCTestServer(t, pool, configure)

	_, err := client.BlockAccount(withAccountCtx(ctx, blocker), &socialv1.BlockAccountRequest{
		BlockedAccountId: blocked.String(),
		BlockedProfileId: selected.String(),
	})
	require.NoError(t, err)
	// Pre-profile-ID callers still produce an account block and a neutral list row.
	_, err = client.BlockAccount(withAccountCtx(ctx, blocker), &socialv1.BlockAccountRequest{
		BlockedAccountId: legacyBlocked.String(),
	})
	require.NoError(t, err)
	cleanup()

	// Start a fresh service instance to ensure the identity came from storage,
	// rather than an in-memory resolver or a request-local value.
	client, cleanup = startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	list, err := client.ListBlocked(withAccountCtx(ctx, blocker), &socialv1.ListBlockedRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetBlockedList().GetBlocked(), 2)
	byAccount := make(map[string]*socialv1.BlockedAccount)
	for _, row := range list.GetBlockedList().GetBlocked() {
		byAccount[row.GetBlockedAccountId()] = row
	}
	selectedRow := byAccount[blocked.String()]
	require.NotNil(t, selectedRow)
	require.Equal(t, selected.String(), selectedRow.GetBlockedProfileId())
	require.Equal(t, "Known Gaming", selectedRow.GetDisplayName())
	require.Equal(t, "known", selectedRow.GetUsername())
	require.Equal(t, "2222", selectedRow.GetDiscriminator())
	require.NotContains(t, selectedRow.String(), primary.String())
	require.NotContains(t, selectedRow.String(), "Private Primary")

	legacyRow := byAccount[legacyBlocked.String()]
	require.NotNil(t, legacyRow)
	require.Empty(t, legacyRow.GetBlockedProfileId())
	require.Empty(t, legacyRow.GetDisplayName())
	require.Empty(t, legacyRow.GetUsername())
	require.Empty(t, legacyRow.GetDiscriminator())
}

func TestBlockAccount_RejectsProfileOutsideBlockedAccount(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	blocker := uuid.New()
	blocked := uuid.New()
	other := uuid.New()
	foreignProfile := uuid.New()
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			blocker: {uuid.New()},
			blocked: {uuid.New()},
			other:   {foreignProfile},
		}
		s.BlockedProfiles = stubBlockedProfiles{
			foreignProfile: {AccountID: other, DisplayName: "Other Person", Username: "other", Discriminator: "3333"},
		}
	})
	t.Cleanup(cleanup)

	_, err := client.BlockAccount(withAccountCtx(ctx, blocker), &socialv1.BlockAccountRequest{
		BlockedAccountId: blocked.String(),
		BlockedProfileId: foreignProfile.String(),
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	list, err := client.ListBlocked(withAccountCtx(ctx, blocker), &socialv1.ListBlockedRequest{})
	require.NoError(t, err)
	require.Empty(t, list.GetBlockedList().GetBlocked())
}

func TestBlockAccount_SelectedProfileLookupUnavailableDoesNotCreateBlock(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	blocker := uuid.New()
	blocked := uuid.New()
	selected := uuid.New()
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			blocker: {uuid.New()},
			blocked: {selected},
		}
		s.BlockedProfiles = blockedProfileResolverFunc(func(context.Context, uuid.UUID) (BlockedProfile, error) {
			return BlockedProfile{}, status.Error(codes.Unavailable, "user lookup unavailable")
		})
	})
	t.Cleanup(cleanup)

	_, err := client.BlockAccount(withAccountCtx(ctx, blocker), &socialv1.BlockAccountRequest{
		BlockedAccountId: blocked.String(),
		BlockedProfileId: selected.String(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	list, err := client.ListBlocked(withAccountCtx(ctx, blocker), &socialv1.ListBlockedRequest{})
	require.NoError(t, err)
	require.Empty(t, list.GetBlockedList().GetBlocked())
}
