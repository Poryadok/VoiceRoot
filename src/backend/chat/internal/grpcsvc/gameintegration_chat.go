package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/gisowner"
	"voice/backend/pkg/principal"
)

type ManagedChatStore interface {
	ProvisionManagedChat(context.Context, store.ManagedChatCreate) (store.ManagedChatCreateResult, error)
	SyncManagedChatMembers(context.Context, store.ManagedChatMemberSync) (store.ManagedChatMemberSyncResult, error)
	AddManagedChatMembers(context.Context, store.ManagedChatMemberAddition) (store.ManagedChatMemberSyncResult, error)
	SetManagedChatRetention(context.Context, store.ManagedChatRetention) (store.ManagedChatRetentionResult, error)
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
		return nil, managedChatOwnerStatus(err, req)
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
	requestHash := mustGISHash(req)
	var result store.ManagedChatMemberSyncResult
	if req.GetAddOnly() {
		result, err = s.Store.AddManagedChatMembers(ctx, store.ManagedChatMemberAddition{ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: operationID, ChatID: chatID, RequestHash: requestHash, ProfileIDs: profileIDs})
	} else {
		result, err = s.Store.SyncManagedChatMembers(ctx, store.ManagedChatMemberSync{ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: operationID, ChatID: chatID, RequestHash: requestHash, ProfileIDs: profileIDs})
	}
	if err != nil {
		return nil, managedChatOwnerStatus(err, req)
	}
	response := &chatv1.SyncManagedChatMembersResponse{Replayed: result.Replayed, ReceiptId: result.ReceiptID.String(),
		RequestHash: result.RequestHash, ProfileIds: make([]string, len(result.ProfileIDs))}
	for i, id := range result.ProfileIDs {
		response.ProfileIds[i] = id.String()
	}
	return response, nil
}

func (s *GameIntegrationChatGRPC) SetManagedChatRetention(ctx context.Context, req *chatv1.SetManagedChatRetentionRequest) (*chatv1.SetManagedChatRetentionResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	if err := requireGISBinding(ctx, req, req.GetOperationId()); err != nil {
		return nil, err
	}
	applicationID, environmentID, operationID, err := parseGISIDs(req.GetApplicationId(), req.GetEnvironmentId(), req.GetOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat retention request")
	}
	if req.GetPurgeAfter() == nil || req.GetPurgeAfter().CheckValid() != nil {
		return nil, status.Error(codes.InvalidArgument, "valid purge_after required")
	}
	result, err := s.Store.SetManagedChatRetention(ctx, store.ManagedChatRetention{
		ApplicationID: applicationID, EnvironmentID: environmentID, OperationID: operationID,
		ExternalKey: req.GetExternalChatKey(), RequestHash: mustGISHash(req), PurgeAfter: req.GetPurgeAfter().AsTime(),
	})
	if err != nil {
		return nil, managedChatStatus(err)
	}
	return &chatv1.SetManagedChatRetentionResponse{
		ChatId: result.ChatID.String(), ReceiptId: result.ReceiptID.String(), RequestHash: result.RequestHash,
		PurgeAfter: timestamppb.New(result.PurgeAfter), CompletedAt: timestamppb.New(result.CompletedAt), Replayed: result.Replayed,
	}, nil
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
	case *chatv1.SetManagedChatRetentionRequest:
		expectedRPC = "/voice.chat.v1.GameIntegrationChatService/SetManagedChatRetention"
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

type managedChatOwnerRequest interface {
	proto.Message
	GetOperationId() string
}

func managedChatOwnerStatus(err error, request managedChatOwnerRequest) error {
	publicErr := managedChatStatus(err)
	if request == nil {
		return publicErr
	}
	category := ""
	switch {
	case errors.Is(err, store.ErrManagedOperationConflict):
		category = gisowner.CategoryOperationConflict
	case errors.Is(err, store.ErrManagedResourceConflict):
		category = gisowner.CategoryResourceConflict
	case errors.Is(err, store.ErrManagedChatNotFound):
		if _, ok := request.(*chatv1.SyncManagedChatMembersRequest); ok {
			category = gisowner.CategoryResourceMissing
		}
	}
	if category == "" {
		return publicErr
	}
	rpc := ""
	switch request.(type) {
	case *chatv1.ProvisionManagedChatRequest:
		rpc = gisowner.ChatProvisionRPC
	case *chatv1.SyncManagedChatMembersRequest:
		rpc = gisowner.ChatRosterRPC
	default:
		return publicErr
	}
	hash, hashErr := principal.RequestHash(request)
	if hashErr != nil {
		return publicErr
	}
	return gisowner.Annotate(publicErr, gisowner.ChatDomain, rpc, category, request.GetOperationId(), hash)
}
