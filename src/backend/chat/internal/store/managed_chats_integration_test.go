package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProvisionManagedChatIsOwnerlessAndIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	chatStore := &DMStore{Pool: pool}
	applicationID, environmentID := uuid.New(), uuid.New()
	request := ManagedChatCreate{
		ApplicationID: applicationID,
		EnvironmentID: environmentID,
		OperationID:   uuid.New(),
		ExternalKey:   "party:season-3:squad-8",
		RequestHash:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:          "Season 3 squad 8",
	}

	first, err := chatStore.ProvisionManagedChat(ctx, request)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, first.Chat.ID)
	require.Equal(t, uuid.Nil, first.Chat.CreatorProfileID)
	require.Equal(t, &applicationID, first.Chat.ManagedByApplicationID)
	require.False(t, first.Replayed)
	require.Equal(t, 0, countMembers(t, chatStore, first.Chat.ID))

	replayed, err := chatStore.ProvisionManagedChat(ctx, request)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.Chat.ID, replayed.Chat.ID)

	conflict := request
	conflict.RequestHash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	_, err = chatStore.ProvisionManagedChat(ctx, conflict)
	require.ErrorIs(t, err, ErrManagedOperationConflict)
}

func TestSyncManagedChatMembersReplacesRosterWithMemberRolesAndReplaysReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	chatStore := &DMStore{Pool: pool}
	applicationID, environmentID := uuid.New(), uuid.New()
	created, err := chatStore.ProvisionManagedChat(ctx, ManagedChatCreate{
		ApplicationID: applicationID,
		EnvironmentID: environmentID,
		OperationID:   uuid.New(),
		ExternalKey:   "match:match-42",
		RequestHash:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:          "Match 42",
	})
	require.NoError(t, err)
	firstMember, secondMember, thirdMember := uuid.New(), uuid.New(), uuid.New()
	request := ManagedChatMemberSync{
		ApplicationID: applicationID,
		EnvironmentID: environmentID,
		OperationID:   uuid.New(),
		ChatID:        created.Chat.ID,
		RequestHash:   "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ProfileIDs:    []uuid.UUID{firstMember, secondMember},
	}

	first, err := chatStore.SyncManagedChatMembers(ctx, request)
	require.NoError(t, err)
	require.False(t, first.Replayed)
	require.ElementsMatch(t, []uuid.UUID{firstMember, secondMember}, first.ProfileIDs)
	require.Equal(t, "member", memberRole(t, chatStore, created.Chat.ID, firstMember))
	require.Equal(t, "member", memberRole(t, chatStore, created.Chat.ID, secondMember))
	page, err := chatStore.ListChatsPage(ctx, firstMember, "", 20, "main", nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
	require.Equal(t, uuid.Nil, page.Rows[0].CreatorProfileID)

	replayed, err := chatStore.SyncManagedChatMembers(ctx, request)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.ProfileIDs, replayed.ProfileIDs)

	changed := request
	changed.RequestHash = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	changed.ProfileIDs = []uuid.UUID{thirdMember}
	_, err = chatStore.SyncManagedChatMembers(ctx, changed)
	require.ErrorIs(t, err, ErrManagedOperationConflict)

	_, err = chatStore.SyncManagedChatMembers(ctx, ManagedChatMemberSync{
		ApplicationID: uuid.New(), EnvironmentID: environmentID, OperationID: uuid.New(),
		ChatID: created.Chat.ID, RequestHash: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		ProfileIDs: []uuid.UUID{thirdMember},
	})
	require.ErrorIs(t, err, ErrManagedChatNotFound)
}

func TestPlayerRosterMutationsCannotChangeManagedChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	chatStore := &DMStore{Pool: pool}
	applicationID, environmentID, operationID := uuid.New(), uuid.New(), uuid.New()
	created, err := chatStore.ProvisionManagedChat(ctx, ManagedChatCreate{
		ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: uuid.New(),
		ExternalKey: "party:roster-guard", RequestHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "Roster guard",
	})
	require.NoError(t, err)
	member, other := uuid.New(), uuid.New()
	_, err = chatStore.SyncManagedChatMembers(ctx, ManagedChatMemberSync{
		ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: operationID,
		ChatID: created.Chat.ID, RequestHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ProfileIDs: []uuid.UUID{member},
	})
	require.NoError(t, err)

	_, err = chatStore.AddGroupMembers(ctx, created.Chat.ID, []uuid.UUID{other})
	require.ErrorIs(t, err, ErrManagedChatPrincipalRequired)
	require.ErrorIs(t, chatStore.RemoveStandaloneGroupMember(ctx, created.Chat.ID, member, other), ErrManagedChatPrincipalRequired)
	require.ErrorIs(t, chatStore.LeaveGroupChat(ctx, created.Chat.ID, member), ErrManagedChatPrincipalRequired)
	require.ErrorIs(t, chatStore.RemoveGroupMember(ctx, created.Chat.ID, member), ErrManagedChatPrincipalRequired)
	require.Equal(t, 1, countMembers(t, chatStore, created.Chat.ID))
}

func countMembers(t *testing.T, chatStore *DMStore, chatID uuid.UUID) int {
	t.Helper()
	count, err := chatStore.CountChatMembers(context.Background(), chatID)
	require.NoError(t, err)
	return count
}

func memberRole(t *testing.T, chatStore *DMStore, chatID, profileID uuid.UUID) string {
	t.Helper()
	role, err := chatStore.GetMemberRole(context.Background(), chatID, profileID)
	require.NoError(t, err)
	return role
}
