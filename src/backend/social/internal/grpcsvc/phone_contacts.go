package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/social/internal/authctx"
	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"

	socialv1 "voice.app/voice/social/v1"
)

// PhoneHashLookup resolves hashed phone numbers to profile IDs.
type PhoneHashLookup interface {
	ProfileIDsByPhoneHashes(ctx context.Context, hashes []string) (map[string]uuid.UUID, error)
}

// PhoneSearchPrivacyChecker reads allow_phone_search audience.
type PhoneSearchPrivacyChecker interface {
	AllowPhoneSearchAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

func (s *SocialGRPC) SyncPhoneContacts(ctx context.Context, req *socialv1.SyncPhoneContactsRequest) (*socialv1.SyncPhoneContactsResponse, error) {
	if s == nil {
		return nil, status.Error(codes.FailedPrecondition, "social service not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.PhoneHashes == nil {
		return nil, status.Error(codes.FailedPrecondition, "phone hash lookup not configured")
	}
	hashes := req.GetHashedPhoneNumbers()
	if len(hashes) == 0 {
		return &socialv1.SyncPhoneContactsResponse{}, nil
	}
	byHash, err := s.PhoneHashes.ProfileIDsByPhoneHashes(ctx, hashes)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	matched := make([]string, 0, len(byHash))
	for _, profileID := range byHash {
		if err := s.ensurePhoneSearchPrivacy(ctx, caller, profileID); err != nil {
			if status.Code(err) == codes.PermissionDenied {
				continue
			}
			return nil, err
		}
		matched = append(matched, profileID.String())
	}
	return &socialv1.SyncPhoneContactsResponse{MatchedProfileIds: matched}, nil
}

func (s *SocialGRPC) ensurePhoneSearchPrivacy(ctx context.Context, searcher, owner uuid.UUID) error {
	if s == nil || s.PhoneSearchPrivacy == nil {
		return nil
	}
	audience, err := s.PhoneSearchPrivacy.AllowPhoneSearchAudience(ctx, owner)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	var graph privacy.SocialGraph
	if s.Friends != nil {
		graph = s.Friends
	}
	matcher := privacy.Matcher{Social: graph, Space: s.SpaceCoMembership}
	if err := privacy.CheckAllowed(matcher, ctx, owner, searcher, audience, guestguard.IsGuest(ctx)); err != nil {
		if errors.Is(err, privacy.ErrDenied) {
			return status.Error(codes.PermissionDenied, "phone search blocked by privacy settings")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
