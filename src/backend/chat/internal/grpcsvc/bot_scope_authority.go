package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type BotChatScopeAuthorityStore interface {
	GetBotChatScopeAuthority(context.Context, uuid.UUID, uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error)
}

type BotChatScopeAuthorityGRPC struct {
	chatv1.UnimplementedChatScopeAuthorityServiceServer
	Store BotChatScopeAuthorityStore
}

func (s *BotChatScopeAuthorityGRPC) GetBotChatScopeAuthority(ctx context.Context, request *chatv1.GetBotChatScopeAuthorityRequest) (*chatv1.GetBotChatScopeAuthorityResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Chat scope principal required")
	}
	if p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return nil, status.Error(codes.PermissionDenied, "Bot scope service principal required")
	}
	if request == nil || request.ProtocolVersion != 1 || len(request.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid Chat scope request")
	}
	request = proto.Clone(request).(*chatv1.GetBotChatScopeAuthorityRequest)
	ids := make([]uuid.UUID, 0, 3)
	for _, raw := range []string{request.ChatId, request.ExpectedSpaceId, request.OperationId} {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return nil, status.Error(codes.InvalidArgument, "invalid Chat scope request")
		}
		ids = append(ids, id)
	}
	hash, err := principal.RequestHash(request)
	validBinding := func() bool {
		return p.Audience == "chat" && p.RPC == chatv1.ChatScopeAuthorityService_GetBotChatScopeAuthority_FullMethodName && p.RequestID == request.OperationId && p.RequestHash == hash && p.JWTID != "" && !p.IssuedAt.IsZero() && p.ExpiresAt.After(time.Now()) && p.ExpiresAt.Sub(p.IssuedAt) > 0 && p.ExpiresAt.Sub(p.IssuedAt) <= 30*time.Second
	}
	if err != nil || !validBinding() {
		return nil, status.Error(codes.Unauthenticated, "invalid Chat scope principal binding")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	scope, err := s.Store.GetBotChatScopeAuthority(ctx, ids[0], ids[1])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "Chat scope not found")
		}
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	if !validBinding() {
		return nil, status.Error(codes.Unauthenticated, "Chat scope principal expired")
	}
	if scope == nil || len(scope.ProtoReflect().GetUnknown()) != 0 || scope.ProtocolVersion != 1 || scope.ChatId != request.ChatId || scope.ExpectedSpaceId != request.ExpectedSpaceId || scope.ActualSpaceId != request.ExpectedSpaceId || scope.ChatAuthorityRevision == 0 || scope.ChatAuthorityRevision > math.MaxInt64 || scope.LifecycleGeneration > math.MaxInt64 {
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	incarnation, err := uuid.Parse(scope.ChatScopeIncarnation)
	if err != nil || incarnation == uuid.Nil || incarnation.String() != scope.ChatScopeIncarnation || scope.LifecycleState < chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE || scope.LifecycleState > chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_PURGED {
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	if scope.Exists {
		if (scope.ChatType != chatv1.ChatType_CHAT_TYPE_GROUP && scope.ChatType != chatv1.ChatType_CHAT_TYPE_CHANNEL) || scope.ManagedKind < chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY || scope.ManagedKind > chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD || scope.Live != (scope.LifecycleState == chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE) {
			return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
		}
	} else if scope.Live || scope.ChatType != chatv1.ChatType_CHAT_TYPE_UNSPECIFIED || scope.ManagedKind != chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED {
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	scope = proto.Clone(scope).(*chatv1.GetSpaceChatScopeAuthorityResponse)
	scope.OperationId = request.OperationId
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Chat scope request")
	}
	digest := sha256.Sum256(body)
	return &chatv1.GetBotChatScopeAuthorityResponse{Scope: scope, RequestSha256: digest[:]}, nil
}
