package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

// ProjectGameActionResult accepts only a request-bound GIS service principal.
// The caller may project a result only onto a matching immutable card/action.
func (s *MessagingGRPC) ProjectGameActionResult(ctx context.Context, req *messagingv1.ProjectGameActionResultRequest) (*messagingv1.ProjectGameActionResultResponse, error) {
	verified, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(req)
	if !ok || err != nil || verified.Kind != "service" || verified.Issuer != "gameintegration" || verified.Subject != "service:gameintegration" ||
		verified.Audience != "messaging" || verified.RPC != messagingv1.MessagingService_ProjectGameActionResult_FullMethodName ||
		verified.RequestID != req.GetOperationId() || verified.RequestHash != hash {
		return nil, status.Error(codes.Unauthenticated, "invalid GIS principal binding")
	}
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.Unavailable, "game action result projection unavailable")
	}
	messageID, err := uuid.Parse(req.GetMessageId())
	if err != nil || messageID == uuid.Nil || messageID.String() != req.GetMessageId() {
		return nil, status.Error(codes.InvalidArgument, "invalid result projection")
	}
	appID, err := uuid.Parse(req.GetApplicationId())
	if err != nil || appID == uuid.Nil || appID.String() != req.GetApplicationId() {
		return nil, status.Error(codes.InvalidArgument, "invalid result projection")
	}
	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil || envID == uuid.Nil || envID.String() != req.GetEnvironmentId() {
		return nil, status.Error(codes.InvalidArgument, "invalid result projection")
	}
	operationID, err := uuid.Parse(req.GetOperationId())
	if err != nil || operationID == uuid.Nil || operationID.String() != req.GetOperationId() {
		return nil, status.Error(codes.InvalidArgument, "invalid result projection")
	}
	resultID, err := uuid.Parse(req.GetResultId())
	if err != nil || resultID == uuid.Nil || resultID.String() != req.GetResultId() {
		return nil, status.Error(codes.InvalidArgument, "invalid result projection")
	}
	projected, err := s.Messages.ProjectGameActionResult(ctx, store.ProjectGameActionResultRequest{MessageID: messageID, AppID: appID, EnvID: envID, Result: store.GameActionResult{OperationID: operationID, ActionID: req.GetActionId(), ResultID: resultID, StateVersion: req.GetStateVersion(), Status: req.GetStatus(), SafeSummary: req.GetSafeSummary()}})
	if err != nil {
		if errors.Is(err, store.ErrGameActionResultConflict) {
			return nil, status.Error(codes.FailedPrecondition, "game action result conflicts with card state")
		}
		return nil, status.Error(codes.Unavailable, "game action result projection unavailable")
	}
	r := projected.Result
	return &messagingv1.ProjectGameActionResultResponse{Replayed: projected.Replayed, Result: &messagingv1.GameActionResult{OperationId: r.OperationID.String(), ActionId: r.ActionID, ResultId: r.ResultID.String(), StateVersion: r.StateVersion, Status: r.Status, SafeSummary: r.SafeSummary, RecordedAt: timestamppb.New(r.RecordedAt.UTC())}}, nil
}

var _ = context.Background
