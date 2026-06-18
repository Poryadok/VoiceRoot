package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"voice/backend/pkg/privacy"

	filev1 "voice.app/voice/file/v1"
)

// ChatGuard validates chat membership and resolves DM peers (Phase 1: DM, two members).
// Implemented by SQL against chat_db or by S2S calls to ChatService.
type ChatGuard interface {
	EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error
	DMOtherProfileID(ctx context.Context, chatID, profileID uuid.UUID) (uuid.UUID, error)
}

// ProfileAccountLookup resolves profile_id → account_id (User Service).
type ProfileAccountLookup interface {
	AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error)
}

// AccountPairBlockChecker reports whether two accounts must not exchange DM messages (Social IsBlocked, both directions).
type AccountPairBlockChecker interface {
	AccountPairBlocked(ctx context.Context, viewerAccountID, otherAccountID uuid.UUID) (bool, error)
}

// PrivacyChecker reads recipient DM privacy policy.
type PrivacyChecker interface {
	AllowDMAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

// SpaceCoMembershipChecker checks shared space membership for privacy audiences.
type SpaceCoMembershipChecker interface {
	AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error)
}

// ProfileFriendChecker verifies if two profiles are friends or friends-of-friends.
type ProfileFriendChecker interface {
	AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
	AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
}

// FileMetadataLookup validates File Service metadata for message attachments.
type FileMetadataLookup interface {
	GetBulkMetadata(ctx context.Context, req *filev1.GetBulkMetadataRequest, opts ...grpc.CallOption) (*filev1.GetBulkMetadataResponse, error)
}
