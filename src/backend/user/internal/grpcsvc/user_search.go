package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"
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
)

// SearchProfiles discovers profiles by username/display_name substring (user_db).
// Discovery uses the target profile's allow_friend_requests audience and pairwise blocks.
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
	viewerProfile, err := s.resolveOwnedActiveProfile(ctx, viewer)
	if err != nil {
		return nil, err
	}
	privacyStore := s.privacyStore()

	want := pageSize + 1
	out := make([]*userv1.Profile, 0, want)
	emittedRows := make([]*store.ProfileRow, 0, want)
	dbExhausted := false
	var lastEmitted *store.ProfileRow

	for len(out) < want {
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
		}
		rows, err = s.filterDeletedAccountProfiles(ctx, rows)
		if err != nil {
			return nil, deletedAccountCheckUnavailable(err)
		}
		for _, row := range rows {
			blocked, err := s.pairwiseBlocked(ctx, viewer, row.AccountID)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			if blocked {
				continue
			}
			if !s.mayDiscoverProfile(ctx, privacyStore, viewerProfile, row) {
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

func (s *UserGRPC) mayDiscoverProfile(ctx context.Context, privacyStore *store.PrivacyStore, viewerProfile uuid.UUID, target *store.ProfileRow) bool {
	if privacyStore == nil || target == nil {
		return false
	}
	settings, err := privacyStore.GetByProfileID(ctx, target.ID)
	if err != nil {
		return false
	}
	if settings == nil {
		// GetPrivacySettings bootstraps this same canonical default. Search must not
		// create state on a read path, but must evaluate absent rows identically.
		defaults := store.PrivacyRowFromSettings(target.ID, privacy.SettingsForPreset("gaming"))
		settings = &defaults
	}
	allowed, err := s.audienceMatcher().Allowed(ctx, target.ID, viewerProfile, settings.AllowFriendRequests, guestguard.IsGuest(ctx))
	if err != nil {
		return false
	}
	return allowed
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
	verificationRank := 1
	if p.VerificationType != "none" {
		verificationRank = 0
	}
	return store.ProfileSearchCursor{
		VerificationRank: &verificationRank,
		UsernameLower:    strings.ToLower(p.Username),
		Discriminator:    p.Discriminator,
		ID:               p.ID,
	}
}
