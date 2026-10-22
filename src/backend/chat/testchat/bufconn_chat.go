// Package testchat hosts Chat gRPC over bufconn for cross-module tests (e.g. Messaging).
// It lives outside chat/internal so other modules under voice/backend may import it.
package testchat

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/chat/internal/chatevents"
	chatgrpc "voice/backend/chat/internal/grpcsvc"
	chatstore "voice/backend/chat/internal/store"
	"voice/backend/chat/testcreate"
	"voice/backend/pkg/privacy"

	chatv1 "voice.app/voice/chat/v1"
)

// ChatDeps configures ChatService collaborators (same as production ChatGRPC wiring).
// DeletedAccounts is mandatory so test fixtures cannot silently bypass Chat's
// fail-closed deleted-peer gate.
type ChatDeps struct {
	Profiles        chatgrpc.UserProfileLookup
	LifecycleOwners chatgrpc.LifecycleOwnerLookup
	Blocks          chatgrpc.AccountBlockChecker
	Privacy         chatgrpc.PrivacyChecker
	ListEnrich      chatgrpc.ListChatsEnrichment
	DeletedAccounts chatgrpc.AccountDeletedChecker
	ChatEvents      chatevents.Publisher
}

// AllowAllBlocks is an explicit Social decision for fixtures that need a DM
// without exercising the block-service failure path.
type AllowAllBlocks struct{}

func (AllowAllBlocks) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// AllowAllPrivacy is an explicit User decision for fixtures that need a DM
// without exercising recipient privacy.
type AllowAllPrivacy struct{}

func (AllowAllPrivacy) AllowDMAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

func (AllowAllPrivacy) AllowChatSpaceInvitesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

// NewBufconnChatClient returns a ChatService client backed by an in-process server using pool.
// Caller must apply chat_db migrations including 000011_deleted_for_self (ListMembers checks soft-delete).
func NewBufconnChatClient(t *testing.T, pool *pgxpool.Pool, deps ChatDeps) (chatv1.ChatServiceClient, func()) {
	t.Helper()
	return NewBufconnChatClientWith(t, pool, deps)
}

// NewBufconnChatClientWith wires collaborators and the signed CreateChat test transport.
func NewBufconnChatClientWith(t *testing.T, pool *pgxpool.Pool, deps ChatDeps) (chatv1.ChatServiceClient, func()) {
	t.Helper()
	require.NotNil(t, deps.DeletedAccounts, "testchat.ChatDeps.DeletedAccounts is required")
	require.NotNil(t, deps.Blocks, "testchat.ChatDeps.Blocks is required")
	require.NotNil(t, deps.Privacy, "testchat.ChatDeps.Privacy is required")
	return testcreate.NewClient(t, &chatgrpc.ChatGRPC{
		DM:              &chatstore.DMStore{Pool: pool},
		Profiles:        deps.Profiles,
		LifecycleOwners: deps.LifecycleOwners,
		Blocks:          deps.Blocks,
		Privacy:         deps.Privacy,
		ListEnrich:      deps.ListEnrich,
		DeletedAccounts: deps.DeletedAccounts,
		ChatEvents:      deps.ChatEvents,
	})
}
