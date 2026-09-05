package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"voice/backend/pkg/privacy"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
)

// ChatGuard validates chat membership and resolves DM peers (app stack: DM, two members).
// Implemented by SQL against chat_db or by S2S calls to ChatService.
type ChatGuard interface {
	EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error
	DMOtherProfileID(ctx context.Context, chatID, profileID uuid.UUID) (uuid.UUID, error)
	OtherMemberProfileIDs(ctx context.Context, chatID, profileID uuid.UUID) ([]uuid.UUID, error)
	// MemberRole returns owner | admin | member for chat_members, or "" when unknown.
	MemberRole(ctx context.Context, chatID, profileID uuid.UUID) (string, error)
}

// AuthoritativeChatTypeResolver obtains a chat's stored type from Chat. Client
// supplied ChatRef.type is routing metadata only and must never decide a DM
// account-lifecycle policy.
type AuthoritativeChatTypeResolver interface {
	ResolveChatType(ctx context.Context, chatID, profileID uuid.UUID) (chatv1.ChatType, error)
}

// ProfileAccountLookup resolves profile_id → account_id (User Service).
type ProfileAccountLookup interface {
	AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error)
}

// AccountDeletedChecker reports soft-deleted accounts from Auth, the source of
// truth for account lifecycle. DM writes fail closed when it is unavailable.
type AccountDeletedChecker interface {
	DeletedAmong(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

// AccountPairBlockChecker reports whether two accounts must not exchange DM messages (Social IsBlocked, both directions).
type AccountPairBlockChecker interface {
	AccountPairBlocked(ctx context.Context, viewerAccountID, otherAccountID uuid.UUID) (bool, error)
}

// PrivacyChecker reads recipient privacy policy for DM and attachment gates,
// plus author allow_forward for ForwardMessage (privacy.md / forward-messages.md).
type PrivacyChecker interface {
	AllowDMAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
	AllowGuestDM(ctx context.Context, profileID uuid.UUID) (bool, error)
	AllowFilesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
	AllowVoiceMessagesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
	// AllowForward reports whether profileID's messages may be forwarded by others.
	// Default when unset is true (privacy.md).
	AllowForward(ctx context.Context, profileID uuid.UUID) (bool, error)
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
