package lifecyclecoord

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

type MatchmakingLifecycleClient interface {
	ApplySpaceLifecycleFence(context.Context, *matchmakingv1.ApplySpaceLifecycleFenceRequest, ...grpc.CallOption) (*matchmakingv1.ApplySpaceLifecycleFenceResponse, error)
	PurgeSpace(context.Context, *matchmakingv1.PurgeSpaceRequest, ...grpc.CallOption) (*matchmakingv1.PurgeSpaceResponse, error)
}

// MatchmakingParticipant uses the dedicated mTLS listener and binds every call
// to its exact typed protobuf request with a short-lived Space service principal.
type MatchmakingParticipant struct {
	Issuer *principal.Issuer
	Client MatchmakingLifecycleClient
}

func (p *MatchmakingParticipant) ApplySpaceLifecycleFence(ctx context.Context, fence *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if p == nil || p.Issuer == nil || p.Client == nil || fence == nil {
		return nil, status.Error(codes.Unavailable, "Matchmaking lifecycle transport unavailable")
	}
	request := &matchmakingv1.ApplySpaceLifecycleFenceRequest{Fence: fence}
	requestID, err := matchmakingLifecycleRequestID(fence.GetDeletionOperationId(), fence.GetGeneration(), fmt.Sprintf("fence-%d", fence.GetDesiredState()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Matchmaking lifecycle request binding")
	}
	ctx, err = signedLifecycleContextFor(ctx, p.Issuer, matchmakingv1.MatchmakingService_ApplySpaceLifecycleFence_FullMethodName, requestID, request)
	if err != nil {
		return nil, err
	}
	response, err := p.Client.ApplySpaceLifecycleFence(ctx, request)
	if err != nil {
		return nil, err
	}
	return response.GetReceipt(), nil
}

func (p *MatchmakingParticipant) PurgeSpace(ctx context.Context, purge *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	if p == nil || p.Issuer == nil || p.Client == nil || purge == nil {
		return nil, status.Error(codes.Unavailable, "Matchmaking lifecycle transport unavailable")
	}
	request := &matchmakingv1.PurgeSpaceRequest{Purge: purge}
	requestID, err := matchmakingLifecycleRequestID(purge.GetDeletionOperationId(), purge.GetGeneration(), "purge")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Matchmaking purge request binding")
	}
	ctx, err = signedLifecycleContextFor(ctx, p.Issuer, matchmakingv1.MatchmakingService_PurgeSpace_FullMethodName, requestID, request)
	if err != nil {
		return nil, err
	}
	response, err := p.Client.PurgeSpace(ctx, request)
	if err != nil {
		return nil, err
	}
	return response.GetReceipt(), nil
}

func signedLifecycleContextFor(ctx context.Context, issuer *principal.Issuer, method, requestID string, request proto.Message) (context.Context, error) {
	return signedLifecycleContextForAudience(ctx, issuer, "matchmaking", method, requestID, request)
}

func signedLifecycleContextForAudience(ctx context.Context, issuer *principal.Issuer, audience, method, requestID string, request proto.Message) (context.Context, error) {
	if ctx == nil || issuer == nil || request == nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle principal unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle RPC request")
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle principal signing unavailable")
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", requestID), nil
}

func matchmakingLifecycleRequestID(operationID string, generation uint64, action string) (string, error) {
	operation, err := uuid.Parse(operationID)
	if err != nil || operation == uuid.Nil || operation.String() != operationID || generation == 0 || action == "" {
		return "", fmt.Errorf("invalid lifecycle operation identity")
	}
	return uuid.NewSHA1(operation, []byte(fmt.Sprintf("matchmaking.space-lifecycle.%s.v1\x00%d", action, generation))).String(), nil
}
