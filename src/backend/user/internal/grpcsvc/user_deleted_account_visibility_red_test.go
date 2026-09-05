package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/privacy"
	"voice/backend/user/internal/store"

	commonv1 "voice.app/voice/common/v1"
	userv1 "voice.app/voice/user/v1"
)

// deletedAccountCheckerStub is the narrow User-side Auth contract. Returning
// an account that was not requested is malformed and must fail closed.
type deletedAccountCheckerStub struct {
	deleted   map[uuid.UUID]struct{}
	err       error
	malformed bool
	calls     [][]uuid.UUID
}

func (c *deletedAccountCheckerStub) DeletedAmong(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	c.calls = append(c.calls, append([]uuid.UUID(nil), ids...))
	if c.err != nil {
		return nil, c.err
	}
	out := make(map[uuid.UUID]struct{})
	for _, id := range ids {
		if _, ok := c.deleted[id]; ok {
			out[id] = struct{}{}
		}
	}
	if c.malformed {
		out[uuid.New()] = struct{}{}
	}
	return out, nil
}

// setDeletedAccountChecker is intentionally reflective while UserGRPC lacks
// the new injection point. It keeps this RED suite compiling on the current
// train, then enforces the small internal contract once GREEN adds it.
func setDeletedAccountChecker(t *testing.T, svc *UserGRPC, checker *deletedAccountCheckerStub) {
	t.Helper()
	field := reflect.ValueOf(svc).Elem().FieldByName("DeletedAccounts")
	if !field.IsValid() {
		t.Fatalf("UserGRPC must expose DeletedAccounts Auth checker")
	}
	if !field.CanSet() {
		t.Fatalf("UserGRPC.DeletedAccounts must be injectable")
	}
	if !reflect.TypeOf(checker).AssignableTo(field.Type()) {
		t.Fatalf("UserGRPC.DeletedAccounts must accept DeletedAmong checker, got %s", field.Type())
	}
	field.Set(reflect.ValueOf(checker))
}

func TestUserGRPC_DeletedAccountsCheckerContract(t *testing.T) {
	svc := &UserGRPC{}
	checker := &deletedAccountCheckerStub{}
	setDeletedAccountChecker(t, svc, checker)
}

func startDeletedAccountVisibilityServer(t *testing.T, checker *deletedAccountCheckerStub) (context.Context, *store.ProfileStore, *store.PrivacyStore, userv1.UserServiceClient) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)
	profiles := store.NewProfileStore(pool)
	privacyStore := store.NewPrivacyStore(pool)

	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := &UserGRPC{
		Profiles: profiles,
		Privacy:  privacyStore,
		Presence: store.NewPresenceStore(rdb),
	}
	setDeletedAccountChecker(t, svc, checker)

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return ctx, profiles, privacyStore, userv1.NewUserServiceClient(conn)
}

func insertVisibleProfile(t *testing.T, ctx context.Context, profiles *store.ProfileStore, profileID, accountID uuid.UUID, username, discriminator string, primary bool) {
	t.Helper()
	_, err := profiles.Pool().Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, $3, $4, $5, $6)`, profileID, accountID, username, discriminator, username, primary)
	require.NoError(t, err)
}

func TestDeletedAccountProfiles_AreHiddenFromDirectAndBatchReads(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	deletedAccount, deletedProfile := uuid.New(), uuid.New()
	activeAccount, activeProfile := uuid.New(), uuid.New()
	checker := &deletedAccountCheckerStub{deleted: map[uuid.UUID]struct{}{deletedAccount: {}}}
	ctx, profiles, _, client := startDeletedAccountVisibilityServer(t, checker)
	insertVisibleProfile(t, ctx, profiles, deletedProfile, deletedAccount, "deletedprofile", "0001", true)
	insertVisibleProfile(t, ctx, profiles, activeProfile, activeAccount, "activeprofile", "0002", true)

	byID := &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: deletedProfile.String()}}
	_, idErr := client.GetProfile(ctx, byID)
	require.Equal(t, codes.NotFound, status.Code(idErr))

	byUsername := &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_Username{Username: "deletedprofile#0001"}}
	_, usernameErr := client.GetProfile(ctx, byUsername)
	require.Equal(t, codes.NotFound, status.Code(usernameErr))
	require.Equal(t, status.Convert(idErr).Message(), status.Convert(usernameErr).Message(), "deleted profiles must not be distinguishable by lookup path")

	batch, err := client.GetProfiles(ctx, &userv1.GetProfilesRequest{ProfileIds: []string{deletedProfile.String(), activeProfile.String()}})
	require.NoError(t, err)
	require.Equal(t, []string{activeProfile.String()}, collectProfileIDs(batch.GetProfileList().GetProfiles()))
}

func TestDeletedAccountProfiles_SearchFillsPagesAndPreservesCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	checker := &deletedAccountCheckerStub{deleted: make(map[uuid.UUID]struct{})}
	ctx, profiles, privacyStore, client := startDeletedAccountVisibilityServer(t, checker)
	insertVisibleProfile(t, ctx, profiles, viewerProfile, viewerAccount, "visibilityviewer", "0001", true)

	for i := 0; i < searchProfilesBatch; i++ {
		accountID, profileID := uuid.New(), uuid.New()
		insertVisibleProfile(t, ctx, profiles, profileID, accountID, fmt.Sprintf("a%03dhidepage", i), "0002", true)
		checker.deleted[accountID] = struct{}{}
	}
	firstAccount, firstProfile := uuid.New(), uuid.New()
	secondAccount, secondProfile := uuid.New(), uuid.New()
	insertVisibleProfile(t, ctx, profiles, firstProfile, firstAccount, "bvisiblehidepage", "0003", true)
	insertVisibleProfile(t, ctx, profiles, secondProfile, secondAccount, "cvisiblehidepage", "0004", true)
	for _, profileID := range []uuid.UUID{firstProfile, secondProfile} {
		row := store.PrivacyRowFromSettings(profileID, privacy.SettingsForPreset("gaming"))
		row.AllowFriendRequests = privacy.EveryoneWithGuests()
		_, err := privacyStore.Upsert(ctx, row)
		require.NoError(t, err)
	}

	first, err := client.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{
		Query: "hidepage",
		Page:  &commonv1.CursorPageRequest{PageSize: 1},
	})
	require.NoError(t, err)
	require.Equal(t, []string{firstProfile.String()}, collectProfileIDs(first.GetProfileList().GetProfiles()))
	require.True(t, first.GetPage().GetHasMore(), "a hidden first DB batch must not truncate the visible page")
	require.NotEmpty(t, first.GetPage().GetNextCursor())

	second, err := client.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{
		Query: "hidepage",
		Page:  &commonv1.CursorPageRequest{PageSize: 1, Cursor: first.GetPage().GetNextCursor()},
	})
	require.NoError(t, err)
	require.Equal(t, []string{secondProfile.String()}, collectProfileIDs(second.GetProfileList().GetProfiles()))
	require.False(t, second.GetPage().GetHasMore())
	require.GreaterOrEqual(t, len(checker.calls), 2, "search must continue after a deleted-only DB batch")
}

func TestDeletedAccountProfiles_DoNotRevealPresence(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	checker := &deletedAccountCheckerStub{deleted: make(map[uuid.UUID]struct{})}
	ctx, profiles, privacyStore, client := startDeletedAccountVisibilityServer(t, checker)
	insertVisibleProfile(t, ctx, profiles, ownerProfile, ownerAccount, "deletedpresence", "0001", true)
	insertVisibleProfile(t, ctx, profiles, viewerProfile, viewerAccount, "presenceviewer", "0002", true)
	row := store.PrivacyRowFromSettings(ownerProfile, privacy.SettingsForPreset("gaming"))
	row.ShowOnline = privacy.EveryoneWithGuests()
	_, err := privacyStore.Upsert(ctx, row)
	require.NoError(t, err)

	_, err = client.UpdatePresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdatePresenceRequest{Status: "online"})
	require.NoError(t, err)
	checker.deleted[ownerAccount] = struct{}{}

	direct, err := client.GetPresence(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.GetPresenceRequest{ProfileId: ownerProfile.String()})
	require.NoError(t, err)
	require.Empty(t, direct.GetPresenceStatus().GetStatus())
	require.Nil(t, direct.GetPresenceStatus().GetLastSeen())

	bulk, err := client.GetBulkPresence(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.GetBulkPresenceRequest{ProfileIds: []string{ownerProfile.String()}})
	require.NoError(t, err)
	require.NotContains(t, bulk.GetByProfileId(), ownerProfile.String(), "deleted profile presence must be absent from bulk response")
}

func TestDeletedAccountCheckerFailuresFailClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	accountID, profileID := uuid.New(), uuid.New()
	for _, checker := range []*deletedAccountCheckerStub{
		{err: errors.New("auth unavailable")},
		{malformed: true},
	} {
		ctx, profiles, _, client := startDeletedAccountVisibilityServer(t, checker)
		insertVisibleProfile(t, ctx, profiles, profileID, accountID, "failclosed", "0001", true)

		_, err := client.GetProfile(ctx, &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()}})
		require.Error(t, err)
		require.Equal(t, codes.Unavailable, status.Code(err))

		_, err = client.GetProfiles(ctx, &userv1.GetProfilesRequest{ProfileIds: []string{profileID.String()}})
		require.Error(t, err)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
}
