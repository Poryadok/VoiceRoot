package chatmembers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
)

func TestNoopLister_ReturnsEmpty(t *testing.T) {
	ids, err := NoopLister{}.ListMemberProfileIDs(context.Background(), "chat-1")
	require.NoError(t, err)
	require.Empty(t, ids)
}

func TestMembersFromList_PreservesArchivedState(t *testing.T) {
	members := membersFromList(&chatv1.MemberList{Members: []*chatv1.ChatMember{{
		ProfileId:   "recipient-1",
		InboxBucket: stringPtr("main"),
		IsArchived:  true,
	}}})

	require.Equal(t, []Member{{
		ProfileID:   "recipient-1",
		InboxBucket: "main",
		IsArchived:  true,
	}}, members)
}

func stringPtr(value string) *string { return &value }
