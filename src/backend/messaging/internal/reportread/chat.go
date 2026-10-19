package reportread

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	"time"
	chatv1 "voice.app/voice/chat/v1"
)

type ChatScopeClient interface {
	GetSpaceChatScopeAuthority(context.Context, *chatv1.GetSpaceChatScopeAuthorityRequest, ...grpc.CallOption) (*chatv1.GetSpaceChatScopeAuthorityResponse, error)
	ReadSpaceReportAttachment(context.Context, *chatv1.ReadSpaceReportAttachmentRequest, ...grpc.CallOption) (*chatv1.ReadSpaceReportAttachmentResponse, error)
}
type ChatPolicyClient interface {
	CheckMessageReadEntitlement(context.Context, *chatv1.CheckMessageReadEntitlementRequest, ...grpc.CallOption) (*chatv1.CheckMessageReadEntitlementResponse, error)
}

// ErrNotVisible is only a viewer decision. It must never trigger retirement,
// cleanup or an ACK; those require the separately typed positive observation.
var ErrNotVisible = errors.New("report target is not visible")

func (o *Owners) ReadChat(ctx context.Context, space, chat uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	if o == nil || o.ChatScope == nil {
		return nil, unavailable()
	}
	if space == uuid.Nil || chat == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid report chat scope")
	}
	r := &chatv1.GetSpaceChatScopeAuthorityRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ChatId: chat.String(), ExpectedSpaceId: space.String()}
	call, cancel, err := o.signed(ctx, "chat", chatv1.ChatScopeAuthorityService_GetSpaceChatScopeAuthority_FullMethodName, r.OperationId, r)
	if err != nil {
		return nil, err
	}
	defer cancel()
	v, err := o.ChatScope.GetSpaceChatScopeAuthority(call, r)
	if status.Code(err) == codes.NotFound {
		return nil, ErrNotVisible
	}
	if err != nil || v == nil || unknown(v) || v.ProtocolVersion != 1 || v.OperationId != r.OperationId || v.ChatId != r.ChatId || v.ExpectedSpaceId != r.ExpectedSpaceId || v.ActualSpaceId != r.ExpectedSpaceId || !v.Exists || !canonicalID(v.ChatScopeIncarnation) || v.ChatAuthorityRevision == 0 || v.ChatAuthorityRevision > math.MaxInt64 || v.LifecycleGeneration > math.MaxInt64 || v.ReportAttachmentGeneration == 0 || v.ReportAttachmentGeneration > math.MaxInt64 || (v.ChatType != chatv1.ChatType_CHAT_TYPE_GROUP && v.ChatType != chatv1.ChatType_CHAT_TYPE_CHANNEL) {
		return nil, unavailable()
	}
	if v.ManagedKind < chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY || v.ManagedKind > chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD || v.LifecycleState < chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE || v.LifecycleState > chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_PURGED {
		return nil, unavailable()
	}
	ordinaryLive := v.ManagedKind == chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY && v.LifecycleState == chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE
	if v.Live != ordinaryLive {
		return nil, unavailable()
	}
	if !v.Live {
		return nil, ErrNotVisible
	}
	return proto.Clone(v).(*chatv1.GetSpaceChatScopeAuthorityResponse), nil
}

func (o *Owners) ManagedEntitled(ctx context.Context, chat, viewer uuid.UUID, created time.Time) (bool, error) {
	if o == nil || o.ChatPolicy == nil {
		return false, unavailable()
	}
	r := &chatv1.CheckMessageReadEntitlementRequest{ChatId: chat.String(), ProfileId: viewer.String(), MessageCreatedAt: timestamppb.New(created)}
	if chat == uuid.Nil || viewer == uuid.Nil || r.MessageCreatedAt.CheckValid() != nil {
		return false, status.Error(codes.InvalidArgument, "invalid report message scope")
	}
	call, cancel, err := o.signed(ctx, "chat", chatv1.ChatService_CheckMessageReadEntitlement_FullMethodName, uuid.NewString(), r)
	if err != nil {
		return false, err
	}
	defer cancel()
	v, err := o.ChatPolicy.CheckMessageReadEntitlement(call, r)
	if err != nil || v == nil || unknown(v) {
		return false, unavailable()
	}
	return v.Entitled, nil
}

// ReadAttachment never uses ReadChat/permission/absence as a retirement proof.
// Its exact old attachment remains observable after an A→B→A move.
func (o *Owners) ReadAttachment(ctx context.Context, chat, originalSpace uuid.UUID, generation uint64) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
	if o == nil || o.ChatScope == nil {
		return nil, unavailable()
	}
	if chat == uuid.Nil || originalSpace == uuid.Nil || generation == 0 || generation > math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "invalid report attachment binding")
	}
	r := &chatv1.ReadSpaceReportAttachmentRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ChatId: chat.String(), ExpectedSpaceId: originalSpace.String(), SavedAttachmentGeneration: generation}
	call, cancel, err := o.signed(ctx, "chat", chatv1.ChatScopeAuthorityService_ReadSpaceReportAttachment_FullMethodName, r.OperationId, r)
	if err != nil {
		return nil, err
	}
	defer cancel()
	v, err := o.ChatScope.ReadSpaceReportAttachment(call, r)
	if err != nil || v == nil || unknown(v) || v.ProtocolVersion != 1 || v.OperationId != r.OperationId || v.ChatId != r.ChatId || v.ExpectedSpaceId != r.ExpectedSpaceId || v.SavedAttachmentGeneration != r.SavedAttachmentGeneration {
		return nil, unavailable()
	}
	switch v.Disposition {
	case chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED:
		if v.Reason != 0 || v.OwnerEventId != "" || v.OwnerRevision != 0 {
			return nil, unavailable()
		}
	case chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED:
		if !canonicalID(v.OwnerEventId) || v.OwnerRevision == 0 || v.OwnerRevision > math.MaxInt64 || (v.Reason != chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED && v.Reason != chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED) {
			return nil, unavailable()
		}
	default:
		return nil, unavailable()
	}
	return proto.Clone(v).(*chatv1.ReadSpaceReportAttachmentResponse), nil
}
