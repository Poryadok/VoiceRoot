package grpcsvc

import (
	"log/slog"

	"voice/backend/space/internal/spaceevents"
	"voice/backend/space/internal/store"

	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// SpaceGRPC implements SpaceService RPCs backed by space_db.
type SpaceGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Store           *store.SpaceStore
	SpaceEvents     spaceevents.Publisher // optional; CreateSpace publishes space.created
	Roles           rolev1.RoleServiceClient
	ProfileAccounts ProfileAccountLookup // optional; resolves profile_id → account_id for bans
	Chats           ChatLookup           // optional; enriches text_chat nodes in ListSpaceTree
	Privacy         InvitePrivacyChecker
	Friends         InviteProfileFriendChecker
	SpaceCoMembership InviteSpaceCoMembershipChecker

	// Logger emits structured nats_publish errors when JetStream publish fails after a successful RPC.
	Logger *slog.Logger

	// Test hooks for subscription entitlement integration tests.
	SeedSpaceProActive bool
}
