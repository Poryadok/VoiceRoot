package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"voice/backend/messaging/internal/store"
)

type explicitChatAuthority struct {
	channelRoleAuthority
	allowed bool
	calls   int
}

func (a *explicitChatAuthority) CanPostAsChat(_ context.Context, chatID, profileID uuid.UUID) (bool, error) {
	a.chatID, a.profileID = chatID, profileID
	a.calls++
	return a.allowed, a.err
}

func TestOfficialMemberGrantRecheckedBeforeMainFeedAndThread(t *testing.T) {
	for _, kind := range []string{"channel", "group"} {
		for _, thread := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/feed", true: "/thread"}[thread], func(t *testing.T) {
				chatID, profileID, parentID := uuid.New(), uuid.New(), uuid.New()
				a := &explicitChatAuthority{channelRoleAuthority: channelRoleAuthority{role: "member"}, allowed: true}
				d := threadPolicyDeps{Members: a,
					Policy:   channelThreadPolicy{&store.ChatThreadPolicy{ChatType: kind, AllowUserMainFeed: true, ThreadsEnabled: true}},
					Messages: channelThreadMessages{&store.MessageRow{ID: parentID, ChatID: chatID}}}
				var parent *uuid.UUID
				if thread {
					parent = &parentID
				}
				require.NoError(t, d.validateSend(context.Background(), chatID, profileID, parent, true))
				require.Equal(t, 1, a.calls, "official identity requires one fresh decision")
				require.Equal(t, chatID, a.chatID)
				require.Equal(t, profileID, a.profileID)
				a.allowed = false
				require.Equal(t, codes.PermissionDenied, status.Code(d.validateSend(context.Background(), chatID, profileID, parent, true)))
				require.Equal(t, 2, a.calls, "revocation affects the next operation")
			})
		}
	}
}
