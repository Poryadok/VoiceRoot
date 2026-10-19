// Package reportread implements the owner checks for Space-local reports.
// Owner observations are positioning facts; none is a reusable access grant.
package reportread

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	socialv1 "voice.app/voice/social/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type SpaceClient interface {
	ResolveSpaceReportReadContext(context.Context, *spacev1.ResolveSpaceReportReadContextRequest, ...grpc.CallOption) (*spacev1.ResolveSpaceReportReadContextResponse, error)
}
type RoleClient interface {
	CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error)
}
type SocialClient interface {
	IsProfilePairsBlocked(context.Context, *socialv1.IsProfilePairsBlockedRequest, ...grpc.CallOption) (*socialv1.IsProfilePairsBlockedResponse, error)
}

// Owners uses the actual Messaging issuer. It never forwards the inbound bearer
// or profile headers. Each downstream request receives its own UUID and JTI.
type Owners struct {
	Issuer     *principal.Issuer
	Space      SpaceClient
	Role       RoleClient
	Social     SocialClient
	ChatScope  ChatScopeClient
	ChatPolicy ChatPolicyClient
	Clock      func() time.Time
}

type SpaceObservation struct {
	Facts *spacev1.ResolveSpaceReportReadContextResponse
	// Deadline starts BEFORE the RPC; network transit cannot extend the lease.
	Deadline time.Time
}

func (o *Owners) now() time.Time {
	if o != nil && o.Clock != nil {
		return o.Clock()
	}
	return time.Now()
}
func unavailable() error {
	return status.Error(codes.Unavailable, "report owner observation unavailable")
}
func canonicalID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}
func validChat(c *chatv1.ChatRef) bool {
	return c != nil && !unknown(c) && canonicalID(c.Id) && c.Type != nil && (c.GetType() == chatv1.ChatType_CHAT_TYPE_GROUP || c.GetType() == chatv1.ChatType_CHAT_TYPE_CHANNEL)
}
func unknown(m proto.Message) bool {
	if m == nil || !m.ProtoReflect().IsValid() {
		return true
	}
	var visit func(protoreflect.Message) bool
	visit = func(v protoreflect.Message) bool {
		if len(v.GetUnknown()) != 0 {
			return true
		}
		bad := false
		v.Range(func(f protoreflect.FieldDescriptor, x protoreflect.Value) bool {
			if f.IsMap() && f.MapValue().Kind() == protoreflect.MessageKind {
				x.Map().Range(func(_ protoreflect.MapKey, z protoreflect.Value) bool { bad = visit(z.Message()); return !bad })
			} else if f.IsList() && f.Kind() == protoreflect.MessageKind {
				for i := 0; i < x.List().Len() && !bad; i++ {
					bad = visit(x.List().Get(i).Message())
				}
			} else if f.Kind() == protoreflect.MessageKind {
				bad = visit(x.Message())
			}
			return !bad
		})
		return bad
	}
	return visit(m.ProtoReflect())
}

func (o *Owners) signed(ctx context.Context, audience, method, operation string, request proto.Message) (context.Context, context.CancelFunc, error) {
	if o == nil || o.Issuer == nil {
		return nil, nil, unavailable()
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, nil, unavailable()
	}
	token, err := o.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: operation, RequestHash: hash})
	if err != nil {
		return nil, nil, unavailable()
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation)), cancel, nil
}

func validBinding(b *rolev1.MembershipBinding) bool {
	if b == nil || unknown(b) {
		return false
	}
	if v := b.GetOrdinary(); v != nil {
		return v.MembershipGeneration <= math.MaxInt64 && v.Legacy == (v.MembershipGeneration == 0) && canonicalID(v.PublicationIdentity)
	}
	if v := b.GetCommunity(); v != nil {
		return v.OwnerGeneration > 0 && v.OwnerGeneration <= math.MaxInt64 && v.MembershipGeneration > 0 && v.MembershipGeneration <= math.MaxInt64 && canonicalID(v.MembershipEpisodeId)
	}
	return false
}

func validCounter(value uint64) bool { return value > 0 && value <= math.MaxInt64 }

func (o *Owners) ReadSpace(ctx context.Context, space, viewer uuid.UUID, chat *chatv1.ChatRef) (*SpaceObservation, error) {
	if o == nil || o.Space == nil {
		return nil, unavailable()
	}
	if space == uuid.Nil || viewer == uuid.Nil || !validChat(chat) {
		return nil, status.Error(codes.InvalidArgument, "invalid report owner scope")
	}
	req := &spacev1.ResolveSpaceReportReadContextRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SpaceId: space.String(), ViewerProfileId: viewer.String(), Chat: proto.Clone(chat).(*chatv1.ChatRef)}
	call, cancel, err := o.signed(ctx, "space", spacev1.SpaceService_ResolveSpaceReportReadContext_FullMethodName, req.OperationId, req)
	if err != nil {
		return nil, err
	}
	defer cancel()
	start := o.now()
	r, err := o.Space.ResolveSpaceReportReadContext(call, req)
	if err != nil || r == nil || unknown(r) || r.ProtocolVersion != 1 || r.OperationId != req.OperationId || r.SpaceId != req.SpaceId || r.ViewerProfileId != req.ViewerProfileId || !proto.Equal(r.Chat, req.Chat) {
		return nil, unavailable()
	}
	if !r.Eligible {
		// A denied reply contains only echoes, even if a compromised owner sends
		// apparently useful internal identifiers alongside the denial.
		expected := &spacev1.ResolveSpaceReportReadContextResponse{ProtocolVersion: 1, OperationId: req.OperationId, SpaceId: req.SpaceId, ViewerProfileId: req.ViewerProfileId, Chat: req.Chat}
		if !proto.Equal(r, expected) {
			return nil, unavailable()
		}
		return &SpaceObservation{Facts: proto.Clone(r).(*spacev1.ResolveSpaceReportReadContextResponse)}, nil
	}
	if !validCounter(r.SpaceAuthorityRevision) || !validCounter(r.LifecycleGeneration) || !canonicalID(r.TreeAttachmentIncarnation) || !validBinding(r.MembershipBinding) || !canonicalID(r.EffectiveEpisodeId) || !validCounter(r.EffectiveEpisodeGeneration) || !validCounter(r.LedgerRevision) || r.EvaluatedAt == nil || r.EvaluatedAt.CheckValid() != nil || r.ValidityRemainingMs == 0 || r.ValidityRemainingMs > 5000 || r.HistoryKnown != (r.HistoryVisibleSince != nil) {
		return nil, unavailable()
	}
	if r.HistoryKnown && (r.HistoryVisibleSince.CheckValid() != nil || r.HistoryVisibleSince.AsTime().After(r.EvaluatedAt.AsTime())) {
		return nil, unavailable()
	}
	deadline := start.Add(time.Duration(r.ValidityRemainingMs) * time.Millisecond)
	if !o.now().Before(deadline) {
		return nil, unavailable()
	}
	return &SpaceObservation{Facts: proto.Clone(r).(*spacev1.ResolveSpaceReportReadContextResponse), Deadline: deadline}, nil
}

func (o *Owners) Permission(ctx context.Context, space, viewer uuid.UUID, chat *chatv1.ChatRef, permission string) (bool, error) {
	if o == nil || o.Role == nil {
		return false, unavailable()
	}
	if space == uuid.Nil || viewer == uuid.Nil || !validChat(chat) || (permission != "TEXT_CHAT_VIEW" && permission != "TEXT_CHAT_READ_HISTORY") {
		return false, status.Error(codes.InvalidArgument, "invalid report permission scope")
	}
	req := &rolev1.CheckPermissionRequest{SpaceId: space.String(), ProfileId: viewer.String(), Chat: proto.Clone(chat).(*chatv1.ChatRef), PermissionName: permission}
	call, cancel, err := o.signed(ctx, "role", rolev1.RoleService_CheckPermission_FullMethodName, uuid.NewString(), req)
	if err != nil {
		return false, err
	}
	defer cancel()
	r, err := o.Role.CheckPermission(call, req)
	if err != nil || r == nil || unknown(r) {
		return false, unavailable()
	}
	return r.Allowed, nil
}

func (o *Owners) Blocked(ctx context.Context, viewer uuid.UUID, senders []uuid.UUID) (map[uuid.UUID]bool, error) {
	if o == nil || o.Social == nil {
		return nil, unavailable()
	}
	if viewer == uuid.Nil || len(senders) == 0 || len(senders) > 100 {
		return nil, status.Error(codes.InvalidArgument, "invalid report sender batch")
	}
	ids := make([]string, len(senders))
	expected := make(map[string]bool, len(senders))
	for i, sender := range senders {
		if sender == uuid.Nil || expected[sender.String()] {
			return nil, status.Error(codes.InvalidArgument, "invalid report sender batch")
		}
		ids[i] = sender.String()
		expected[ids[i]] = true
	}
	req := &socialv1.IsProfilePairsBlockedRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ViewerProfileId: viewer.String(), OtherProfileIds: ids}
	call, cancel, err := o.signed(ctx, "social", socialv1.SocialService_IsProfilePairsBlocked_FullMethodName, req.OperationId, req)
	if err != nil {
		return nil, err
	}
	defer cancel()
	r, err := o.Social.IsProfilePairsBlocked(call, req)
	if err != nil || r == nil || unknown(r) || r.ProtocolVersion != 1 || r.OperationId != req.OperationId || r.ViewerProfileId != req.ViewerProfileId || len(r.Results) != len(ids) {
		return nil, unavailable()
	}
	out := make(map[uuid.UUID]bool, len(ids))
	for _, row := range r.Results {
		if row == nil || !expected[row.OtherProfileId] {
			return nil, unavailable()
		}
		delete(expected, row.OtherProfileId)
		id, _ := uuid.Parse(row.OtherProfileId)
		out[id] = row.Blocked
	}
	if len(expected) != 0 {
		return nil, unavailable()
	}
	return out, nil
}
