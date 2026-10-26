package catalogassets

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/chat/internal/store"
)

type destinationChatFixture struct {
	row             *store.ChatRow
	member, deleted bool
	reads           int
	flipE2E         bool
}

func (f *destinationChatFixture) FindChatByID(context.Context, uuid.UUID) (*store.ChatRow, error) {
	f.reads++
	if f.row == nil {
		return nil, nil
	}
	r := *f.row
	if f.flipE2E && f.reads > 1 {
		r.E2EEnabled = true
	}
	return &r, nil
}
func (f *destinationChatFixture) IsChatMember(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return f.member, nil
}
func (f *destinationChatFixture) IsMemberDeletedForSelf(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return f.deleted, nil
}

type destinationSpaceFixture bool

func (f destinationSpaceFixture) IsSpaceMember(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return bool(f), nil
}

type destinationRoleFixture struct {
	rolev1.RoleServiceClient
	allowed bool
	request *rolev1.CheckPermissionRequest
}

func (f *destinationRoleFixture) CheckPermission(_ context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	f.request = r
	return &rolev1.CheckPermissionResponse{Allowed: f.allowed}, nil
}

func TestNativeDestinationAuthority(t *testing.T) {
	actor := Actor{AccountID: uuid.New(), ProfileID: uuid.New()}
	id, space := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name                                          string
		kind                                          string
		space                                         bool
		member, deleted, e2e, flip, role, missingRole bool
		supplied                                      chatv1.ChatType
		want                                          codes.Code
	}{
		{name: "dm", kind: "dm", member: true},
		{name: "missing member", kind: "group", want: codes.PermissionDenied},
		{name: "hidden dm", kind: "dm", member: true, deleted: true, want: codes.PermissionDenied},
		{name: "encrypted", kind: "dm", member: true, e2e: true, want: codes.FailedPrecondition},
		{name: "changed after read", kind: "dm", member: true, flip: true, want: codes.FailedPrecondition},
		{name: "forged type", kind: "group", member: true, supplied: chatv1.ChatType_CHAT_TYPE_DM, want: codes.InvalidArgument},
		{name: "unknown native kind", kind: "unknown", member: true, want: codes.Unavailable},
		{name: "space member", kind: "channel", space: true, member: true, role: true},
		{name: "space not member", kind: "channel", space: true, want: codes.PermissionDenied},
		{name: "space hidden", kind: "group", space: true, member: true, want: codes.PermissionDenied},
		{name: "space no role transport", kind: "channel", space: true, member: true, missingRole: true, want: codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &destinationChatFixture{row: &store.ChatRow{ID: id, Type: tc.kind, E2EEnabled: tc.e2e}, member: tc.member, deleted: tc.deleted, flipE2E: tc.flip}
			if tc.space {
				f.row.SpaceID = &space
			}
			role := &destinationRoleFixture{allowed: tc.role}
			a := &NativeDestinationAuthority{Chats: f, Spaces: destinationSpaceFixture(tc.member), Roles: role}
			if tc.missingRole {
				a.Roles = nil
			}
			ref := &chatv1.ChatRef{Id: id.String()}
			if tc.supplied != 0 {
				ref.Type = &tc.supplied
			}
			observed, err := a.ObservePlaintextMember(context.Background(), actor, ref)
			require.Equal(t, tc.want, status.Code(err))
			if err == nil {
				require.NotEqual(t, chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, observed)
				require.Equal(t, 2, f.reads)
			}
			if tc.space && tc.member && !tc.missingRole {
				require.Equal(t, actor.ProfileID.String(), role.request.GetProfileId())
				require.Equal(t, "TEXT_CHAT_VIEW", role.request.GetPermissionName())
				require.Equal(t, id.String(), role.request.GetChat().GetId())
			}
		})
	}
	a := &NativeDestinationAuthority{Chats: &destinationChatFixture{}}
	_, err := a.ObservePlaintextMember(context.Background(), actor, &chatv1.ChatRef{Id: id.String()})
	require.Equal(t, codes.NotFound, status.Code(err))
}
