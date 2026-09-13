package chatmembers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
)

func TestNoopLister_FailsClosed(t *testing.T) {
	ids, err := NoopLister{}.ListMemberProfileIDs(context.Background(), "chat-1")
	require.Error(t, err)
	require.Empty(t, ids)
}

func TestListAllMemberPages_FetchesEveryPage(t *testing.T) {
	var cursors []string
	members, err := listAllMemberPages(context.Background(), "chat-1", func(_ context.Context, req *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
		cursors = append(cursors, req.GetPage().GetCursor())
		switch req.GetPage().GetCursor() {
		case "":
			return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{Members: []*chatv1.ChatMember{{ProfileId: "sender"}}, NextCursor: "sender"}}, nil
		case "sender":
			return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{Members: []*chatv1.ChatMember{{ProfileId: "recipient", IsArchived: true}}}}, nil
		default:
			t.Fatalf("unexpected cursor %q", req.GetPage().GetCursor())
			return nil, nil
		}
	})
	require.NoError(t, err)
	require.Equal(t, []string{"", "sender"}, cursors)
	require.Equal(t, []Member{{ProfileID: "sender"}, {ProfileID: "recipient", IsArchived: true}}, members)
}

func TestListAllMemberPages_RejectsEmptyContinuationPage(t *testing.T) {
	_, err := listAllMemberPages(context.Background(), "chat-1", func(_ context.Context, _ *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
		return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{NextCursor: "next"}}, nil
	})
	require.Error(t, err)
}

func TestListAllMemberPages_RejectsTerminalEmptyContinuationPage(t *testing.T) {
	var cursors []string
	_, err := listAllMemberPages(context.Background(), "chat-1", func(_ context.Context, req *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
		cursors = append(cursors, req.GetPage().GetCursor())
		switch req.GetPage().GetCursor() {
		case "":
			return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{
				Members:    []*chatv1.ChatMember{{ProfileId: "sender"}},
				NextCursor: "sender",
			}}, nil
		case "sender":
			return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{}}, nil
		default:
			t.Fatalf("unexpected cursor %q", req.GetPage().GetCursor())
			return nil, nil
		}
	})
	require.Error(t, err)
	require.Equal(t, []string{"", "sender"}, cursors)
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
