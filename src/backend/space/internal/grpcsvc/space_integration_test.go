package grpcsvc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/spaceevents"
	"voice/backend/space/internal/store"

	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startSpacePostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "spacedb", "")
}

func applySpaceMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{"000001_init.up.sql", "000002_tree.up.sql", "000003_invites.up.sql", "000004_moderation.up.sql", "000005_space_subscriptions.up.sql"} {
		migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

func withAccountProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

type mapProfileAccounts map[uuid.UUID]uuid.UUID

func (m mapProfileAccounts) AccountIDByProfileID(_ context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	a, ok := m[profileID]
	if !ok {
		return uuid.Nil, errors.New("profile not found")
	}
	return a, nil
}

type spySpaceEvents struct {
	mu      sync.Mutex
	created [][2]string // space_id, owner_profile_id
}

func (s *spySpaceEvents) PublishSpaceCreated(_ context.Context, spaceID, ownerProfileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created = append(s.created, [2]string{spaceID, ownerProfileID})
	return nil
}

func (*spySpaceEvents) PublishTreeNodeUpserted(context.Context, string, string, string, string, string) error {
	return nil
}

func (*spySpaceEvents) PublishTreeNodeRemoved(context.Context, string, string) error {
	return nil
}

func (*spySpaceEvents) PublishVoiceRoomCreated(context.Context, string, string) error { return nil }

func (*spySpaceEvents) PublishVoiceRoomDeleted(context.Context, string, string) error { return nil }

func (*spySpaceEvents) PublishInviteCreated(context.Context, string, string) error { return nil }

func (s *spySpaceEvents) snapshot() [][2]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][2]string(nil), s.created...)
}

type errSpaceEvents struct{}

func (errSpaceEvents) PublishSpaceCreated(context.Context, string, string) error {
	return errors.New("nats unavailable")
}

func (errSpaceEvents) PublishTreeNodeUpserted(context.Context, string, string, string, string, string) error {
	return nil
}

func (errSpaceEvents) PublishTreeNodeRemoved(context.Context, string, string) error { return nil }

func (errSpaceEvents) PublishVoiceRoomCreated(context.Context, string, string) error { return nil }

func (errSpaceEvents) PublishVoiceRoomDeleted(context.Context, string, string) error { return nil }

func (errSpaceEvents) PublishInviteCreated(context.Context, string, string) error {
	return errors.New("nats unavailable")
}

type spaceServerOption func(*SpaceGRPC)

func withSpaceEventsPublisher(p spaceevents.Publisher) spaceServerOption {
	return func(s *SpaceGRPC) { s.SpaceEvents = p }
}

func withRoleClient(c rolev1.RoleServiceClient) spaceServerOption {
	return func(s *SpaceGRPC) { s.Roles = c }
}

func withProfileAccounts(m mapProfileAccounts) spaceServerOption {
	return func(s *SpaceGRPC) { s.ProfileAccounts = m }
}

func withSpaceChatLookup(l ChatLookup) spaceServerOption {
	return func(s *SpaceGRPC) { s.Chats = l }
}

func startSpaceGRPCTestServer(t *testing.T, pool *pgxpool.Pool, opts ...spaceServerOption) (spacev1.SpaceServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: pool}}
	for _, o := range opts {
		o(svc)
	}
	spacev1.RegisterSpaceServiceServer(srv, svc)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
	}
	return spacev1.NewSpaceServiceClient(conn), cleanup
}

func profileFixture(t *testing.T) (owner, account uuid.UUID, ctx context.Context) {
	t.Helper()
	account = uuid.New()
	owner = uuid.New()
	ctx = withAccountProfileCtx(context.Background(), account, owner)
	return owner, account, ctx
}

func countSpaceMembers(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID, profileID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM space_members WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID).Scan(&n)
	require.NoError(t, err)
	return n
}

// TestCreateSpace_OwnerGetsSpaceWithDefaults documents PLAN Phase 5 / spaces.md:
// registered user creates space with name + description; default visibility private.
func TestCreateSpace_OwnerGetsSpaceWithDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Friday squad",
		Description: "We raid on Fridays",
	})
	require.NoError(t, err)
	space := resp.GetSpace()
	require.NotNil(t, space)
	require.NotEmpty(t, space.GetId())
	require.Equal(t, "Friday squad", space.GetName())
	require.Equal(t, "We raid on Fridays", space.GetDescription())
	require.Equal(t, owner.String(), space.GetOwnerProfileId())
	require.Equal(t, int32(1), space.GetMemberCount())
	require.Equal(t, "private", space.GetVisibility())
	require.NotNil(t, space.GetCreatedAt())
	require.NotNil(t, space.GetUpdatedAt())
}

// TestCreateSpace_EmptyName_InvalidArgument documents validation on required name.
func TestCreateSpace_EmptyName_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "   ",
		Description: "ignored",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateSpace_Unauthenticated documents missing JWT/profile metadata.
func TestCreateSpace_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateSpace(context.Background(), &spacev1.CreateSpaceRequest{
		Name:        "No auth",
		Description: "should fail",
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestUpdateSpace_IconAndDescription_Owner documents icon via UpdateSpace after create (group avatar pattern).
func TestUpdateSpace_IconAndDescription_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Icon space",
		Description: "Before icon",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	icon := "https://cdn.voice.gg/spaces/party.webp"
	desc := "After icon"
	updated, err := client.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
		SpaceId:     spaceID,
		IconUrl:     &icon,
		Description: &desc,
	})
	require.NoError(t, err)
	require.Equal(t, icon, updated.GetSpace().GetIconUrl())
	require.Equal(t, desc, updated.GetSpace().GetDescription())
	require.Equal(t, owner.String(), updated.GetSpace().GetOwnerProfileId())
}

// TestGetSpace_ReturnsDescriptionAndIcon documents owner can fetch space metadata.
func TestGetSpace_ReturnsDescriptionAndIcon(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Readable",
		Description: "About us",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	icon := "https://cdn.voice.gg/spaces/readable.webp"
	_, err = client.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
		SpaceId: spaceID,
		IconUrl: &icon,
	})
	require.NoError(t, err)

	got, err := client.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, "Readable", got.GetSpace().GetName())
	require.Equal(t, "About us", got.GetSpace().GetDescription())
	require.Equal(t, icon, got.GetSpace().GetIconUrl())
}

// TestListMySpaces_IncludesCreatedSpace documents created space appears in sidebar list.
func TestListMySpaces_IncludesCreatedSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Listed",
		Description: "Shows up",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	list, err := client.ListMySpaces(ctx, &spacev1.ListMySpacesRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.NotNil(t, list.GetSpaceList())
	found := false
	for _, item := range list.GetSpaceList().GetSpaces() {
		if item.GetId() != spaceID {
			continue
		}
		found = true
		require.Equal(t, "Listed", item.GetName())
		require.Equal(t, "Shows up", item.GetDescription())
	}
	require.True(t, found, "created space must appear in ListMySpaces")
}

// TestCreateSpace_SpaceCreatedEvent documents space-service.md NATS space.created on create.
func TestCreateSpace_SpaceCreatedEvent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	spy := &spySpaceEvents{}
	client, cleanup := startSpaceGRPCTestServer(t, pool, withSpaceEventsPublisher(spy))
	t.Cleanup(cleanup)

	resp, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Events",
		Description: "NATS",
	})
	require.NoError(t, err)
	spaceID := resp.GetSpace().GetId()
	ownerID := resp.GetSpace().GetOwnerProfileId()

	events := spy.snapshot()
	require.Len(t, events, 1)
	require.Equal(t, spaceID, events[0][0])
	require.Equal(t, ownerID, events[0][1])
}

// TestCreateSpace_InsertsCreatorAsMember documents space_members row for creator on create.
func TestCreateSpace_InsertsCreatorAsMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Membership",
		Description: "Creator is member",
	})
	require.NoError(t, err)
	spaceID, err := uuid.Parse(resp.GetSpace().GetId())
	require.NoError(t, err)
	require.Equal(t, 1, countSpaceMembers(t, context.Background(), pool, spaceID, owner))
}

// TestCreateSpace_PublicVisibility documents space-service.md visibility types on create.
func TestCreateSpace_PublicVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Public hub",
		Description: "Open to all",
		Visibility:  "public",
	})
	require.NoError(t, err)
	require.Equal(t, "public", resp.GetSpace().GetVisibility())
}

// TestCreateSpace_InviteOnlyVisibility documents invite_only visibility on create.
func TestCreateSpace_InviteOnlyVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Invite squad",
		Description: "Link only",
		Visibility:  "invite_only",
	})
	require.NoError(t, err)
	require.Equal(t, "invite_only", resp.GetSpace().GetVisibility())
}

// TestCreateSpace_InvalidVisibility documents DB constraint on unknown visibility values.
func TestCreateSpace_InvalidVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Bad vis",
		Description: "should fail",
		Visibility:  "secret",
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

// TestUpdateSpace_DescriptionOnly_Owner documents description-only PATCH after create.
func TestUpdateSpace_DescriptionOnly_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Desc only",
		Description: "Before",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	desc := "After description-only"
	updated, err := client.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
		SpaceId:     spaceID,
		Description: &desc,
	})
	require.NoError(t, err)
	require.Equal(t, desc, updated.GetSpace().GetDescription())
	require.Equal(t, "Desc only", updated.GetSpace().GetName())
}

// TestUpdateSpace_NonOwner_PermissionDenied documents only owner may update space metadata.
func TestUpdateSpace_NonOwner_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	otherAccount := uuid.New()
	otherProfile := uuid.New()
	otherCtx := withAccountProfileCtx(context.Background(), otherAccount, otherProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{
		Name:        "Owned",
		Description: "Not yours",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	icon := "https://cdn.voice.gg/spaces/hijack.webp"
	_, err = client.UpdateSpace(otherCtx, &spacev1.UpdateSpaceRequest{
		SpaceId: spaceID,
		IconUrl: &icon,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestGetSpace_NotMember_PermissionDenied documents non-members cannot read space metadata.
func TestGetSpace_NotMember_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	otherCtx := withAccountProfileCtx(context.Background(), uuid.New(), uuid.New())

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{
		Name:        "Private meta",
		Description: "Members only",
	})
	require.NoError(t, err)

	_, err = client.GetSpace(otherCtx, &spacev1.GetSpaceRequest{SpaceId: created.GetSpace().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestGetSpace_NotFound documents missing space id.
func TestGetSpace_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Exists",
		Description: "For membership",
	})
	require.NoError(t, err)
	_ = created

	_, err = client.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: uuid.New().String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestListMySpaces_InvalidCursor_InvalidArgument documents bad pagination cursor.
func TestListMySpaces_InvalidCursor_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.ListMySpaces(ctx, &spacev1.ListMySpacesRequest{
		Page: &commonv1.CursorPageRequest{Cursor: "not-a-valid-cursor", PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateSpace_PublishFailureStillCreates documents chat-style degradation: NATS failure must not fail create.
func TestCreateSpace_PublishFailureStillCreates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withSpaceEventsPublisher(errSpaceEvents{}))
	t.Cleanup(cleanup)

	resp, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Resilient",
		Description: "NATS down",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetSpace().GetId())
}

// TestUpdateSpace_NotFound documents missing space id on update.
func TestUpdateSpace_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	desc := "nope"
	_, err := client.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
		SpaceId:     uuid.New().String(),
		Description: &desc,
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

// TestUpdateSpace_InvalidSpaceID documents UUID validation on update.
func TestUpdateSpace_InvalidSpaceID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	desc := "nope"
	_, err := client.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
		SpaceId:     "not-a-uuid",
		Description: &desc,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestGetSpace_InvalidSpaceID documents UUID validation on get.
func TestGetSpace_InvalidSpaceID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: "bad-id"})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestUpdateSpace_Unauthenticated documents missing profile on update.
func TestUpdateSpace_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	desc := "nope"
	_, err := client.UpdateSpace(context.Background(), &spacev1.UpdateSpaceRequest{
		SpaceId:     uuid.New().String(),
		Description: &desc,
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestGetSpace_Unauthenticated documents missing profile on get.
func TestGetSpace_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.GetSpace(context.Background(), &spacev1.GetSpaceRequest{SpaceId: uuid.New().String()})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestListMySpaces_Unauthenticated documents missing profile on list.
func TestListMySpaces_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.ListMySpaces(context.Background(), &spacev1.ListMySpacesRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestUpdateSpace_NameAndBanner_Owner documents name + banner updates.
func TestUpdateSpace_NameAndBanner_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Before rename",
		Description: "Banner test",
	})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	name := "After rename"
	banner := "https://cdn.voice.gg/spaces/banner.webp"
	updated, err := client.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
		SpaceId:   spaceID,
		Name:      &name,
		BannerUrl: &banner,
	})
	require.NoError(t, err)
	require.Equal(t, name, updated.GetSpace().GetName())
	require.Equal(t, banner, updated.GetSpace().GetBannerUrl())
}

const spaceHeaderAccountType = "x-voice-account-type"

func withGuestAccountProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = withAccountProfileCtx(ctx, accountID, profileID)
	return metadata.AppendToOutgoingContext(ctx, spaceHeaderAccountType, "guest")
}

// TestCreateSpace_GuestCaller_PermissionDenied documents auth-and-contacts.md: guests cannot create spaces.
func TestCreateSpace_GuestCaller_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	guestAccount := uuid.New()
	guestProfile := uuid.New()
	ctx := withGuestAccountProfileCtx(context.Background(), guestAccount, guestProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:        "Guest space",
		Description: "should be denied",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestJoinByInvite_GuestBlockedWhenAllowGuestsFalse documents space allow_guests enforcement.
func TestJoinByInvite_GuestBlockedWhenAllowGuestsFalse(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	guestAccount := uuid.New()
	guestProfile := uuid.New()
	guestCtx := withGuestAccountProfileCtx(context.Background(), guestAccount, guestProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	bg := context.Background()
	_, err := pool.Exec(bg, `ALTER TABLE spaces ADD COLUMN IF NOT EXISTS allow_guests BOOLEAN NOT NULL DEFAULT true`)
	require.NoError(t, err)

	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "No guests"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	_, err = pool.Exec(bg, `UPDATE spaces SET allow_guests = false WHERE id = $1`, spaceID)
	require.NoError(t, err)

	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)

	_, err = client.JoinByInvite(guestCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
