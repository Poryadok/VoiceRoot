package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

// SearchPublicSpaces permits authenticated nonmembers to read only catalog cards.
// It deliberately does not call GetSpace or its unrestricted row projection.
func (s *SpaceGRPC) SearchPublicSpaces(ctx context.Context, req *spacev1.SearchPublicSpacesRequest) (*spacev1.SearchPublicSpacesResponse, error) {
	if _, ok := authctx.ProfileID(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "space catalog unavailable")
	}
	query, cursor, limit := "", "", 50
	if req != nil {
		query = strings.TrimSpace(req.GetQuery())
		if page := req.GetPage(); page != nil {
			cursor = page.GetCursor()
			if page.GetPageSize() > 0 {
				limit = int(page.GetPageSize())
			}
		}
	}
	page, err := s.Store.SearchPublicSpaces(ctx, query, cursor, limit)
	if errors.Is(err, store.ErrInvalidListCursor) {
		return nil, status.Error(codes.InvalidArgument, "invalid catalog cursor")
	}
	if err != nil {
		return nil, mapSpaceStoreError(err)
	}
	spaces := make([]*spacev1.Space, 0, len(page.Spaces))
	for _, row := range page.Spaces {
		spaces = append(spaces, &spacev1.Space{Id: row.ID.String(), Name: row.Name, Description: row.Description, IconUrl: row.IconURL, MemberCount: row.MemberCount, IsVerified: row.IsVerified})
	}
	return &spacev1.SearchPublicSpacesResponse{SpaceList: &spacev1.SpaceList{Spaces: spaces, NextCursor: page.NextCursor}}, nil
}
