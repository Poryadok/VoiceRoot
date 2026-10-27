package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

type recordingSpaceCatalog struct {
	spacev1.UnimplementedSpaceServiceServer
	last *spacev1.SearchPublicSpacesRequest
}

func (s *recordingSpaceCatalog) SearchPublicSpaces(_ context.Context, req *spacev1.SearchPublicSpacesRequest) (*spacev1.SearchPublicSpacesResponse, error) {
	s.last = req
	return &spacev1.SearchPublicSpacesResponse{SpaceList: &spacev1.SpaceList{Spaces: []*spacev1.Space{{Id: "catalog-space", Name: "Raid", Description: "Friday raids", MemberCount: 42, IsVerified: true}}, NextCursor: "next"}}, nil
}

func TestSpaceCatalogRoute(t *testing.T) {
	rec := &recordingSpaceCatalog{}
	conn, cleanup := startBufconnSpaceConn(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"valid": {UserID: "account-1", ProfileID: "profile-1"}},
		transcoder:  &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})
	denied := performRequest(h, http.MethodGet, "/api/v1/spaces/catalog", "", nil)
	require.Equal(t, http.StatusUnauthorized, denied.Code)
	require.Nil(t, rec.last)
	for _, path := range []string{"/api/v1/spaces/catalog?q=raid&cursor=old&page_size=7", "/api/v1/spaces/catalog"} {
		resp := performRequest(h, http.MethodGet, path, "", map[string]string{"Authorization": "Bearer valid"})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		require.NotNil(t, rec.last)
		require.Contains(t, resp.Body.String(), `"is_verified":true`)
		require.Contains(t, resp.Body.String(), `"member_count":42`)
		if rec.last.Query != "" {
			require.Equal(t, "raid", rec.last.Query)
			require.EqualValues(t, 7, rec.last.Page.PageSize)
			require.Equal(t, "old", rec.last.Page.Cursor)
		}
	}
}
