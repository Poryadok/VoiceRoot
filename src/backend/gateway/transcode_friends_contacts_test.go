package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	socialv1 "voice.app/voice/social/v1"
)

type recordingSocialContacts struct {
	socialv1.UnimplementedSocialServiceServer
	lastListContacts  *socialv1.ListContactsRequest
	lastAddContact    *socialv1.AddContactRequest
	lastListFavorites *socialv1.ListFavoritesRequest
	lastSetFavorite   *socialv1.SetFavoriteRequest
}

func (s *recordingSocialContacts) ListContacts(_ context.Context, req *socialv1.ListContactsRequest) (*socialv1.ListContactsResponse, error) {
	s.lastListContacts = req
	return &socialv1.ListContactsResponse{
		ContactList: &socialv1.ContactList{
			Contacts: []*socialv1.Contact{
				{ProfileId: "contact-1", Source: "manual", IsFavorite: true},
			},
			NextCursor: "next-c",
		},
	}, nil
}

func (s *recordingSocialContacts) AddContact(_ context.Context, req *socialv1.AddContactRequest) (*socialv1.AddContactResponse, error) {
	s.lastAddContact = req
	return &socialv1.AddContactResponse{}, nil
}

func (s *recordingSocialContacts) ListFavorites(_ context.Context, _ *socialv1.ListFavoritesRequest) (*socialv1.ListFavoritesResponse, error) {
	s.lastListFavorites = &socialv1.ListFavoritesRequest{}
	return &socialv1.ListFavoritesResponse{
		FriendList: &socialv1.FriendList{
			Friends: []*socialv1.FriendEdge{{ProfileId: "fav-1"}},
		},
	}, nil
}

func (s *recordingSocialContacts) SetFavorite(_ context.Context, req *socialv1.SetFavoriteRequest) (*socialv1.SetFavoriteResponse, error) {
	s.lastSetFavorite = req
	return &socialv1.SetFavoriteResponse{}, nil
}

func TestTranscodeFriendsContactsAndFavorites(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSocialContacts{}
	conn, cleanup := startBufconnSocialConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{social: socialv1.NewSocialServiceClient(conn)}},
	})

	auth := map[string]string{"Authorization": "Bearer valid-user-token"}

	t.Run("list_contacts", func(t *testing.T) {
		t.Parallel()
		rec := performRequest(h, http.MethodGet, "/api/v1/friends/contacts?cursor=c0&page_size=25", "", auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		require.NotNil(t, grpcRec.lastListContacts)
		require.Equal(t, "c0", grpcRec.lastListContacts.GetPage().GetCursor())
		require.EqualValues(t, 25, grpcRec.lastListContacts.GetPage().GetPageSize())
		require.Contains(t, rec.Body.String(), "contact-1")
	})

	t.Run("add_contact", func(t *testing.T) {
		t.Parallel()
		body := `{"target_profile_id":"p-target","source":"manual"}`
		rec := performRequest(h, http.MethodPost, "/api/v1/friends/contacts", body, auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		require.NotNil(t, grpcRec.lastAddContact)
		require.Equal(t, "p-target", grpcRec.lastAddContact.GetTargetProfileId())
		require.Equal(t, "manual", grpcRec.lastAddContact.GetSource())
	})

	t.Run("list_favorites", func(t *testing.T) {
		t.Parallel()
		rec := performRequest(h, http.MethodGet, "/api/v1/friends/favorites", "", auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		require.NotNil(t, grpcRec.lastListFavorites)
		require.Contains(t, rec.Body.String(), "fav-1")
	})

	t.Run("set_favorite", func(t *testing.T) {
		t.Parallel()
		body := `{"friend_profile_id":"p-friend","favorite":true}`
		rec := performRequest(h, http.MethodPost, "/api/v1/friends/favorites", body, auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		require.NotNil(t, grpcRec.lastSetFavorite)
		require.Equal(t, "p-friend", grpcRec.lastSetFavorite.GetFriendProfileId())
		require.True(t, grpcRec.lastSetFavorite.GetFavorite())
	})
}

func TestTranscodeFriendsContactsPrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSocialContacts{}
	conn, cleanup := startBufconnSocialConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{social: socialv1.NewSocialServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"friends": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/friends/contacts" {
					proxyCalled = true
				}
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/friends/contacts", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled, "REST proxy must not run when gRPC transcoder handles /api/v1/friends/contacts")
	require.NotNil(t, grpcRec.lastListContacts)
}

func TestTranscodeFriendsContactsSyncStillWorks(t *testing.T) {
	t.Parallel()

	var syncCalled bool
	grpcRec := &recordingSocialContacts{}
	conn, cleanup := startBufconnSocialConn(t, &syncPhoneContactsServer{
		inner: grpcRec,
		onSync: func() { syncCalled = true },
	})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{social: socialv1.NewSocialServiceClient(conn)}},
	})

	rec := performRequest(h, http.MethodPost, "/api/v1/friends/contacts/sync", `{"hashed_phone_numbers":["hash-1"]}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.True(t, syncCalled)
}

type syncPhoneContactsServer struct {
	socialv1.UnimplementedSocialServiceServer
	inner  *recordingSocialContacts
	onSync func()
}

func (s *syncPhoneContactsServer) ListContacts(ctx context.Context, req *socialv1.ListContactsRequest) (*socialv1.ListContactsResponse, error) {
	return s.inner.ListContacts(ctx, req)
}

func (s *syncPhoneContactsServer) SyncPhoneContacts(ctx context.Context, req *socialv1.SyncPhoneContactsRequest) (*socialv1.SyncPhoneContactsResponse, error) {
	if s.onSync != nil {
		s.onSync()
	}
	_ = req
	return &socialv1.SyncPhoneContactsResponse{}, nil
}
