package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"
)

// Official publication needs a current Chat-owned role; the marker is not a grant.
func TestStandaloneChannelOfficialWithoutAuthorityDenied(t *testing.T) {
	d := threadPolicyDeps{}
	for _, allowUser := range []bool{false, true} {
		err := d.validateMainFeedSend(context.Background(), uuid.New(), uuid.New(), &store.ChatThreadPolicy{
			ChatType: "channel", AllowUserMainFeed: allowUser,
		}, true)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
}

type channelRoleAuthority struct {
	ChatGuard
	role              string
	err               error
	chatID, profileID uuid.UUID
}

func (a *channelRoleAuthority) MemberRole(_ context.Context, chatID, profileID uuid.UUID) (string, error) {
	a.chatID, a.profileID = chatID, profileID
	return a.role, a.err
}

func TestStandaloneChannelPublicationMatrix(t *testing.T) {
	for _, role := range []string{"owner", "admin", "member", "", "unknown"} {
		for _, allowUser := range []bool{false, true} {
			for _, official := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/user=%t/official=%t", role, allowUser, official), func(t *testing.T) {
					chatID, profileID := uuid.New(), uuid.New()
					authority := &channelRoleAuthority{role: role}
					d := (&MessagingGRPC{ChatGuard: authority}).threadPolicyDeps()
					err := d.validateMainFeedSend(context.Background(), chatID, profileID, &store.ChatThreadPolicy{
						ChatType: "channel", AllowUserMainFeed: allowUser,
					}, official)
					known := role == "owner" || role == "admin" || role == "member"
					allowed := known && ((!official && allowUser) || (official && role != "member"))
					if allowed {
						require.NoError(t, err)
					} else {
						require.Equal(t, codes.PermissionDenied, status.Code(err))
					}
					require.Equal(t, chatID, authority.chatID)
					require.Equal(t, profileID, authority.profileID)
				})
			}
		}
	}
}

func TestStandaloneChannelRoleFailureClosed(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want codes.Code
	}{
		{store.ErrNotChatMember, codes.PermissionDenied},
		{status.Error(codes.Unavailable, "offline"), codes.Unavailable},
		{errors.New("lookup failed"), codes.Unavailable},
	} {
		authority := &channelRoleAuthority{role: "owner", err: tc.err}
		for _, official := range []bool{false, true} {
			err := (threadPolicyDeps{Members: authority}).validateMainFeedSend(context.Background(), uuid.New(), uuid.New(), &store.ChatThreadPolicy{
				ChatType: "channel", AllowUserMainFeed: true,
			}, official)
			require.Equal(t, tc.want, status.Code(err))
		}
	}
}

func TestSpaceSendMissingRoleAuthorityDenied(t *testing.T) {
	spaceID := uuid.New()
	err := (threadPolicyDeps{}).validateMainFeedSend(context.Background(), uuid.New(), uuid.New(), &store.ChatThreadPolicy{
		ChatType: "channel", SpaceID: &spaceID,
	}, true)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

type channelSpaceAuthority struct {
	allowed                    bool
	err                        error
	spaceID, profileID, chatID uuid.UUID
	permission                 string
}

func (a *channelSpaceAuthority) HasChatPermission(_ context.Context, spaceID, profileID, chatID uuid.UUID, permission string) (bool, error) {
	a.spaceID, a.profileID, a.chatID, a.permission = spaceID, profileID, chatID, permission
	return a.allowed, a.err
}

func TestSpaceChannelLocalOwnerCannotReplaceRoleAuthority(t *testing.T) {
	spaceID, chatID, profileID := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		allowed bool
		err     error
		want    codes.Code
	}{
		{true, nil, codes.OK},
		{false, nil, codes.PermissionDenied},
		{true, errors.New("unknown authority"), codes.Unavailable},
	} {
		role := &channelSpaceAuthority{allowed: tc.allowed, err: tc.err}
		members := &channelRoleAuthority{role: "owner"}
		err := (threadPolicyDeps{RolePerms: role, Members: members}).validateMainFeedSend(context.Background(), chatID, profileID, &store.ChatThreadPolicy{
			ChatType: "channel", SpaceID: &spaceID, AllowUserMainFeed: true,
		}, true)
		require.Equal(t, tc.want, status.Code(err))
		require.Equal(t, spaceID, role.spaceID)
		require.Equal(t, chatID, role.chatID)
		require.Equal(t, profileID, role.profileID)
		require.Equal(t, "TEXT_CHAT_SEND_MESSAGES", role.permission)
		require.Equal(t, uuid.Nil, members.chatID, "Space authorization must not substitute local owner/admin")
	}
}

func TestChannelMissingPolicyFailsClosed(t *testing.T) {
	s := &MessagingGRPC{}
	err := s.checkSpaceSendPermission(context.Background(), uuid.New(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(err))
	err = s.threadPolicyDeps().validateMainFeedSend(context.Background(), uuid.New(), uuid.New(), nil, true)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

type channelThreadPolicy struct{ policy *store.ChatThreadPolicy }

func (p channelThreadPolicy) Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error) {
	return p.policy, nil
}

type channelThreadMessages struct{ parent *store.MessageRow }

func (m channelThreadMessages) GetMessageByID(context.Context, uuid.UUID) (*store.MessageRow, error) {
	return m.parent, nil
}

func (channelThreadMessages) ThreadHasReplies(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// Exercise validateSend's routing branch, rather than only its main-feed helper.
func TestStandaloneChannelThreadOfficialIdentity(t *testing.T) {
	chatID, profileID, parentID := uuid.New(), uuid.New(), uuid.New()
	for _, allowUser := range []bool{false, true} {
		for _, role := range []string{"owner", "admin", "member", "unknown", ""} {
			for _, official := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/user=%t/official=%t", role, allowUser, official), func(t *testing.T) {
					d := threadPolicyDeps{
						Policy:   channelThreadPolicy{&store.ChatThreadPolicy{ChatType: "channel", AllowUserMainFeed: allowUser}},
						Messages: channelThreadMessages{&store.MessageRow{ID: parentID, ChatID: chatID}},
						Members:  &channelRoleAuthority{role: role},
					}
					err := d.validateSend(context.Background(), chatID, profileID, &parentID, official)
					if official && role != "owner" && role != "admin" {
						require.Equal(t, codes.PermissionDenied, status.Code(err))
					} else {
						require.NoError(t, err, "self-identity thread replies do not require the main-feed setting")
					}
				})
			}
		}
	}
}
