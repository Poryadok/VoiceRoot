// Typed Chat observation validation ported from published producer 5c5c94df4.
package store

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"math"
	chatv1 "voice.app/voice/chat/v1"
)

var ErrChatScopeObservation = errors.New("chat source observation is invalid")

// ChatScopeObservation contains facts obtained by the authenticated private Chat
// caller. It is not a transferable capability or a reservation over Chat state.
type ChatScopeObservation struct {
	chat, space, operation, incarnation uuid.UUID
	revision, generation                uint64
	request, response                   []byte
	requestSHA, responseSHA             [32]byte
	exists, live                        bool
}

func NewChatScopeObservation(request *chatv1.GetBotChatScopeAuthorityRequest, response *chatv1.GetBotChatScopeAuthorityResponse, admission bool) (ChatScopeObservation, error) {
	var o ChatScopeObservation
	if request == nil || response == nil || response.Scope == nil || request.ProtocolVersion != 1 || len(request.ProtoReflect().GetUnknown()) != 0 || len(response.ProtoReflect().GetUnknown()) != 0 || len(response.Scope.ProtoReflect().GetUnknown()) != 0 {
		return o, ErrChatScopeObservation
	}
	for _, item := range []struct {
		raw    string
		target *uuid.UUID
	}{{request.ChatId, &o.chat}, {request.ExpectedSpaceId, &o.space}, {request.OperationId, &o.operation}, {response.Scope.ChatScopeIncarnation, &o.incarnation}} {
		id, err := uuid.Parse(item.raw)
		if err != nil || id == uuid.Nil || id.String() != item.raw {
			return o, ErrChatScopeObservation
		}
		*item.target = id
	}
	s := response.Scope
	if s.ProtocolVersion != 1 || s.ChatId != request.ChatId || s.ExpectedSpaceId != request.ExpectedSpaceId || s.ActualSpaceId != request.ExpectedSpaceId || s.OperationId != request.OperationId || s.ChatAuthorityRevision == 0 || s.ChatAuthorityRevision > math.MaxInt64 || s.LifecycleGeneration > math.MaxInt64 || s.LifecycleState < chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE || s.LifecycleState > chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_PURGED {
		return o, ErrChatScopeObservation
	}
	if s.Exists {
		if (s.ChatType != chatv1.ChatType_CHAT_TYPE_GROUP && s.ChatType != chatv1.ChatType_CHAT_TYPE_CHANNEL) || s.ManagedKind < chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY || s.ManagedKind > chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD || s.Live != (s.LifecycleState == chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE) {
			return o, ErrChatScopeObservation
		}
	} else if s.Live || s.ChatType != chatv1.ChatType_CHAT_TYPE_UNSPECIFIED || s.ManagedKind != chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED {
		return o, ErrChatScopeObservation
	}
	if admission && (!s.Exists || !s.Live) {
		return o, ErrChatScopeObservation
	}
	var err error
	o.request, err = (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return o, ErrChatScopeObservation
	}
	o.requestSHA = sha256.Sum256(o.request)
	if !bytes.Equal(response.RequestSha256, o.requestSHA[:]) {
		return o, ErrChatScopeObservation
	}
	o.response, err = (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return o, ErrChatScopeObservation
	}
	o.responseSHA = sha256.Sum256(o.response)
	o.revision, o.generation, o.exists, o.live = s.ChatAuthorityRevision, s.LifecycleGeneration, s.Exists, s.Live
	return o, nil
}
