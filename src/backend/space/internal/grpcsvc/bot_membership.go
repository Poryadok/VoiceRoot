package grpcsvc

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/store"
)

type BotMembershipStore interface {
	AddBotMemberWithInstallation(context.Context, *botmembershipprincipal.VerifiedInstallation, *spacev1.AddBotMemberRequest) error
	RemoveBotMemberWithInstallation(context.Context, *botmembershipprincipal.VerifiedInstallation, *spacev1.RemoveBotMemberRequest) error
}

type BotMembershipGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Store BotMembershipStore
}

type BotGlobalRemovalStore interface {
	ApplyBotInstallationRemoval(context.Context, *botmembershipprincipal.VerifiedGlobalRemoval, *spacev1.ApplyBotInstallationRemovalRequest) (*spacev1.BotInstallationRemovalReceipt, error)
}

func (s *SpaceGRPC) ApplyBotInstallationRemoval(context.Context, *spacev1.ApplyBotInstallationRemovalRequest) (*spacev1.ApplyBotInstallationRemovalResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "protected Bot membership listener required")
}

func (s *BotMembershipGRPC) ApplyBotInstallationRemoval(ctx context.Context, request *spacev1.ApplyBotInstallationRemovalRequest) (*spacev1.ApplyBotInstallationRemovalResponse, error) {
	proof, err := botmembershipprincipal.RequireGlobalRemoval(ctx, request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "native global Bot removal unavailable")
	}
	store, ok := s.Store.(BotGlobalRemovalStore)
	if !ok {
		return nil, status.Error(codes.Unavailable, "native global Bot removal unavailable")
	}
	receipt, err := store.ApplyBotInstallationRemoval(ctx, proof, request)
	if err != nil {
		return nil, mapBotMembershipError(err)
	}
	if receipt == nil {
		return nil, status.Error(codes.Unavailable, "native global Bot removal unavailable")
	}
	return &spacev1.ApplyBotInstallationRemovalResponse{Receipt: receipt}, nil
}

func (s *BotMembershipGRPC) AddBotMember(ctx context.Context, request *spacev1.AddBotMemberRequest) (*spacev1.AddBotMemberResponse, error) {
	proof, err := botmembershipprincipal.RequireAdd(ctx, request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "native Bot membership unavailable")
	}
	if err = s.Store.AddBotMemberWithInstallation(ctx, proof, request); err != nil {
		return nil, mapBotMembershipError(err)
	}
	return &spacev1.AddBotMemberResponse{}, nil
}
func (s *BotMembershipGRPC) RemoveBotMember(ctx context.Context, request *spacev1.RemoveBotMemberRequest) (*spacev1.RemoveBotMemberResponse, error) {
	proof, err := botmembershipprincipal.RequireRemove(ctx, request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "native Bot membership unavailable")
	}
	if err = s.Store.RemoveBotMemberWithInstallation(ctx, proof, request); err != nil {
		return nil, mapBotMembershipError(err)
	}
	return &spacev1.RemoveBotMemberResponse{}, nil
}
func mapBotMembershipError(err error) error {
	switch {
	case errors.Is(err, store.ErrMemberAdmissionStale), errors.Is(err, store.ErrMemberAdmissionConflict):
		return status.Error(codes.FailedPrecondition, "Bot installation is stale or conflicts with its immutable intent")
	case errors.Is(err, store.ErrMemberAuthorityUnavailable), errors.Is(err, store.ErrEffectiveEpisodeUnavailable):
		return status.Error(codes.Unavailable, "native Bot membership unavailable")
	}
	if status.Code(err) == codes.Unauthenticated || status.Code(err) == codes.PermissionDenied {
		return err
	}
	return mapSpaceStoreError(err)
}
func RegisterBotMembershipServer(server grpc.ServiceRegistrar, service *BotMembershipGRPC) {
	desc := spacev1.SpaceService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range spacev1.SpaceService_ServiceDesc.Methods {
		if method.MethodName == "ReconcileBotPurgeInstallation" || method.MethodName == "RegisterBotGlobalTerminal" || method.MethodName == "GetBotPurgeHandoff" || method.MethodName == "AddBotMember" || method.MethodName == "RemoveBotMember" || method.MethodName == "ApplyBotInstallationRemoval" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, service)
}
