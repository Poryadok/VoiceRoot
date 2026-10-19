package reportread

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
)

type chatScopeCalls struct {
	scope      func(context.Context, *chatv1.GetSpaceChatScopeAuthorityRequest) (*chatv1.GetSpaceChatScopeAuthorityResponse, error)
	attachment func(context.Context, *chatv1.ReadSpaceReportAttachmentRequest) (*chatv1.ReadSpaceReportAttachmentResponse, error)
}

func (c chatScopeCalls) GetSpaceChatScopeAuthority(ctx context.Context, r *chatv1.GetSpaceChatScopeAuthorityRequest, _ ...grpc.CallOption) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	return c.scope(ctx, r)
}
func (c chatScopeCalls) ReadSpaceReportAttachment(ctx context.Context, r *chatv1.ReadSpaceReportAttachmentRequest, _ ...grpc.CallOption) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
	return c.attachment(ctx, r)
}

type chatPolicyCall func(context.Context, *chatv1.CheckMessageReadEntitlementRequest) (*chatv1.CheckMessageReadEntitlementResponse, error)

func (f chatPolicyCall) CheckMessageReadEntitlement(ctx context.Context, r *chatv1.CheckMessageReadEntitlementRequest, _ ...grpc.CallOption) (*chatv1.CheckMessageReadEntitlementResponse, error) {
	return f(ctx, r)
}

func TestReportOwnerChatScopeAndSeparateRetirementAuthority(t *testing.T) {
	o, verify := ownerFixture(t)
	space, chat := uuid.New(), uuid.New()
	event := uuid.NewString()
	mode := "live"
	o.ChatScope = chatScopeCalls{
		scope: func(ctx context.Context, r *chatv1.GetSpaceChatScopeAuthorityRequest) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
			verify(ctx, r, "chat", chatv1.ChatScopeAuthorityService_GetSpaceChatScopeAuthority_FullMethodName)
			if mode == "missing" {
				return nil, status.Error(codes.NotFound, "native absent")
			}
			v := &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, OperationId: r.OperationId, ChatId: r.ChatId, ExpectedSpaceId: r.ExpectedSpaceId, ActualSpaceId: r.ExpectedSpaceId, Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, ReportAttachmentGeneration: 4, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE}
			if mode == "foreign" {
				v.ActualSpaceId = uuid.NewString()
			}
			if mode == "frozen" {
				v.Live = false
				v.LifecycleState = chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_FROZEN
			}
			return v, nil
		},
		attachment: func(ctx context.Context, r *chatv1.ReadSpaceReportAttachmentRequest) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
			verify(ctx, r, "chat", chatv1.ChatScopeAuthorityService_ReadSpaceReportAttachment_FullMethodName)
			v := &chatv1.ReadSpaceReportAttachmentResponse{ProtocolVersion: 1, OperationId: r.OperationId, ChatId: r.ChatId, ExpectedSpaceId: r.ExpectedSpaceId, SavedAttachmentGeneration: r.SavedAttachmentGeneration, Disposition: chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED}
			if mode == "moved" {
				v.Disposition = chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED
				v.Reason = chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED
				v.OwnerEventId = event
				v.OwnerRevision = 6
			}
			if mode == "false proof" {
				v.OwnerEventId = event
			}
			return v, nil
		},
	}
	first, err := o.ReadChat(taintedContext(), space, chat)
	require.NoError(t, err)
	second, err := o.ReadChat(taintedContext(), space, chat)
	require.NoError(t, err)
	require.NotEqual(t, first.ChatScopeIncarnation, second.ChatScopeIncarnation)
	unchanged, err := o.ReadAttachment(taintedContext(), chat, space, 4)
	require.NoError(t, err)
	require.Equal(t, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED, unchanged.Disposition, "scope incarnation change is not a move proof")
	for _, m := range []string{"missing", "frozen", "foreign"} {
		mode = m
		_, err = o.ReadChat(taintedContext(), space, chat)
		if m == "foreign" {
			require.Equal(t, codes.Unavailable, status.Code(err))
		} else {
			require.ErrorIs(t, err, ErrNotVisible)
		}
	}
	mode = "moved"
	retired, err := o.ReadAttachment(taintedContext(), chat, space, 4)
	require.NoError(t, err)
	require.Equal(t, event, retired.OwnerEventId)
	require.EqualValues(t, 4, retired.SavedAttachmentGeneration)
	mode = "false proof"
	_, err = o.ReadAttachment(taintedContext(), chat, space, 4)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestReportOwnerManagedPolicySignedExactTimestamp(t *testing.T) {
	o, verify := ownerFixture(t)
	chat, viewer := uuid.New(), uuid.New()
	at := time.Now().UTC()
	allowed := true
	o.ChatPolicy = chatPolicyCall(func(ctx context.Context, r *chatv1.CheckMessageReadEntitlementRequest) (*chatv1.CheckMessageReadEntitlementResponse, error) {
		verify(ctx, r, "chat", chatv1.ChatService_CheckMessageReadEntitlement_FullMethodName)
		require.Equal(t, chat.String(), r.ChatId)
		require.Equal(t, viewer.String(), r.ProfileId)
		require.True(t, at.Equal(r.MessageCreatedAt.AsTime()))
		return &chatv1.CheckMessageReadEntitlementResponse{Entitled: allowed}, nil
	})
	got, err := o.ManagedEntitled(taintedContext(), chat, viewer, at)
	require.NoError(t, err)
	require.True(t, got)
	allowed = false
	got, err = o.ManagedEntitled(taintedContext(), chat, viewer, at)
	require.NoError(t, err)
	require.False(t, got)
}
