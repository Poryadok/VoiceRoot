package grpcsvc

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"

	"voice/backend/chat/internal/chatevents"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/privacy"

	rolev1 "voice.app/voice/role/v1"
)

// ChatGRPC implements ChatService RPCs backed by chat_db (app stack: DM).
type ChatGRPC struct {
	chatv1.UnimplementedChatServiceServer
	DM                DMStore
	Profiles          UserProfileLookup
	Blocks            AccountBlockChecker
	Privacy           PrivacyChecker
	Friends           ProfileFriendChecker
	Contacts          ProfileContactChecker
	SpaceCoMembership SpaceCoMembershipChecker
	ListEnrich        ListChatsEnrichment   // optional; Messaging S2S for preview + unread
	DeletedAccounts   AccountDeletedChecker // mandatory for DM list/open gates; Auth S2S reports deleted peer accounts
	E2EPreKeyGate     E2EPreKeyGate         // required for EnableChatE2E; Messaging S2S pre-key check (fail-closed)
	// ChatEvents is optional; when set, new DM creation publishes to NATS JetStream (stream chat_events, subjects chat.*).
	ChatEvents chatevents.Publisher
	// Roles is optional; space channel slow mode checks TEXT_CHAT_SET_SLOW_MODE when set.
	Roles rolev1.RoleServiceClient
	// SpaceMembers resolves space_db.space_members for chats with space_id (optional).
	SpaceMembers *store.SpaceMembersStore
	// Logger emits structured nats_publish errors when JetStream publish fails after a successful RPC.
	Logger *slog.Logger
}

// PrivacyChecker reads recipient privacy policy for DM and invite gates.
type PrivacyChecker interface {
	AllowDMAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
	AllowChatSpaceInvitesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
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

// DMStore persists chats and lists the caller's inbox (DM + standalone groups).
type DMStore interface {
	EnsureDM(ctx context.Context, callerProfileID, otherProfileID uuid.UUID, recipientInboxBucket string) (*store.ChatRow, bool, error)
	ListChatsPage(ctx context.Context, viewerProfileID uuid.UUID, cursor string, limit int, inbox string, spaceIDs []uuid.UUID) (*store.ListChatsPage, error)
	ListChatsPageByFolder(ctx context.Context, viewerProfileID, folderID uuid.UUID, cursor string, limit int, spaceIDs []uuid.UUID) (*store.ListChatsPage, error)
	ListSpaceChatsForProfile(ctx context.Context, viewerProfileID uuid.UUID, spaceIDs []uuid.UUID) ([]*store.ChatRow, error)
	DMPeerProfileIDs(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error)
	FindDMChatByID(ctx context.Context, chatID uuid.UUID) (*store.ChatRow, error)
	FindChatByID(ctx context.Context, chatID uuid.UUID) (*store.ChatRow, error)
	IsChatMember(ctx context.Context, chatID, profileID uuid.UUID) (bool, error)
	ListChatMembers(ctx context.Context, chatID uuid.UUID) ([]store.ChatMemberRow, error)
	SetInboxBucket(ctx context.Context, chatID, profileID uuid.UUID, bucket string) error
	SetMemberArchived(ctx context.Context, chatID, profileID uuid.UUID, archived bool) error
	SetMemberMutedUntil(ctx context.Context, chatID, profileID uuid.UUID, until *time.Time) error
	MarkChatDeletedForSelf(ctx context.Context, chatID, profileID uuid.UUID) error
	ClearDeletedForSelf(ctx context.Context, chatID, profileID uuid.UUID) error
	IsMemberDeletedForSelf(ctx context.Context, chatID, profileID uuid.UUID) (bool, error)
	ListQuickAccess(ctx context.Context, profileID uuid.UUID) ([]store.QuickAccessRow, error)
	AddQuickAccess(ctx context.Context, profileID, chatID uuid.UUID, sortOrder *int32) error
	RemoveQuickAccess(ctx context.Context, profileID, chatID uuid.UUID) error
	ReorderQuickAccess(ctx context.Context, profileID uuid.UUID, chatIDs []uuid.UUID) error
	ListFolders(ctx context.Context, profileID uuid.UUID) ([]store.FolderRow, error)
	CreateFolder(ctx context.Context, profileID uuid.UUID, name, filterConfigJSON string) (*store.FolderRow, error)
	UpdateFolder(ctx context.Context, profileID, folderID uuid.UUID, upd store.FolderUpdate) (*store.FolderRow, error)
	DeleteFolder(ctx context.Context, profileID, folderID uuid.UUID) error
	GetFolder(ctx context.Context, profileID, folderID uuid.UUID) (*store.FolderRow, error)
	AddChatToFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID, sortOrder *int32) error
	RemoveChatFromFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID) error
	ReorderFolderChats(ctx context.Context, profileID, folderID uuid.UUID, chatIDs []uuid.UUID) error
	PinChatInFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID, pinOrder *int32) error
	UnpinChatInFolder(ctx context.Context, profileID, folderID, chatID uuid.UUID) error
	CreateGroupChat(ctx context.Context, creatorProfileID uuid.UUID, name string, topic *string) (*store.ChatRow, error)
	CreateChannelChat(ctx context.Context, creatorProfileID uuid.UUID, name string, topic *string) (*store.ChatRow, error)
	CreateSpaceGroupChat(ctx context.Context, creatorProfileID, spaceID uuid.UUID, name string, topic *string) (*store.ChatRow, error)
	CreateSpaceChannelChat(ctx context.Context, creatorProfileID, spaceID uuid.UUID, name string, topic *string) (*store.ChatRow, error)
	AddGroupMembers(ctx context.Context, chatID uuid.UUID, profileIDs []uuid.UUID) ([]uuid.UUID, error)
	RemoveGroupMember(ctx context.Context, chatID, profileID uuid.UUID) error
	LeaveGroupChat(ctx context.Context, chatID, profileID uuid.UUID) error
	TransferGroupOwnership(ctx context.Context, chatID, ownerID, newOwnerID uuid.UUID) error
	UpdateGroupChat(ctx context.Context, chatID uuid.UUID, name, avatarURL, topic *string, slowModeSeconds *int32, threadsEnabled, allowUserMainFeed *bool) (*store.ChatRow, error)
	GetMemberRole(ctx context.Context, chatID, profileID uuid.UUID) (string, error)
	SetChatE2EEnabled(ctx context.Context, chatID uuid.UUID, enabled bool) error
}
