package grpcsvc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	commonv1 "voice.app/voice/common/v1"
	userv1 "voice.app/voice/user/v1"
)

type testBlockChecker struct {
	fn func(viewer, other uuid.UUID) bool
}

func (c *testBlockChecker) AccountPairBlocked(ctx context.Context, viewer, other uuid.UUID) (bool, error) {
	if c == nil || c.fn == nil {
		return false, nil
	}
	return c.fn(viewer, other), nil
}

func collectProfileIDs(ps []*userv1.Profile) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.GetId())
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// .../internal/grpcsvc/user_integration_test.go -> repo root is 5 parents up
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func TestProfileGRPC_v1DDL(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:16-bookworm",
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase("userdb"),
		postgres.WithUsername("u"),
		postgres.WithPassword("p"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	if strings.Contains(connStr, "localhost") {
		connStr = strings.Replace(connStr, "localhost", "127.0.0.1", 1)
	}
	connStr = strings.Replace(connStr, "[::1]", "127.0.0.1", 1)

	var pool *pgxpool.Pool
	for i := 0; i < 60; i++ {
		p, err := pgxpool.New(ctx, connStr)
		if err == nil {
			if pingErr := p.Ping(ctx); pingErr == nil {
				pool = p
				break
			}
			p.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NotNil(t, pool, "postgres did not become ready in time")
	t.Cleanup(pool.Close)

	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "user_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)

	accountA := uuid.New()
	accountB := uuid.New()
	pid := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'alice', '0001', 'Alice', true)`,
		pid, accountA)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	blocker := &testBlockChecker{}
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, &UserGRPC{
		Profiles: store.NewProfileStore(pool),
		Presence: store.NewPresenceStore(rdb),
		Blocks:   blocker,
	})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	cli := userv1.NewUserServiceClient(conn)

	t.Run("GetProfile by id", func(t *testing.T) {
		resp, err := cli.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_ProfileId{ProfileId: pid.String()},
		})
		require.NoError(t, err)
		require.Equal(t, pid.String(), resp.GetProfile().GetId())
		require.Equal(t, "alice", resp.GetProfile().GetUsername())
		require.Equal(t, "0001", resp.GetProfile().GetDiscriminator())
		require.True(t, resp.GetProfile().GetIsPrimary())
	})

	t.Run("GetProfile by handle", func(t *testing.T) {
		resp, err := cli.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_Username{Username: "alice#0001"},
		})
		require.NoError(t, err)
		require.Equal(t, pid.String(), resp.GetProfile().GetId())
	})

	t.Run("GetProfile invalid handle", func(t *testing.T) {
		_, err := cli.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_Username{Username: "alice"},
		})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("GetProfiles batch", func(t *testing.T) {
		pid2 := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'bob', '0002', 'Bob', false)`,
			pid2, accountA)
		require.NoError(t, err)

		resp, err := cli.GetProfiles(ctx, &userv1.GetProfilesRequest{
			ProfileIds: []string{pid.String(), pid2.String()},
		})
		require.NoError(t, err)
		require.Len(t, resp.GetProfileList().GetProfiles(), 2)
	})

	t.Run("UpdateProfile forbidden for other account", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountB.String())
		_, err := cli.UpdateProfile(mdCtx, &userv1.UpdateProfileRequest{
			ProfileId:   pid.String(),
			DisplayName: proto.String("Hacker"),
		})
		require.Error(t, err)
		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("UpdateProfile ok", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.UpdateProfile(mdCtx, &userv1.UpdateProfileRequest{
			ProfileId:   pid.String(),
			DisplayName: proto.String("Alice II"),
			Locale:      proto.String("en"),
			Theme:       proto.String("light"),
		})
		require.NoError(t, err)
		require.Equal(t, "Alice II", resp.GetProfile().GetDisplayName())
		require.Equal(t, "en", resp.GetProfile().GetLocale())
		require.Equal(t, "light", resp.GetProfile().GetTheme())
	})

	t.Run("CreateProfile secondary", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.CreateProfile(mdCtx, &userv1.CreateProfileRequest{
			DisplayName: "Alt",
			Username:    proto.String("altuser"),
		})
		require.NoError(t, err)
		require.False(t, resp.GetProfile().GetIsPrimary())
		require.Equal(t, accountA.String(), resp.GetProfile().GetAccountId())
		require.Equal(t, "Alt", resp.GetProfile().GetDisplayName())
		require.NotEmpty(t, resp.GetProfile().GetDiscriminator())
	})

	t.Run("GetOnboardingState unauthenticated", func(t *testing.T) {
		_, err := cli.GetOnboardingState(ctx, &userv1.GetOnboardingStateRequest{})
		require.Error(t, err)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("GetOnboardingState creates row and uses primary when profile header omitted", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM onboarding_state WHERE profile_id = $1`, pid)
		require.NoError(t, err)
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.GetOnboardingState(mdCtx, &userv1.GetOnboardingStateRequest{})
		require.NoError(t, err)
		os := resp.GetOnboardingState()
		require.Equal(t, pid.String(), os.GetProfileId())
		require.False(t, os.GetCompleted())
		require.Empty(t, os.GetCompletedSteps())
	})

	t.Run("GetOnboardingState wrong profile for account", func(t *testing.T) {
		pidOther := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'carol', '0003', 'Carol', true)`,
			pidOther, accountB)
		require.NoError(t, err)
		mdCtx := metadata.AppendToOutgoingContext(ctx,
			authctx.HeaderUserID, accountA.String(),
			authctx.HeaderProfileID, pidOther.String(),
		)
		_, err = cli.GetOnboardingState(mdCtx, &userv1.GetOnboardingStateRequest{})
		require.Error(t, err)
		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("CompleteOnboardingStep invalid step_id", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String(), authctx.HeaderProfileID, pid.String())
		_, err := cli.CompleteOnboardingStep(mdCtx, &userv1.CompleteOnboardingStepRequest{StepId: ""})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("CompleteOnboardingStep dismiss marks completed", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM onboarding_state WHERE profile_id = $1`, pid)
		require.NoError(t, err)
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String(), authctx.HeaderProfileID, pid.String())
		r1, err := cli.CompleteOnboardingStep(mdCtx, &userv1.CompleteOnboardingStepRequest{StepId: store.OnboardingStepSaveAccount})
		require.NoError(t, err)
		require.Contains(t, r1.GetOnboardingState().GetCompletedSteps(), store.OnboardingStepSaveAccount)
		require.False(t, r1.GetOnboardingState().GetCompleted())

		r2, err := cli.CompleteOnboardingStep(mdCtx, &userv1.CompleteOnboardingStepRequest{StepId: store.OnboardingStepDismiss})
		require.NoError(t, err)
		require.True(t, r2.GetOnboardingState().GetCompleted())
		require.NotNil(t, r2.GetOnboardingState().GetCompletedAt())

		r3, err := cli.CompleteOnboardingStep(mdCtx, &userv1.CompleteOnboardingStepRequest{StepId: store.OnboardingStepChatsNav})
		require.NoError(t, err)
		require.True(t, r3.GetOnboardingState().GetCompleted())
	})

	t.Run("CompleteOnboardingStep all canonical steps marks completed", func(t *testing.T) {
		accountC := uuid.New()
		pidTutorial := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'dana', '0004', 'Dana', true)`,
			pidTutorial, accountC)
		require.NoError(t, err)
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountC.String(), authctx.HeaderProfileID, pidTutorial.String())
		for _, step := range []string{
			store.OnboardingStepSaveAccount,
			store.OnboardingStepChatsNav,
			store.OnboardingStepSpaces,
			store.OnboardingStepMatchmaking,
		} {
			resp, err := cli.CompleteOnboardingStep(mdCtx, &userv1.CompleteOnboardingStepRequest{StepId: step})
			require.NoError(t, err)
			require.False(t, resp.GetOnboardingState().GetCompleted())
		}
		rfin, err := cli.CompleteOnboardingStep(mdCtx, &userv1.CompleteOnboardingStepRequest{StepId: store.OnboardingStepWrapUp})
		require.NoError(t, err)
		require.True(t, rfin.GetOnboardingState().GetCompleted())
		require.NotNil(t, rfin.GetOnboardingState().GetCompletedAt())
	})

	t.Run("UpdatePresence unauthenticated", func(t *testing.T) {
		_, err := cli.UpdatePresence(ctx, &userv1.UpdatePresenceRequest{Status: "online"})
		require.Error(t, err)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("UpdatePresence invalid status", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		_, err := cli.UpdatePresence(mdCtx, &userv1.UpdatePresenceRequest{Status: "nope"})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("UpdatePresence GetPresence GetBulkPresence", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String(), authctx.HeaderProfileID, pid.String())
		_, err := cli.UpdatePresence(mdCtx, &userv1.UpdatePresenceRequest{
			Status:       "online",
			CustomStatus: proto.String("coding"),
		})
		require.NoError(t, err)

		g1, err := cli.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: pid.String()})
		require.NoError(t, err)
		ps := g1.GetPresenceStatus()
		require.Equal(t, "online", ps.GetStatus())
		require.Equal(t, "coding", ps.GetCustomStatus())
		require.NotNil(t, ps.GetLastSeen())

		pid2 := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'eve', '0005', 'Eve', false)`,
			pid2, accountA)
		require.NoError(t, err)
		md2 := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String(), authctx.HeaderProfileID, pid2.String())
		_, err = cli.UpdatePresence(md2, &userv1.UpdatePresenceRequest{StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND.Enum()})
		require.NoError(t, err)

		gb, err := cli.GetBulkPresence(ctx, &userv1.GetBulkPresenceRequest{ProfileIds: []string{pid.String(), pid2.String()}})
		require.NoError(t, err)
		m := gb.GetByProfileId()
		require.Equal(t, "online", m[pid.String()].GetStatus())
		require.Equal(t, userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND, m[pid2.String()].GetStatusEnum())
	})

	t.Run("UpdatePresence rejects profile not owned", func(t *testing.T) {
		var otherPID uuid.UUID
		err := pool.QueryRow(ctx, `SELECT id FROM profiles WHERE account_id = $1 LIMIT 1`, accountB).Scan(&otherPID)
		require.NoError(t, err)
		mdCtx := metadata.AppendToOutgoingContext(ctx,
			authctx.HeaderUserID, accountA.String(),
			authctx.HeaderProfileID, otherPID.String(),
		)
		_, err = cli.UpdatePresence(mdCtx, &userv1.UpdatePresenceRequest{Status: "online"})
		require.Error(t, err)
		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("SearchProfiles unauthenticated", func(t *testing.T) {
		_, err := cli.SearchProfiles(ctx, &userv1.SearchProfilesRequest{Query: "x"})
		require.Error(t, err)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("SearchProfiles invalid empty query", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		_, err := cli.SearchProfiles(mdCtx, &userv1.SearchProfilesRequest{Query: "  "})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("SearchProfiles finds other account", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.SearchProfiles(mdCtx, &userv1.SearchProfilesRequest{Query: "Carol"})
		require.NoError(t, err)
		ids := collectProfileIDs(resp.GetProfileList().GetProfiles())
		var carolID string
		err = pool.QueryRow(ctx, `SELECT id::text FROM profiles WHERE username = 'carol' AND account_id = $1`, accountB).Scan(&carolID)
		require.NoError(t, err)
		require.Contains(t, ids, carolID)
	})

	t.Run("SearchProfiles excludes own account", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.SearchProfiles(mdCtx, &userv1.SearchProfilesRequest{Query: "alice"})
		require.NoError(t, err)
		for _, p := range resp.GetProfileList().GetProfiles() {
			require.NotEqual(t, accountA.String(), p.GetAccountId(), "must not return viewer account profiles")
		}
	})

	t.Run("SearchProfiles respects block checker", func(t *testing.T) {
		accD := uuid.New()
		pidD := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'blockedfind', '0099', 'BlockedFindMe', true)`,
			pidD, accD)
		require.NoError(t, err)
		blocker.fn = func(viewer, other uuid.UUID) bool {
			return viewer == accountA && other == accD
		}
		t.Cleanup(func() { blocker.fn = nil })
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.SearchProfiles(mdCtx, &userv1.SearchProfilesRequest{Query: "BlockedFind"})
		require.NoError(t, err)
		for _, p := range resp.GetProfileList().GetProfiles() {
			require.NotEqual(t, pidD.String(), p.GetId())
		}
	})

	t.Run("SearchProfiles cursor pagination", func(t *testing.T) {
		accP1 := uuid.New()
		accP2 := uuid.New()
		pidP1 := uuid.New()
		pidP2 := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'aaapag', '0010', 'PagUnique Alpha', true)`,
			pidP1, accP1)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'zzzpag', '0011', 'PagUnique Zed', true)`,
			pidP2, accP2)
		require.NoError(t, err)
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		r1, err := cli.SearchProfiles(mdCtx, &userv1.SearchProfilesRequest{
			Query: "PagUnique",
			Page:  &commonv1.CursorPageRequest{PageSize: 1},
		})
		require.NoError(t, err)
		require.True(t, r1.GetPage().GetHasMore())
		require.Len(t, r1.GetProfileList().GetProfiles(), 1)
		require.Equal(t, "aaapag", r1.GetProfileList().GetProfiles()[0].GetUsername())
		r2, err := cli.SearchProfiles(mdCtx, &userv1.SearchProfilesRequest{
			Query: "PagUnique",
			Page: &commonv1.CursorPageRequest{
				Cursor:   r1.GetPage().GetNextCursor(),
				PageSize: 1,
			},
		})
		require.NoError(t, err)
		require.False(t, r2.GetPage().GetHasMore())
		require.Len(t, r2.GetProfileList().GetProfiles(), 1)
		require.Equal(t, "zzzpag", r2.GetProfileList().GetProfiles()[0].GetUsername())
	})
}
