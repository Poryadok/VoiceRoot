package grpcsvc

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/chatevents"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/privacy"

	chatv1 "voice.app/voice/chat/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startChatPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "chatdb", "")
}

func applyChatMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{"000001_init.up.sql", "000002_dm_requests.up.sql", "000003_groups.up.sql", "000004_slow_mode.up.sql", "000005_thread_settings.up.sql", "000006_e2e_enabled.up.sql", "000007_allow_guests.up.sql", "000008_folders.up.sql", "000009_folder_chats.up.sql", "000010_quick_access_chats.up.sql", "000011_deleted_for_self.up.sql", "000012_allow_guests_fail_closed.up.sql"} {
		migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db", name)
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

const headerAccountType = "x-voice-account-type"

func withGuestAccountProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = withAccountProfileCtx(ctx, accountID, profileID)
	return metadata.AppendToOutgoingContext(ctx, headerAccountType, "guest")
}

type mapProfileAccounts map[uuid.UUID]uuid.UUID

func (m mapProfileAccounts) AccountIDByProfileID(_ context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	a, ok := m[profileID]
	if !ok {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	return a, nil
}

func (m mapProfileAccounts) IsGuestProfile(_ context.Context, profileID uuid.UUID) (bool, error) {
	if _, ok := m[profileID]; !ok {
		return false, status.Error(codes.NotFound, "profile not found")
	}
	return false, nil
}

type mapLifecycleOwners map[uuid.UUID]uuid.UUID

func (m mapLifecycleOwners) AccountIDByProfileID(_ context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	accountID, ok := m[profileID]
	if !ok {
		return uuid.Nil, status.Error(codes.NotFound, "profile owner not found")
	}
	return accountID, nil
}

type allowDeletedAccounts struct{}

func (allowDeletedAccounts) DeletedAmong(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return map[uuid.UUID]struct{}{}, nil
}

type stubBlocks struct {
	blocked bool
	err     error
}

func (s stubBlocks) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.blocked, s.err
}

type allowDMPrivacyChecker struct{}

func (allowDMPrivacyChecker) AllowDMAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

func (allowDMPrivacyChecker) AllowChatSpaceInvitesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

type unavailableDMPrivacyChecker struct{}

func (unavailableDMPrivacyChecker) AllowDMAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, status.Error(codes.Unavailable, "user unavailable")
}

func (unavailableDMPrivacyChecker) AllowChatSpaceInvitesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

type chatServerOption func(*ChatGRPC)

func WithDMStore(d DMStore) chatServerOption {
	return func(c *ChatGRPC) { c.DM = d }
}

func WithLifecycleOwnerLookup(owners LifecycleOwnerLookup) chatServerOption {
	return func(c *ChatGRPC) { c.LifecycleOwners = owners }
}

// WithChatEventsPublisher wires optional NATS chat.events publisher for integration tests.
func WithChatEventsPublisher(p chatevents.Publisher) chatServerOption {
	return func(c *ChatGRPC) { c.ChatEvents = p }
}

// WithSpaceMembers wires space_db member resolution for integration tests.
func WithSpaceMembers(s *store.SpaceMembersStore) chatServerOption {
	return func(c *ChatGRPC) { c.SpaceMembers = s }
}

func WithPrivacyChecker(p PrivacyChecker) chatServerOption {
	return func(c *ChatGRPC) { c.Privacy = p }
}

func WithBlockChecker(b AccountBlockChecker) chatServerOption {
	return func(c *ChatGRPC) { c.Blocks = b }
}

func WithFriendChecker(f ProfileFriendChecker) chatServerOption {
	return func(c *ChatGRPC) { c.Friends = f }
}

func WithContactChecker(c ProfileContactChecker) chatServerOption {
	return func(s *ChatGRPC) { s.Contacts = c }
}

func WithLogger(l *slog.Logger) chatServerOption {
	return func(c *ChatGRPC) { c.Logger = l }
}

func startChatGRPCTestServer(t *testing.T, pool *pgxpool.Pool, profiles UserProfileLookup, blocks AccountBlockChecker, enrich ListChatsEnrichment, opts ...chatServerOption) (chatv1.ChatServiceClient, func()) {
	t.Helper()
	if blocks == nil {
		blocks = stubBlocks{}
	}
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	svc := &ChatGRPC{
		DM:              &store.DMStore{Pool: pool},
		Profiles:        profiles,
		LifecycleOwners: profiles,
		Blocks:          blocks,
		ListEnrich:      enrich,
		DeletedAccounts: allowDeletedAccounts{},
	}
	for _, o := range opts {
		o(svc)
	}
	if svc.Privacy == nil {
		svc.Privacy = allowDMPrivacyChecker{}
	}
	chatv1.RegisterChatServiceServer(srv, svc)
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
	return chatv1.NewChatServiceClient(conn), cleanup
}

// TestCreateDM_GetDM_NoFriendshipRequired documents PLAN app stack: DM without friendship; only blocks gate.
func TestCreateDM_GetDM_NoFriendshipRequired(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	r1, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.NotNil(t, r1.GetChat())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, r1.GetChat().GetType())
	require.Equal(t, profA.String(), r1.GetChat().GetCreatorProfileId())

	r2, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.Equal(t, r1.GetChat().GetId(), r2.GetChat().GetId(), "GetDM must return existing DM")

	ctxB := withAccountProfileCtx(ctx, accB, profB)
	r3, err := client.CreateDM(ctxB, &chatv1.CreateDMRequest{OtherProfileId: profA.String()})
	require.NoError(t, err)
	require.Equal(t, r1.GetChat().GetId(), r3.GetChat().GetId(), "other participant must resolve to same chat")
}

func TestDMRequestsInboxAcceptDecline(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	ctxB := withAccountProfileCtx(ctx, accB, profB)
	dm, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := dm.GetChat().GetId()

	mainList, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Empty(t, mainList.GetChatList().GetItems())

	requests := "requests"
	requestList, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{Inbox: &requests})
	require.NoError(t, err)
	require.Len(t, requestList.GetChatList().GetItems(), 1)
	require.Equal(t, chatID, requestList.GetChatList().GetItems()[0].GetChat().GetId())
	require.True(t, requestList.GetChatList().GetItems()[0].GetIsStranger())

	_, err = client.AcceptDMRequest(ctxB, &chatv1.AcceptDMRequestRequest{ChatId: chatID})
	require.NoError(t, err)
	mainList, err = client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, mainList.GetChatList().GetItems(), 1)

	profC := uuid.New()
	accC := uuid.New()
	profiles[profC] = accC
	dm2, err := client.CreateDM(withAccountProfileCtx(ctx, accC, profC), &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	_, err = client.DeclineDMRequest(ctxB, &chatv1.DeclineDMRequestRequest{ChatId: dm2.GetChat().GetId()})
	require.NoError(t, err)
	requestList, err = client.ListChats(ctxB, &chatv1.ListChatsRequest{Inbox: &requests})
	require.NoError(t, err)
	for _, item := range requestList.GetChatList().GetItems() {
		require.NotEqual(t, dm2.GetChat().GetId(), item.GetChat().GetId())
	}
}

func TestCreateDM_FriendRecipientMainInbox(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithFriendChecker(stubFriendChecker{ok: true}))
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	ctxB := withAccountProfileCtx(ctx, accB, profB)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)

	mainList, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, mainList.GetChatList().GetItems(), 1)
}

func TestCreateDM_ContactRecipientMainInbox(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithContactChecker(stubContactChecker{ok: true}))
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	ctxB := withAccountProfileCtx(ctx, accB, profB)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)

	mainList, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, mainList.GetChatList().GetItems(), 1)
}

func TestCreateDM_BlockedPair_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, stubBlocks{blocked: true}, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestCreateDM_BlockStatusUnavailableFailsClosed prevents a missing Social
// dependency from creating a DM whose block state cannot be checked.
func TestCreateDM_BlockStatusUnavailableFailsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accountA, accountB := uuid.New(), uuid.New()
	profileA, profileB := uuid.New(), uuid.New()
	client, cleanup := startChatGRPCTestServer(t, pool, mapProfileAccounts{profileA: accountA, profileB: accountB}, nil, nil, WithBlockChecker(nil))
	t.Cleanup(cleanup)

	_, err := client.CreateDM(withAccountProfileCtx(ctx, accountA, profileA), &chatv1.CreateDMRequest{OtherProfileId: profileB.String()})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

// TestCreateDM_PrivacyUnavailableFailsClosed prevents a missing User privacy
// dependency from bypassing the recipient's allow_dm policy.
func TestCreateDM_PrivacyUnavailableFailsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accountA, accountB := uuid.New(), uuid.New()
	profileA, profileB := uuid.New(), uuid.New()
	client, cleanup := startChatGRPCTestServer(t, pool, mapProfileAccounts{profileA: accountA, profileB: accountB}, stubBlocks{}, nil, WithPrivacyChecker(unavailableDMPrivacyChecker{}))
	t.Cleanup(cleanup)

	_, err := client.CreateDM(withAccountProfileCtx(ctx, accountA, profileA), &chatv1.CreateDMRequest{OtherProfileId: profileB.String()})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

// TestGetDM_BlockedPair_PermissionDenied documents chat-service.md: Social blocks gate DM; GetDM uses the same path as CreateDM.
func TestGetDM_BlockedPair_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, stubBlocks{blocked: true}, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestGetDM_Idempotent_RepeatedCallsSameChat documents stable DM identity: find-or-create must not fork rows.
func TestGetDM_Idempotent_RepeatedCallsSameChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	var firstID string
	for i := 0; i < 5; i++ {
		r, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
		require.NoError(t, err)
		id := r.GetChat().GetId()
		if firstID == "" {
			firstID = id
		} else {
			require.Equal(t, firstID, id, "GetDM must be idempotent")
		}
	}
	require.NotEmpty(t, firstID)
}

// TestGetDM_Stranger_OpensDialogWithoutCreateDM documents PLAN app stack: no friendship required; either side may resolve the DM via GetDM alone.
func TestGetDM_Stranger_OpensDialogWithoutCreateDM(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	rA, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, rA.GetChat().GetType())
	require.Equal(t, profA.String(), rA.GetChat().GetCreatorProfileId())

	ctxB := withAccountProfileCtx(ctx, accB, profB)
	rB, err := client.GetDM(ctxB, &chatv1.GetDMRequest{OtherProfileId: profA.String()})
	require.NoError(t, err)
	require.Equal(t, rA.GetChat().GetId(), rB.GetChat().GetId(), "stranger B must join same DM without prior CreateDM")
}

func TestCreateDM_Self_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	profiles := mapProfileAccounts{prof: acc}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, acc, prof)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: prof.String()})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateDM_UnknownProfile_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateDM_MissingAuth_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	profB := uuid.New()
	profiles := mapProfileAccounts{profB: uuid.New()}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.CreateDM(ctx, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestCreateDM_GuestCaller_PermissionDenied documents auth-and-contacts.md: guests cannot initiate DM.
func TestCreateDM_GuestCaller_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accGuest := uuid.New()
	accB := uuid.New()
	profGuest := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profGuest: accGuest, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxGuest := withGuestAccountProfileCtx(ctx, accGuest, profGuest)
	_, err := client.CreateDM(ctxGuest, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestCreateDM_GuestCaller_AllowGuestDMStillDenied documents guest-initiated DM stays blocked
// even when recipient allow_guest_dm=true (docs: guest actions do not depend on recipient settings).
func TestCreateDM_GuestCaller_AllowGuestDMStillDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accGuest := uuid.New()
	accB := uuid.New()
	profGuest := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profGuest: accGuest, profB: accB}

	// Recipient privacy would allow guest DM (allow_guest_dm=true); guest initiation must still fail.
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithPrivacyChecker(dmPrivacyStub{}),
	)
	t.Cleanup(cleanup)

	ctxGuest := withGuestAccountProfileCtx(ctx, accGuest, profGuest)
	_, err := client.CreateDM(ctxGuest, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestEnsureDM_DeletedPeerIsPrivacySafeAndIdempotentlyDenied documents PLAN A1
// and auth-and-contacts.md: no fresh device can create or reopen a DM with a
// deleted peer. Replays must not reveal deletion or create another chat row.
func TestEnsureDM_DeletedPeerIsPrivacySafeAndIdempotentlyDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA, profA := uuid.New(), uuid.New()
	accDeletedExisting, profDeletedExisting := uuid.New(), uuid.New()
	accDeletedAbsent, profDeletedAbsent := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{
		profA:               accA,
		profDeletedExisting: accDeletedExisting,
		profDeletedAbsent:   accDeletedAbsent,
	}

	// This is a pre-delete conversation. It must not be re-openable after the
	// deleted-account gate is attached to the fresh-snapshot service instance.
	_, _, err := (&store.DMStore{Pool: pool}).EnsureDM(ctx, profA, profDeletedExisting, store.InboxMain)
	require.NoError(t, err)

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithAccountDeletedChecker(mapDeletedAccounts{
			accDeletedExisting: {},
			accDeletedAbsent:   {},
		}),
	)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)

	operations := []struct {
		name string
		call func() error
	}{
		{
			name: "create_existing_dm",
			call: func() error {
				_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profDeletedExisting.String()})
				return err
			},
		},
		{
			name: "get_existing_dm",
			call: func() error {
				_, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profDeletedExisting.String()})
				return err
			},
		},
		{
			name: "create_absent_dm",
			call: func() error {
				_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profDeletedAbsent.String()})
				return err
			},
		},
		{
			name: "get_absent_dm",
			call: func() error {
				_, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profDeletedAbsent.String()})
				return err
			},
		},
	}
	for attempt := 0; attempt < 2; attempt++ {
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				err := operation.call()
				require.Error(t, err)
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				require.NotContains(t, strings.ToLower(status.Convert(err).Message()), "deleted",
					"the denial must not disclose the peer's account state")
			})
		}
	}

	var dmCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM chats WHERE type = 'dm'`).Scan(&dmCount)
	require.NoError(t, err)
	require.Equal(t, 1, dmCount, "denied repeats must not create a DM with the deleted peer")
}

// TestEnsureDM_DeletedAccountGateFailuresAreUnavailable documents the A1
// fail-closed boundary: when Chat cannot establish deleted-account state, both
// CreateDM and GetDM must fail honestly rather than create or return a DM.
func TestEnsureDM_DeletedAccountGateFailuresAreUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	tests := []struct {
		name           string
		missingProfile bool
		profileFailure bool
		deleted        AccountDeletedChecker
	}{
		{name: "missing_auth_checker"},
		{name: "missing_profile_lookup", missingProfile: true, deleted: mapDeletedAccounts{}},
		{
			name:    "auth_checker_failure",
			deleted: unavailableDeletedAccounts{},
		},
		{
			name:           "profile_lookup_failure",
			profileFailure: true,
			deleted:        mapDeletedAccounts{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accCaller, profCaller := uuid.New(), uuid.New()
			profPeer := uuid.New()
			var profiles UserProfileLookup = mapProfileAccounts{profCaller: accCaller, profPeer: uuid.New()}
			if tt.missingProfile {
				profiles = nil
			} else if tt.profileFailure {
				profiles = unavailableProfileLookup{}
			}

			options := []chatServerOption{WithAccountDeletedChecker(tt.deleted)}
			client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, options...)
			t.Cleanup(cleanup)
			ctxCaller := withAccountProfileCtx(ctx, accCaller, profCaller)

			created, createErr := client.CreateDM(ctxCaller, &chatv1.CreateDMRequest{OtherProfileId: profPeer.String()})
			got, getErr := client.GetDM(ctxCaller, &chatv1.GetDMRequest{OtherProfileId: profPeer.String()})
			var dmCount int
			err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM chats WHERE type = 'dm'`).Scan(&dmCount)
			require.NoError(t, err)

			require.Error(t, createErr)
			require.Nil(t, created, "a failed deleted-account gate must not return a DM")
			require.Equal(t, codes.Unavailable, status.Code(createErr))
			require.Error(t, getErr)
			require.Nil(t, got, "a failed deleted-account gate must not return a DM")
			require.Equal(t, codes.Unavailable, status.Code(getErr))
			require.Zero(t, dmCount, "a failed deleted-account gate must not create a DM")
		})
	}
}
