package grpcsvc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func installationIdentity(i *store.InstallationIntent, cancel bool) *spacev1.BotInstallationIntent {
	op := i.OperationID
	if cancel {
		op = i.CancelOperationID
	}
	return &spacev1.BotInstallationIntent{OperationId: op.String(), BotId: i.BotID.String(), InstallationId: i.InstallationID.String(), SpaceId: i.SpaceID.String(), ActorProfileId: i.ActorProfileID.String(), Generation: uint64(i.Generation)}
}

func (s *BotGRPC) installationCallContext(ctx context.Context, request proto.Message, method, operation string) (context.Context, context.CancelFunc, error) {
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, nil, status.Error(codes.Internal, "Bot installation request binding failed")
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
	if err != nil {
		return nil, nil, status.Error(codes.Unavailable, "Bot installation principal unavailable")
	}
	// Do not forward the initiating user's raw headers or previous credentials.
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	return callCtx, cancel, nil
}

func (s *BotGRPC) addInstallationMember(ctx context.Context, i *store.InstallationIntent) error {
	req := &spacev1.AddBotMemberRequest{SpaceId: i.SpaceID.String(), ProfileId: i.ActorProfileID.String(), InstallationIntent: installationIdentity(i, false)}
	callCtx, cancel, err := s.installationCallContext(ctx, req, spacev1.SpaceService_AddBotMember_FullMethodName, i.OperationID.String())
	if err != nil {
		return err
	}
	defer cancel()
	resp, err := s.InstallationSpace.AddBotMember(callCtx, req)
	if err != nil || resp == nil {
		return status.Error(codes.Unavailable, "Space Bot installation acknowledgement unavailable; retry the same installation")
	}
	return nil
}

func (s *BotGRPC) removeInstallationMember(ctx context.Context, i *store.InstallationIntent) error {
	req := &spacev1.RemoveBotMemberRequest{SpaceId: i.SpaceID.String(), ProfileId: i.ActorProfileID.String(), InstallationIntent: installationIdentity(i, true), InstallOperationId: i.OperationID.String()}
	callCtx, cancel, err := s.installationCallContext(ctx, req, spacev1.SpaceService_RemoveBotMember_FullMethodName, i.CancelOperationID.String())
	if err != nil {
		return err
	}
	defer cancel()
	resp, err := s.InstallationSpace.RemoveBotMember(callCtx, req)
	if err != nil || resp == nil {
		return status.Error(codes.Unavailable, "Space Bot cancellation acknowledgement unavailable; retry the same cancellation")
	}
	return nil
}
