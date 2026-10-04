package testsocial

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/privacy"
	"voice/backend/social/internal/authctx"

	socialv1 "voice.app/voice/social/v1"
)

type fixtureAccountProfiles map[uuid.UUID][]uuid.UUID

func (m fixtureAccountProfiles) ProfileIDsForAccount(_ context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	return m[accountID], nil
}

type friendRequestPrivacyFixture struct {
	called    int
	profileID uuid.UUID
	audience  privacy.Audience
}

func (f *friendRequestPrivacyFixture) AllowFriendRequestsAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	f.called++
	f.profileID = profileID
	return f.audience, nil
}

type spaceCoMembershipFixture struct {
	called             int
	profileA, profileB uuid.UUID
	spaceIDs           []string
	allowed            bool
}

func (f *spaceCoMembershipFixture) AreCoMembers(_ context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error) {
	f.called++
	f.profileA, f.profileB = profileA, profileB
	f.spaceIDs = append([]string(nil), spaceIDs...)
	return f.allowed, nil
}

func TestNewBufconnClientWiresPrivacyDependenciesAndFailsClosed(t *testing.T) {
	caller := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	account := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	target := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	privacyCheck := &friendRequestPrivacyFixture{audience: privacy.Audience{
		SpaceMembers: true,
		SpaceIDs:     []string{"44444444-4444-4444-8444-444444444444"},
	}}
	spaceCheck := &spaceCoMembershipFixture{allowed: true}
	conn, cleanup := NewBufconnClient(t, nil, Dependencies{
		AccountProfiles:   fixtureAccountProfiles{},
		Privacy:           privacyCheck,
		SpaceCoMembership: spaceCheck,
	})
	t.Cleanup(cleanup)
	client := socialv1.NewSocialServiceClient(conn)
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		authctx.HeaderProfileID, caller.String(),
		authctx.HeaderUserID, account.String(),
	)

	_, err := client.SendFriendInvitation(ctx, &socialv1.SendFriendInvitationRequest{TargetProfileId: target.String()})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "request must fail closed at the next unconfigured production dependency")
	require.Equal(t, "social block check not configured", status.Convert(err).Message())
	require.Equal(t, 1, privacyCheck.called, "target privacy must be read")
	require.Equal(t, target, privacyCheck.profileID)
	require.Equal(t, 1, spaceCheck.called, "Space audience must consult the injected co-membership dependency")
	require.Equal(t, caller, spaceCheck.profileA)
	require.Equal(t, target, spaceCheck.profileB)
	require.Equal(t, []string{"44444444-4444-4444-8444-444444444444"}, spaceCheck.spaceIDs)

	spaceCheck.allowed = false
	privacyCheck.called = 0
	spaceCheck.called = 0
	_, err = client.SendFriendInvitation(ctx, &socialv1.SendFriendInvitationRequest{TargetProfileId: target.String()})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "missing shared Space membership must deny the request")
	require.Equal(t, 1, privacyCheck.called)
	require.Equal(t, 1, spaceCheck.called)
}

func socialRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

// TestNewBufconnClient_BlockAccount_CascadesFixtureProfiles requires the shared
// cross-service fixture to accept the explicit User-owned account -> profile
// mapping. BlockAccount is account-scoped, but its friendship cascade needs all
// mapped profile IDs before mutating Social storage.
func TestNewBufconnClient_BlockAccount_CascadesFixtureProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	root := socialRepoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "socialdb", filepath.Join(root, "src", "backend", "migrations", "social_db", "000001_init.up.sql"))
	identityMigration, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "social_db", "000003_blocked_profile_identity.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(identityMigration))
	require.NoError(t, err)

	blockerAccount := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	blockedAccount := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	blockerProfile := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	blockedProfile := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	_, err = pool.Exec(ctx, `
		INSERT INTO friendships (id, requester_profile_id, target_profile_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'accepted', now(), now())`, uuid.New(), blockerProfile, blockedProfile)
	require.NoError(t, err)

	conn, cleanup := NewBufconnClient(t, pool, Dependencies{AccountProfiles: fixtureAccountProfiles{
		blockerAccount: {blockerProfile},
		blockedAccount: {blockedProfile},
	}})
	t.Cleanup(cleanup)
	client := socialv1.NewSocialServiceClient(conn)

	_, err = client.BlockAccount(
		metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, blockerAccount.String()),
		&socialv1.BlockAccountRequest{BlockedAccountId: blockedAccount.String()},
	)
	require.NoError(t, err)

	var blocks int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM blocks WHERE blocker_account_id = $1 AND blocked_account_id = $2`, blockerAccount, blockedAccount).Scan(&blocks)
	require.NoError(t, err)
	require.Equal(t, 1, blocks)

	var friendships int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM friendships WHERE requester_profile_id = $1 AND target_profile_id = $2`, blockerProfile, blockedProfile).Scan(&friendships)
	require.NoError(t, err)
	require.Zero(t, friendships)
}
