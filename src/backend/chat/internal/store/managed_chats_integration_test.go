package store

import (
	"context"
	"testing"
	"time"

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
	require.NotEqual(t, uuid.Nil, first.ChatID)
	var creatorID *uuid.UUID
	var managedApp uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT creator_profile_id, managed_by_application_id FROM chats WHERE id=$1`, first.ChatID).Scan(&creatorID, &managedApp))
	require.Nil(t, creatorID)
	require.Equal(t, applicationID, managedApp)
	require.False(t, first.Replayed)
	require.Equal(t, 0, countMembers(t, chatStore, first.ChatID))

	replayed, err := chatStore.ProvisionManagedChat(ctx, request)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.ChatID, replayed.ChatID)
	_, err = pool.Exec(ctx, `UPDATE chats SET updated_at=updated_at + interval '1 minute', last_message_at=now() WHERE id=$1`, first.ChatID)
	require.NoError(t, err)
	secondReplay, err := chatStore.ProvisionManagedChat(ctx, request)
	require.NoError(t, err)
	require.True(t, secondReplay.Replayed)
	require.Equal(t, first.ChatID, secondReplay.ChatID)

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
		ChatID:        created.ChatID,
		RequestHash:   "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ProfileIDs:    []uuid.UUID{firstMember, secondMember},
	}

	first, err := chatStore.SyncManagedChatMembers(ctx, request)
	require.NoError(t, err)
	require.False(t, first.Replayed)
	require.ElementsMatch(t, []uuid.UUID{firstMember, secondMember}, first.ProfileIDs)
	require.Equal(t, "member", memberRole(t, chatStore, created.ChatID, firstMember))
	require.Equal(t, "member", memberRole(t, chatStore, created.ChatID, secondMember))
	page, err := chatStore.ListChatsPage(ctx, firstMember, "", 20, "main", nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
	require.Equal(t, uuid.Nil, page.Rows[0].CreatorProfileID)

	replayed, err := chatStore.SyncManagedChatMembers(ctx, request)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.ProfileIDs, replayed.ProfileIDs)

	joinedAt := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	mutedUntil := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Microsecond)
	_, err = pool.Exec(ctx, `UPDATE chat_members SET joined_at=$3, muted_until=$4, is_archived=true, inbox_bucket='requests' WHERE chat_id=$1 AND profile_id=$2`, created.ChatID, firstMember, joinedAt, mutedUntil)
	require.NoError(t, err)
	_, err = chatStore.SyncManagedChatMembers(ctx, ManagedChatMemberSync{
		ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: uuid.New(),
		ChatID: created.ChatID, RequestHash: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		ProfileIDs: []uuid.UUID{firstMember, secondMember},
	})
	require.NoError(t, err)
	var gotJoinedAt, gotMutedUntil time.Time
	var gotArchived bool
	var gotInboxBucket string
	err = pool.QueryRow(ctx, `SELECT joined_at, muted_until, is_archived, inbox_bucket FROM chat_members WHERE chat_id=$1 AND profile_id=$2`, created.ChatID, firstMember).
		Scan(&gotJoinedAt, &gotMutedUntil, &gotArchived, &gotInboxBucket)
	require.NoError(t, err)
	require.True(t, joinedAt.Equal(gotJoinedAt))
	require.True(t, mutedUntil.Equal(gotMutedUntil))
	require.True(t, gotArchived)
	require.Equal(t, "requests", gotInboxBucket)

	changed := request
	changed.RequestHash = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	changed.ProfileIDs = []uuid.UUID{thirdMember}
	_, err = chatStore.SyncManagedChatMembers(ctx, changed)
	require.ErrorIs(t, err, ErrManagedOperationConflict)

	_, err = chatStore.SyncManagedChatMembers(ctx, ManagedChatMemberSync{
		ApplicationID: uuid.New(), EnvironmentID: environmentID, OperationID: uuid.New(),
		ChatID: created.ChatID, RequestHash: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
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
		ChatID: created.ChatID, RequestHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ProfileIDs: []uuid.UUID{member},
	})
	require.NoError(t, err)

	_, err = chatStore.AddGroupMembers(ctx, created.ChatID, []uuid.UUID{other})
	require.ErrorIs(t, err, ErrManagedChatPrincipalRequired)
	require.ErrorIs(t, chatStore.RemoveStandaloneGroupMember(ctx, created.ChatID, member, other), ErrManagedChatPrincipalRequired)
	require.ErrorIs(t, chatStore.LeaveGroupChat(ctx, created.ChatID, member), ErrManagedChatPrincipalRequired)
	require.ErrorIs(t, chatStore.RemoveGroupMember(ctx, created.ChatID, member), ErrManagedChatPrincipalRequired)
	require.Equal(t, 1, countMembers(t, chatStore, created.ChatID))
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
