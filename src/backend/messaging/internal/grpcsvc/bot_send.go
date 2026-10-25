package grpcsvc

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
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/messaging/internal/botsendprincipal"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
)

type BotSendOwner interface {
	CheckBotMessageSendAuthority(context.Context, *botv1.CheckBotMessageSendAuthorityRequest, ...grpc.CallOption) (*botv1.CheckBotMessageSendAuthorityResponse, error)
}
type BotSendSpaceOwner interface {
	GetMemberAdmissionAuthority(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest, ...grpc.CallOption) (*spacev1.GetMemberAdmissionAuthorityResponse, error)
}
type BotSendAuthority struct {
	Bot    BotSendOwner
	Space  BotSendSpaceOwner
	Issuer *principal.Issuer
	Scope  ChatCommunicationScope
}

func canonicalBotSendID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}
func validBotSendBinding(b *commonv1.BotMessageSendBinding) bool {
	if b == nil || botsendprincipal.Unknown(b) || b.ProtocolVersion != 1 || b.InstallationGeneration == 0 || b.InstallationGeneration > math.MaxInt64 {
		return false
	}
	for _, id := range []string{b.BotId, b.ActorProfileId, b.SpaceId, b.InstallationId, b.InstallOperationId, b.ChatId} {
		if !canonicalBotSendID(id) {
			return false
		}
	}
	return true
}
func (a *BotSendAuthority) signed(ctx context.Context, audience, rpc string, body proto.Message) (context.Context, error) {
	if a == nil || a.Issuer == nil {
		return nil, status.Error(codes.Unavailable, "Bot send signer unavailable")
	}
	hash, err := principal.RequestHash(body)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid authority request")
	}
	id := uuid.NewString()
	token, err := a.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Bot send signer unavailable")
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), nil
}

// Check obtains independent current owner observations. Their shortest remaining
// validity is measured from BEFORE dispatch, never from response receipt.
func (a *BotSendAuthority) Check(ctx context.Context, b *commonv1.BotMessageSendBinding) (*store.ChatThreadPolicy, time.Time, error) {
	deny := func() (*store.ChatThreadPolicy, time.Time, error) {
		return nil, time.Time{}, status.Error(codes.PermissionDenied, "Bot send authority denied")
	}
	unavailable := func() (*store.ChatThreadPolicy, time.Time, error) {
		return nil, time.Time{}, status.Error(codes.Unavailable, "Bot send authority unavailable")
	}
	if !validBotSendBinding(b) {
		return deny()
	}
	deadline, finite := ctx.Deadline()
	if !finite || time.Until(deadline) > 5*time.Second || ctx.Err() != nil {
		return unavailable()
	}
	if a == nil || isNilDependency(a.Bot) || isNilDependency(a.Space) || isNilDependency(a.Scope) || a.Issuer == nil {
		return unavailable()
	}
	policy, err := a.Scope.Load(ctx, uuid.MustParse(b.ChatId))
	if err != nil || policy == nil {
		return unavailable()
	}
	if policy.SpaceID == nil || policy.SpaceID.String() != b.SpaceId || (policy.ChatType != "group" && policy.ChatType != "channel") {
		return deny()
	}
	request := &botv1.CheckBotMessageSendAuthorityRequest{Binding: proto.Clone(b).(*commonv1.BotMessageSendBinding)}
	call, err := a.signed(ctx, "bot", botv1.BotService_CheckBotMessageSendAuthority_FullMethodName, request)
	if err != nil {
		return unavailable()
	}
	decision, err := a.Bot.CheckBotMessageSendAuthority(call, request)
	if err != nil {
		return unavailable()
	}
	if decision == nil || botsendprincipal.Unknown(decision) || !decision.Allowed || !proto.Equal(decision.Binding, b) {
		return deny()
	}
	spaceRequest := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: b.SpaceId, ProfileId: b.ActorProfileId, ChatId: &b.ChatId, BotMessageSendBinding: proto.Clone(b).(*commonv1.BotMessageSendBinding)}
	call, err = a.signed(ctx, "space", spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName, spaceRequest)
	if err != nil {
		return unavailable()
	}
	started := time.Now()
	observation, err := a.Space.GetMemberAdmissionAuthority(call, spaceRequest)
	if err != nil {
		return unavailable()
	}
	if observation == nil || botsendprincipal.Unknown(observation) || observation.ProtocolVersion != 1 || observation.SpaceId != b.SpaceId || observation.ProfileId != b.ActorProfileId || observation.SpaceAuthorityRevision == 0 || !proto.Equal(observation.BotMessageSendBinding, b) {
		return deny()
	}
	life, owner, ordinary, tree := observation.GetLifecycle(), observation.GetOwnership(), observation.GetOrdinary(), observation.GetTreeRelation()
	// Space authenticates a fresh LIVE lifecycle as generation zero.
	// Ownership and native membership generations remain independently positive.
	if life.GetState() != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE || owner.GetGeneration() == 0 || owner.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE || ordinary.GetState() != spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE || ordinary.GetExplicitLegacy() || ordinary.GetHeadGeneration() == 0 || ordinary.GetHeadGeneration() <= ordinary.GetRetiredThrough() || ordinary.GetPublicationKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL || tree.GetChatId() != b.ChatId || !tree.GetPresent() || tree.GetRevision() == 0 {
		return deny()
	}
	native := ordinary.GetNativePublication()
	published := ordinary.GetPublishedBinding().GetOrdinary()
	if native.GetKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL || native.GetOperationId() != b.InstallOperationId || len(native.GetBindingSha256()) != 32 || len(ordinary.GetRoleReceiptSha256()) != 0 || published == nil || published.Legacy || published.MembershipGeneration != ordinary.HeadGeneration || published.PublicationIdentity != b.InstallOperationId {
		return deny()
	}
	validity := observation.GetValidityRemaining()
	if validity == nil || validity.CheckValid() != nil || validity.AsDuration() <= 0 || validity.AsDuration() > 5*time.Second {
		return unavailable()
	}
	expires := started.Add(validity.AsDuration())
	if deadline.Before(expires) {
		expires = deadline
	}
	if !time.Now().Before(expires) || ctx.Err() != nil {
		return unavailable()
	}
	return policy, expires, nil
}

type botSendAdmission struct {
	request   *messagingv1.SendBotMessageRequest
	proof     botsendprincipal.Verified
	authority *BotSendAuthority
	chatType  chatv1.ChatType
}

func (a *botSendAdmission) beforeWrite(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if a == nil || a.proof.Validate(a.request) != nil {
		return nil, nil, status.Error(codes.Unauthenticated, "Bot send proof expired or changed")
	}
	policy, expires, err := a.authority.Check(ctx, a.request.Binding)
	if err != nil {
		return nil, nil, err
	}
	if a.proof.Validate(a.request) != nil {
		return nil, nil, status.Error(codes.Unauthenticated, "Bot send proof expired or changed")
	}
	if chatTypeName(a.chatType) != policy.ChatType {
		return nil, nil, status.Error(codes.PermissionDenied, "Bot destination changed")
	}
	next, cancel := context.WithDeadline(ctx, expires)
	return next, cancel, nil
}

// BotSendGRPC is registered on a descriptor containing ONLY SendBotMessage.
// MessagingGRPC retains the generated unimplemented method on all other ports.
type BotSendGRPC struct {
	messagingv1.UnimplementedMessagingServiceServer
	Messaging *MessagingGRPC
	Authority *BotSendAuthority
}

func (s *BotSendGRPC) SendBotMessage(ctx context.Context, request *messagingv1.SendBotMessageRequest) (*messagingv1.SendBotMessageResponse, error) {
	proof, err := botsendprincipal.Require(ctx, request)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified Bot send required")
	}
	if s == nil || s.Messaging == nil {
		return nil, status.Error(codes.Unavailable, "Bot send unavailable")
	}
	b, m := request.GetBinding(), request.GetMessage()
	if !validBotSendBinding(b) || m == nil || m.GetChat().GetId() != b.ChatId {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot send binding")
	}
	if m.DeliverySchedule != nil || m.GetPostedAsChat() {
		return nil, status.Error(codes.PermissionDenied, "Bot send requires immediate actor identity")
	}
	policy, expires, err := s.Authority.Check(ctx, b)
	if err != nil {
		return nil, err
	}
	kind := chatv1.ChatType_CHAT_TYPE_GROUP
	if policy.ChatType == "channel" {
		kind = chatv1.ChatType_CHAT_TYPE_CHANNEL
	}
	if m.GetChat().GetType() != kind {
		return nil, status.Error(codes.PermissionDenied, "Bot destination type mismatch")
	}
	attempt, cancel := context.WithDeadline(ctx, expires)
	defer cancel()
	admission := &botSendAdmission{request: proto.Clone(request).(*messagingv1.SendBotMessageRequest), proof: proof, authority: s.Authority, chatType: kind}
	response, err := s.Messaging.sendMessage(attempt, admission.request.Message, admission)
	if err != nil {
		return nil, err
	}
	return &messagingv1.SendBotMessageResponse{Message: response.Message}, nil
}

// Only SEND itself is replaced by Bot's exact owner decision. Thread and all
// other Role permissions still use the ordinary checker.
type botSendRoleChecker struct {
	admission *botSendAdmission
	ordinary  mentionsRoleChecker
}

func (c botSendRoleChecker) HasChatPermission(ctx context.Context, space, profile, chat uuid.UUID, permission string) (bool, error) {
	b := c.admission.request.Binding
	if permission == permissions.TextChatSendMessages && space.String() == b.SpaceId && profile.String() == b.ActorProfileId && chat.String() == b.ChatId && ctx.Err() == nil && c.admission.proof.Validate(c.admission.request) == nil {
		return true, nil
	}
	if isNilDependency(c.ordinary) {
		return false, status.Error(codes.Unavailable, "role authority unavailable")
	}
	return c.ordinary.HasChatPermission(ctx, space, profile, chat, permission)
}
