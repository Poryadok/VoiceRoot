package grpcsvc

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	"voice/backend/space/internal/spaceevents"
	"voice/backend/space/internal/store"

	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// SpaceGRPC implements SpaceService RPCs backed by space_db.
type SpaceGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Store             *store.SpaceStore
	SpaceEvents       spaceevents.Publisher // optional; CreateSpace publishes space.created
	Roles             rolev1.RoleServiceClient
	ProfileAccounts   ProfileAccountLookup // optional; resolves profile_id → account_id for bans
	Chats             ChatLookup           // optional; enriches text_chat nodes in ListSpaceTree
	Privacy           InvitePrivacyChecker
	Friends           InviteProfileFriendChecker
	SpaceCoMembership InviteSpaceCoMembershipChecker
	Blocks            JoinAccountBlockChecker
	MutationLocker    SpaceMutationLocker

	// skipJoinBlockDefaults disables permissive join-block stubs in integration tests.
	skipJoinBlockDefaults bool

	// Logger emits structured nats_publish errors when JetStream publish fails after a successful RPC.
	Logger *slog.Logger

	// Test hooks for subscription entitlement integration tests.
	SeedSpaceProActive bool

	// ownershipTransfers serializes the full ownership transition per space.
	// It deliberately does not serialize transfers for different spaces.
	ownershipTransfers ownershipTransferLocker
}

// SpaceMutationLocker coordinates mutations of one space across service
// instances. Production wiring supplies a PostgreSQL-backed implementation.
type SpaceMutationLocker interface {
	Acquire(context.Context, uuid.UUID) (func(), error)
}

type ownershipTransferLocker struct {
	mu    sync.Mutex
	locks map[string]*ownershipTransferLock
}

type ownershipTransferLock struct {
	semaphore chan struct{}
	refs      int
}
