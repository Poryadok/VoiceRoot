package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

type ManagedChatStore interface {
	ProvisionManagedChat(context.Context, store.ManagedChatCreate) (store.ManagedChatCreateResult, error)
	SyncManagedChatMembers(context.Context, store.ManagedChatMemberSync) (store.ManagedChatMemberSyncResult, error)
}

// GameIntegrationChatGRPC is registered only on Chat's GIS mTLS listener.
type GameIntegrationChatGRPC struct {
	chatv1.UnimplementedGameIntegrationChatServiceServer
	Store ManagedChatStore
}

func (s *GameIntegrationChatGRPC) ProvisionManagedChat(ctx context.Context, req *chatv1.ProvisionManagedChatRequest) (*chatv1.ProvisionManagedChatResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	if err := requireGISBinding(ctx, req, req.GetOperationId()); err != nil {
		return nil, err
	}
	applicationID, environmentID, operationID, err := parseGISIDs(req.GetApplicationId(), req.GetEnvironmentId(), req.GetOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat request")
	}
	result, err := s.Store.ProvisionManagedChat(ctx, store.ManagedChatCreate{ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: operationID, ExternalKey: req.GetExternalChatKey(), RequestHash: mustGISHash(req), Name: req.GetName(), Topic: req.Topic})
	if err != nil {
		return nil, managedChatStatus(err)
	}
	return &chatv1.ProvisionManagedChatResponse{ChatId: result.ChatID.String(), Replayed: result.Replayed,
		ReceiptId: result.ReceiptID.String(), RequestHash: result.RequestHash}, nil
}

func (s *GameIntegrationChatGRPC) SyncManagedChatMembers(ctx context.Context, req *chatv1.SyncManagedChatMembersRequest) (*chatv1.SyncManagedChatMembersResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	if err := requireGISBinding(ctx, req, req.GetOperationId()); err != nil {
		return nil, err
	}
	applicationID, environmentID, operationID, err := parseGISIDs(req.GetApplicationId(), req.GetEnvironmentId(), req.GetOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat request")
	}
	chatID, err := uuid.Parse(req.GetChatId())
	if err != nil || chatID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat request")
	}
	profileIDs := make([]uuid.UUID, 0, len(req.GetProfileIds()))
	for _, raw := range req.GetProfileIds() {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "invalid managed chat request")
		}
		profileIDs = append(profileIDs, id)
	}
	result, err := s.Store.SyncManagedChatMembers(ctx, store.ManagedChatMemberSync{ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: operationID, ChatID: chatID, RequestHash: mustGISHash(req), ProfileIDs: profileIDs})
	if err != nil {
		return nil, managedChatStatus(err)
	}
	response := &chatv1.SyncManagedChatMembersResponse{Replayed: result.Replayed, ReceiptId: result.ReceiptID.String(),
		RequestHash: result.RequestHash, ProfileIds: make([]string, len(result.ProfileIDs))}
	for i, id := range result.ProfileIDs {
		response.ProfileIds[i] = id.String()
	}
	return response, nil
}

func requireGISBinding(ctx context.Context, request proto.Message, operationID string) error {
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "gameintegration" || verified.Subject != "service:gameintegration" {
		return status.Error(codes.PermissionDenied, "GIS service principal required")
	}
	hash, err := principal.RequestHash(request)
	expectedRPC := ""
	switch request.(type) {
	case *chatv1.ProvisionManagedChatRequest:
		expectedRPC = "/voice.chat.v1.GameIntegrationChatService/ProvisionManagedChat"
	case *chatv1.SyncManagedChatMembersRequest:
		expectedRPC = "/voice.chat.v1.GameIntegrationChatService/SyncManagedChatMembers"
	}
	if err != nil || expectedRPC == "" || verified.Audience != "chat" || verified.RPC != expectedRPC || verified.RequestID != operationID || verified.RequestHash != hash {
		return status.Error(codes.Unauthenticated, "invalid principal binding")
	}
	return nil
}
func mustGISHash(request proto.Message) string {
	hash, _ := principal.RequestHash(request)
	return hash
}
func parseGISIDs(values ...string) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	parsed := make([]uuid.UUID, 3)
	for i, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return uuid.Nil, uuid.Nil, uuid.Nil, errors.New("invalid UUID")
		}
		parsed[i] = id
	}
	return parsed[0], parsed[1], parsed[2], nil
}
func managedChatStatus(err error) error {
	switch {
	case errors.Is(err, store.ErrManagedOperationConflict), errors.Is(err, store.ErrManagedResourceConflict):
		return status.Error(codes.AlreadyExists, "managed chat operation conflicts")
	case errors.Is(err, store.ErrManagedChatNotFound):
		return status.Error(codes.NotFound, "managed chat not found")
	case errors.Is(err, store.ErrManagedChatPrincipalRequired):
		return status.Error(codes.PermissionDenied, "managed chat principal required")
	default:
		return status.Error(codes.Internal, "managed chat operation failed")
	}
}
