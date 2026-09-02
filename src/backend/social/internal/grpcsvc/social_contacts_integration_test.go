package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/privacy"

	socialv1 "voice.app/voice/social/v1"
)

func TestContacts_AddListFavoriteRemove(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	owner := uuid.New()
	contact := uuid.New()

	_, err := client.AddContact(withProfileCtx(ctx, owner), &socialv1.AddContactRequest{
		TargetProfileId: contact.String(),
		Source:          "manual",
	})
	require.NoError(t, err)

	listed, err := client.ListContacts(withProfileCtx(ctx, owner), &socialv1.ListContactsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, listed.GetContactList().GetContacts(), 1)
	require.Equal(t, contact.String(), listed.GetContactList().GetContacts()[0].GetProfileId())
	require.Equal(t, "manual", listed.GetContactList().GetContacts()[0].GetSource())

	_, err = client.SetFavorite(withProfileCtx(ctx, owner), &socialv1.SetFavoriteRequest{
		FriendProfileId: contact.String(),
		Favorite:        true,
	})
	require.NoError(t, err)

	favs, err := client.ListFavorites(withProfileCtx(ctx, owner), &socialv1.ListFavoritesRequest{})
	require.NoError(t, err)
	require.Len(t, favs.GetFriendList().GetFriends(), 1)
	require.Equal(t, contact.String(), favs.GetFriendList().GetFriends()[0].GetProfileId())

	_, err = client.RemoveContact(withProfileCtx(ctx, owner), &socialv1.RemoveContactRequest{
		TargetProfileId: contact.String(),
	})
	require.NoError(t, err)

	_, err = client.SetFavorite(withProfileCtx(ctx, owner), &socialv1.SetFavoriteRequest{
		FriendProfileId: contact.String(),
		Favorite:        true,
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

type stubPhoneHashLookup map[string]uuid.UUID

func (m stubPhoneHashLookup) ProfileIDsByPhoneHashes(_ context.Context, hashes []string) (map[string]uuid.UUID, error) {
	out := make(map[string]uuid.UUID, len(hashes))
	for _, h := range hashes {
		if id, ok := m[h]; ok {
			out[h] = id
		}
	}
	return out, nil
}

type allowAllPhonePrivacy struct{}

func (allowAllPhonePrivacy) AllowPhoneSearchAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

func TestSyncPhoneContacts_WritesContacts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	owner := uuid.New()
	matched := uuid.New()
	hash := "sha256-deadbeef"

	client, cleanup := startSocialGRPCTestServer(t, pool,
		withPhoneHashLookup(stubPhoneHashLookup{hash: matched}),
		withPhoneSearchPrivacy(allowAllPhonePrivacy{}),
	)
	t.Cleanup(cleanup)

	resp, err := client.SyncPhoneContacts(withProfileCtx(ctx, owner), &socialv1.SyncPhoneContactsRequest{
		HashedPhoneNumbers: []string{hash},
	})
	require.NoError(t, err)
	require.Equal(t, []string{matched.String()}, resp.GetMatchedProfileIds())

	listed, err := client.ListContacts(withProfileCtx(ctx, owner), &socialv1.ListContactsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, listed.GetContactList().GetContacts(), 1)
	require.Equal(t, "phone_sync", listed.GetContactList().GetContacts()[0].GetSource())
}

func TestSyncPhoneContacts_FailClosedWithoutPrivacy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool,
		withPhoneHashLookup(stubPhoneHashLookup{"h1": uuid.New()}),
	)
	t.Cleanup(cleanup)

	_, err := client.SyncPhoneContacts(withProfileCtx(ctx, uuid.New()), &socialv1.SyncPhoneContactsRequest{
		HashedPhoneNumbers: []string{"h1"},
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

type stubAccountProfiles map[uuid.UUID][]uuid.UUID

func (m stubAccountProfiles) ProfileIDsForAccount(_ context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	return m[accountID], nil
}

func TestHasContact_S2S(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	owner := uuid.New()
	contact := uuid.New()
	other := uuid.New()

	missing, err := client.HasContact(ctx, &socialv1.HasContactRequest{
		OwnerProfileId:   owner.String(),
		ContactProfileId: other.String(),
	})
	require.NoError(t, err)
	require.False(t, missing.GetContact())

	_, err = client.AddContact(withProfileCtx(ctx, owner), &socialv1.AddContactRequest{
		TargetProfileId: contact.String(),
		Source:          "manual",
	})
	require.NoError(t, err)

	found, err := client.HasContact(ctx, &socialv1.HasContactRequest{
		OwnerProfileId:   owner.String(),
		ContactProfileId: contact.String(),
	})
	require.NoError(t, err)
	require.True(t, found.GetContact())
}
