package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type SpaceChatScopeAuthorityStore interface {
	GetSpaceChatScopeAuthority(context.Context, uuid.UUID, uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error)
}

type SpaceChatScopeAuthorityGRPC struct {
	chatv1.UnimplementedChatScopeAuthorityServiceServer
	Store SpaceChatScopeAuthorityStore
}

func (s *SpaceChatScopeAuthorityGRPC) GetSpaceChatScopeAuthority(ctx context.Context, req *chatv1.GetSpaceChatScopeAuthorityRequest) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Chat scope principal required")
	}
	if p.Kind != "service" || (p.Issuer != "role" && p.Issuer != "space") || p.Subject != "service:"+p.Issuer || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "Chat scope service principal required")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.ProtocolVersion != 1 {
		return nil, status.Error(codes.InvalidArgument, "invalid Chat scope request")
	}
	parse := func(raw string) (uuid.UUID, error) {
		id, e := uuid.Parse(raw)
		if e != nil || id == uuid.Nil || id.String() != raw {
			return uuid.Nil, status.Error(codes.InvalidArgument, "invalid Chat scope identifier")
		}
		return id, nil
	}
	chatID, err := parse(req.ChatId)
	if err != nil {
		return nil, err
	}
	spaceID, err := parse(req.ExpectedSpaceId)
	if err != nil {
		return nil, err
	}
	if _, err := parse(req.OperationId); err != nil {
		return nil, err
	}
	hash, err := principal.RequestHash(req)
	if err != nil || p.Audience != "chat" || p.RPC != chatv1.ChatScopeAuthorityService_GetSpaceChatScopeAuthority_FullMethodName || p.RequestID != req.OperationId || p.RequestHash != hash {
		return nil, status.Error(codes.Unauthenticated, "invalid Chat scope principal binding")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	response, err := s.Store.GetSpaceChatScopeAuthority(ctx, chatID, spaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "Chat scope not found")
		}
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	if response == nil || response.ChatId != req.ChatId || response.ActualSpaceId != req.ExpectedSpaceId || response.ChatAuthorityRevision == 0 || response.ChatScopeIncarnation == "" {
		return nil, status.Error(codes.Unavailable, "Chat scope authority unavailable")
	}
	response.OperationId = req.OperationId
	return response, nil
}
