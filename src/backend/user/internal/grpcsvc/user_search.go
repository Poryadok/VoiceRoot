package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	commonv1 "voice.app/voice/common/v1"
	userv1 "voice.app/voice/user/v1"
)

const (
	searchProfilesDefaultPage = 20
	searchProfilesMaxPage     = 50
	searchProfilesMaxQuery    = 128
	searchProfilesBatch       = 40
	searchProfilesMaxIters    = 32
)

// SearchProfiles discovers profiles by username/display_name substring (user_db).
// v1 DDL has no privacy_settings row yet (see docs/microservices/user-service.md); block filtering uses optional Social S2S when Blocks is set.
func (s *UserGRPC) SearchProfiles(ctx context.Context, req *userv1.SearchProfilesRequest) (*userv1.SearchProfilesResponse, error) {
	viewer, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	q := strings.TrimSpace(req.GetQuery())
	if q == "" {
		return nil, status.Error(codes.InvalidArgument, "query required")
	}
	if len(q) > searchProfilesMaxQuery {
		return nil, status.Error(codes.InvalidArgument, "query too long")
	}

	page := req.GetPage()
	pageSize := 0
	if page != nil {
		pageSize = int(page.GetPageSize())
	}
	if pageSize <= 0 {
		pageSize = searchProfilesDefaultPage
	}
	if pageSize > searchProfilesMaxPage {
		pageSize = searchProfilesMaxPage
	}

	cursorIn := ""
	if page != nil {
		cursorIn = page.GetCursor()
	}
	after, err := store.DecodeSearchCursor(cursorIn)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var scan *store.ProfileSearchCursor
	if after != nil {
		scan = after
	}

	want := pageSize + 1
	out := make([]*userv1.Profile, 0, want)
	emittedRows := make([]*store.ProfileRow, 0, want)
	iter := 0
	dbExhausted := false
	var lastEmitted *store.ProfileRow

	for len(out) < want && iter < searchProfilesMaxIters {
		iter++
		rows, err := s.Profiles.SearchProfilesAfter(ctx, viewer, q, scan, searchProfilesBatch)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if len(rows) < searchProfilesBatch {
			dbExhausted = true
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			scan = profileRowSearchCursor(row)
			blocked, err := s.pairwiseBlocked(ctx, viewer, row.AccountID)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			if blocked {
				continue
			}
			out = append(out, rowToProto(row))
			emittedRows = append(emittedRows, row)
			if len(out) >= want {
				break
			}
		}
		if len(out) >= want {
			break
		}
		if dbExhausted {
			break
		}
	}

	hasMore := len(out) > pageSize
	if hasMore {
		out = out[:pageSize]
		emittedRows = emittedRows[:pageSize]
	}
	if len(emittedRows) > 0 {
		lastEmitted = emittedRows[len(emittedRows)-1]
	}

	next := ""
	if hasMore && lastEmitted != nil {
		c := profileSearchCursorFromRow(lastEmitted)
		next, err = store.EncodeSearchCursor(c)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &userv1.SearchProfilesResponse{
		ProfileList: &userv1.ProfileList{Profiles: out},
		Page: &commonv1.CursorPageResponse{
			NextCursor: next,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *UserGRPC) pairwiseBlocked(ctx context.Context, viewer, other uuid.UUID) (bool, error) {
	if s.Blocks == nil {
		return false, nil
	}
	return s.Blocks.AccountPairBlocked(ctx, viewer, other)
}

func profileRowSearchCursor(p *store.ProfileRow) *store.ProfileSearchCursor {
	c := profileSearchCursorFromRow(p)
	return &c
}

func profileSearchCursorFromRow(p *store.ProfileRow) store.ProfileSearchCursor {
	return store.ProfileSearchCursor{
		UsernameLower: strings.ToLower(p.Username),
		Discriminator: p.Discriminator,
		ID:            p.ID,
	}
}
