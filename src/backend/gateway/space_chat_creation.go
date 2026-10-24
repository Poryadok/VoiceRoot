package main

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) createCompositeSpaceChat(w http.ResponseWriter, r *http.Request, spaceID string, request *chatv1.CreateChatRequest) {
	if request.RequestId == "" {
		request.RequestId = uuid.NewString()
	}
	ctx, err := t.auditInitiatingContext(r, request, chatv1.ChatService_CreateChat_FullMethodName, request.RequestId)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !canonicalLifecycleUUID(spaceID) || !canonicalLifecycleUUID(request.RequestId) {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "canonical Space and request IDs required"))
		return
	}
	// Require both protected transports before creating any prepared Chat.
	if t.clients.spaceLifecycle == nil {
		writeGRPCError(w, status.Error(codes.Unavailable, "protected Space outcome transport unavailable"))
		return
	}
	response, err := t.clients.chatAudit.CreateChat(ctx, request)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if response == nil || response.Chat == nil || !canonicalLifecycleUUID(response.GetCompositeOperationId()) || !canonicalLifecycleUUID(response.GetCompositeNodeId()) || !canonicalLifecycleUUID(response.Chat.Id) || response.Chat.GetSpaceId() != spaceID || response.Chat.Type != request.Type {
		writeGRPCError(w, status.Error(codes.Unavailable, "incomplete composite Chat result"))
		return
	}
	lookup := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: spaceID, OperationId: response.CompositeOperationId}
	outcomeCtx, err := t.compositeOutcomeContext(r, lookup)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	result, err := t.clients.spaceLifecycle.GetSpaceChatCreationOutcome(outcomeCtx, lookup)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	claims, _ := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !validCompositeSpaceResult(result, response, spaceID, claims) {
		writeGRPCError(w, status.Error(codes.Unavailable, "invalid composite Space outcome"))
		return
	}
	node := proto.Clone(result.Outcome.Node).(*spacev1.SpaceTreeNode)
	if name := strings.TrimSpace(response.Chat.GetName()); name != "" {
		node.DisplayName = &name
	}
	writeProtoJSON(w, http.StatusOK, &spacev1.UpsertTreeNodeResponse{SpaceTreeNode: node})
}

func validCompositeSpaceResult(result *spacev1.GetSpaceChatCreationOutcomeResponse, chat *chatv1.CreateChatResponse, spaceID string, actor tokenClaims) bool {
	if result == nil || result.Outcome == nil || chat == nil || chat.Chat == nil {
		return false
	}
	out := result.Outcome
	b := out.Binding
	node := out.Node
	if b == nil || node == nil || out.State != spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED || out.FailureCode != 0 || b.ProtocolVersion != 1 || len(b.CreateRequestSha256) != 32 {
		return false
	}
	for _, id := range []string{b.OperationId, b.CreatorProfileId, b.SpaceId, b.ChatId, b.NodeId, b.AuditEventId} {
		if !canonicalLifecycleUUID(id) {
			return false
		}
	}
	user := b.GetUser()
	if user == nil || user.AccountId != actor.UserID || b.CreatorProfileId != actor.ProfileID || user.SessionEpoch <= 0 || (user.AccountType != "regular" && user.AccountType != "guest") {
		return false
	}
	if b.OperationId != chat.CompositeOperationId || b.SpaceId != spaceID || b.ChatId != chat.Chat.Id || b.ChatType != chat.Chat.Type || b.NodeId != chat.CompositeNodeId {
		return false
	}
	if b.ChatType != chatv1.ChatType_CHAT_TYPE_GROUP && b.ChatType != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return false
	}
	return node.Id == b.NodeId && node.SpaceId == b.SpaceId && node.Kind == "text_chat" && node.VoiceRoomId == nil && node.LinkedChat != nil && node.LinkedChat.Id == b.ChatId && node.LinkedChat.Type != nil && node.LinkedChat.GetType() == b.ChatType
}
