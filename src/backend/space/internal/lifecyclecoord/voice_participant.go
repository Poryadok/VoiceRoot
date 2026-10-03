package lifecyclecoord

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/principal"
)

// VoiceParticipant adapts Voice's protected lifecycle RPCs to the coordinator
// contract. The supplied connection must use the dedicated mTLS listener.
type VoiceParticipant struct {
	Issuer *principal.Issuer
	Client callsv1.VoiceServiceClient
}

func (p *VoiceParticipant) ApplySpaceLifecycleFence(ctx context.Context, fence *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if p == nil || p.Issuer == nil || p.Client == nil || fence == nil {
		return nil, status.Error(codes.Unavailable, "Voice lifecycle transport unavailable")
	}
	request := &callsv1.ApplySpaceLifecycleFenceRequest{Fence: fence}
	requestID, err := lifecycleRequestID(fence.GetDeletionOperationId(), fence.GetGeneration(), fmt.Sprintf("fence-%d", fence.GetDesiredState()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Voice lifecycle request binding")
	}
	ctx, err = signedLifecycleContext(ctx, p.Issuer, callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName, requestID, request)
	if err != nil {
		return nil, err
	}
	response, err := p.Client.ApplySpaceLifecycleFence(ctx, request)
	if err != nil {
		return nil, err
	}
	return response.GetReceipt(), nil
}

func (p *VoiceParticipant) PurgeSpace(ctx context.Context, purge *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	if p == nil || p.Issuer == nil || p.Client == nil || purge == nil {
		return nil, status.Error(codes.Unavailable, "Voice lifecycle transport unavailable")
	}
	request := &callsv1.PurgeSpaceRequest{Purge: purge}
	requestID, err := lifecycleRequestID(purge.GetDeletionOperationId(), purge.GetGeneration(), "purge")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Voice purge request binding")
	}
	ctx, err = signedLifecycleContext(ctx, p.Issuer, callsv1.VoiceService_PurgeSpace_FullMethodName, requestID, request)
	if err != nil {
		return nil, err
	}
	response, err := p.Client.PurgeSpace(ctx, request)
	if err != nil {
		return nil, err
	}
	return response.GetReceipt(), nil
}

func signedLifecycleContext(ctx context.Context, issuer *principal.Issuer, method, requestID string, request proto.Message) (context.Context, error) {
	if ctx == nil || issuer == nil || request == nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle principal unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle RPC request")
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "voice", RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle principal signing unavailable")
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", requestID), nil
}

func lifecycleRequestID(operationID string, generation uint64, action string) (string, error) {
	operation, err := uuid.Parse(operationID)
	if err != nil || operation == uuid.Nil || operation.String() != operationID || generation == 0 || action == "" {
		return "", fmt.Errorf("invalid lifecycle operation identity")
	}
	return uuid.NewSHA1(operation, []byte(fmt.Sprintf("voice.space-lifecycle.%s.v1\x00%d", action, generation))).String(), nil
}
