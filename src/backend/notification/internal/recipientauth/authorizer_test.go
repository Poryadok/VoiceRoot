package recipientauth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type chatFn func(context.Context, *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error)

func (f chatFn) GetChat(c context.Context, r *chatv1.GetChatRequest, _ ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	return f(c, r)
}

func TestAuthorizeSocialDenialAndUncertaintyFailClosed(t *testing.T) {
	r, s, c, ra, sa := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name                          string
		kind                          chatv1.ChatType
		reverse, missing, unavailable bool
		want                          bool
	}{
		{name: "recipient block", kind: chatv1.ChatType_CHAT_TYPE_GROUP},
		{name: "sender block does not hide shared chat", kind: chatv1.ChatType_CHAT_TYPE_GROUP, reverse: true, want: true},
		{name: "sender block denies DM", kind: chatv1.ChatType_CHAT_TYPE_DM, reverse: true},
		{name: "missing response", kind: chatv1.ChatType_CHAT_TYPE_GROUP, missing: true},
		{name: "outage", kind: chatv1.ChatType_CHAT_TYPE_GROUP, unavailable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Authorizer{Profiles: profiles{r: ra, s: sa}, Chat: chatFn(func(context.Context, *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
				return &chatv1.GetChatResponse{Chat: &chatv1.Chat{Id: c, Type: tc.kind}}, nil
			}), Blocks: blockFn(func(req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
				if tc.unavailable {
					return nil, errors.New("unavailable")
				}
				if tc.missing {
					return nil, nil
				}
				blocked := req.GetAccountIdA() == ra
				if tc.reverse {
					blocked = req.GetAccountIdA() == sa
				}
				return &socialv1.IsBlockedResponse{Blocked: blocked}, nil
			})}
			allowed, err := a.Authorize(context.Background(), r, s, c)
			if tc.missing || tc.unavailable {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, allowed)
		})
	}
}

type profiles map[string]string

func (p profiles) GetProfile(_ context.Context, r *userv1.GetProfileRequest, _ ...grpc.CallOption) (*userv1.GetProfileResponse, error) {
	return &userv1.GetProfileResponse{Profile: &userv1.Profile{Id: r.GetProfileId(), AccountId: p[r.GetProfileId()]}}, nil
}

type blockFn func(*socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error)

func (f blockFn) IsBlocked(_ context.Context, r *socialv1.IsBlockedRequest, _ ...grpc.CallOption) (*socialv1.IsBlockedResponse, error) {
	return f(r)
}

func TestAuthorizeCurrentACLAndDirectedBlocksWithoutAmbientPrivilege(t *testing.T) {
	r, s, c, ra, sa := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, kind := range []chatv1.ChatType{chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_DM} {
		t.Run(kind.String(), func(t *testing.T) {
			var checked [][2]string
			a := &Authorizer{Profiles: profiles{r: ra, s: sa}, Chat: chatFn(func(ctx context.Context, req *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Equal(t, metadata.Pairs("x-voice-user-id", ra, "x-voice-profile-id", r), md)
				return &chatv1.GetChatResponse{Chat: &chatv1.Chat{Id: c, Type: kind}}, nil
			}), Blocks: blockFn(func(req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
				checked = append(checked, [2]string{req.GetAccountIdA(), req.GetAccountIdB()})
				return &socialv1.IsBlockedResponse{}, nil
			})}
			ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "ambient", "x-voice-internal-caller", "admin", "x-voice-profile-id", s))
			allowed, err := a.Authorize(ctx, r, s, c)
			require.NoError(t, err)
			require.True(t, allowed)
			want := [][2]string{{ra, sa}}
			if kind == chatv1.ChatType_CHAT_TYPE_DM {
				want = append(want, [2]string{sa, ra})
			}
			require.Equal(t, want, checked)
			a.Chat = chatFn(func(context.Context, *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
				return nil, status.Error(codes.PermissionDenied, "removed")
			})
			allowed, err = a.Authorize(ctx, r, s, c)
			require.NoError(t, err)
			require.False(t, allowed)
		})
	}
}
