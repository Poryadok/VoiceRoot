package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"
	"voice/backend/role/permissions"
)

func TestCheckReactionPermission(t *testing.T) {
	spaceID, profileID, chatID := uuid.New(), uuid.New(), uuid.New()
	policy := &store.ChatThreadPolicy{ChatType: "group", SpaceID: &spaceID}
	for _, tc := range []struct {
		name string
		role *reactionRoleAuthority
		want codes.Code
	}{
		{"denied", &reactionRoleAuthority{}, codes.PermissionDenied},
		{"allowed", &reactionRoleAuthority{allowed: true}, codes.OK},
		{"missing", nil, codes.Unavailable},
		{"typed nil", (*reactionRoleAuthority)(nil), codes.Unavailable},
		{"unavailable", &reactionRoleAuthority{err: errors.New("private detail")}, codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &MessagingGRPC{}
			if tc.name == "typed nil" || tc.role != nil {
				service.RolePermissions = tc.role
			}
			calls := 0
			if tc.role != nil {
				tc.role.check = func(gotSpace, gotProfile, gotChat uuid.UUID, permission string) {
					calls++
					require.Equal(t, spaceID, gotSpace)
					require.Equal(t, profileID, gotProfile)
					require.Equal(t, chatID, gotChat)
					require.Equal(t, permissions.TextChatAddReactions, permission)
				}
			}
			err := service.checkReactionPermission(context.Background(), policy, chatID, profileID)
			require.Equal(t, tc.want, status.Code(err))
			if err != nil {
				require.NotContains(t, err.Error(), "private detail")
			}
			if tc.role != nil {
				require.Equal(t, 1, calls)
			}
		})
	}
	for _, kind := range []string{"dm", "group", "channel"} {
		t.Run("standalone_"+kind, func(t *testing.T) {
			service := &MessagingGRPC{RolePermissions: &reactionRoleAuthority{check: func(uuid.UUID, uuid.UUID, uuid.UUID, string) {
				t.Fatal("standalone chat must not call Space Role authority")
			}}}
			require.NoError(t, service.checkReactionPermission(context.Background(), &store.ChatThreadPolicy{ChatType: kind}, chatID, profileID))
		})
	}
}
