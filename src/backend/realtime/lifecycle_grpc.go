package main

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
	realtimev1 "voice.app/voice/realtime/v1"
	"voice/backend/pkg/principal"
	"voice/backend/realtime/internal/lifecycle"
)

type realtimeLifecycleGRPC struct {
	realtimev1.UnimplementedRealtimeLifecycleServiceServer
	store *lifecycle.Store
}

func requireLifecycleCaller(ctx context.Context, request proto.Message, method, operation string) error {
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(request)
	if !ok || err != nil || p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "realtime" || p.RPC != method || p.RequestID != operation || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "request-bound Space principal required")
	}
	return nil
}
func lifecycleRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, lifecycle.ErrConflict) {
		return status.Error(codes.Aborted, "Realtime lifecycle binding conflict")
	}
	if errors.Is(err, lifecycle.ErrPending) {
		return status.Error(codes.Unavailable, "Realtime lifecycle admissions remain pending")
	}
	return status.Error(codes.Unavailable, "Realtime lifecycle unavailable")
}
func (s *realtimeLifecycleGRPC) ImportSpacePurgeManifestPage(ctx context.Context, req *realtimev1.ImportSpacePurgeManifestPageRequest) (*realtimev1.ImportSpacePurgeManifestPageResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	if err := requireLifecycleCaller(ctx, req, realtimev1.RealtimeLifecycleService_ImportSpacePurgeManifestPage_FullMethodName, req.GetDeletionOperationId()); err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, lifecycleRPCError(errors.New("no store"))
	}
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	response, err := s.store.Import(call, req)
	return response, lifecycleRPCError(err)
}
func (s *realtimeLifecycleGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *realtimev1.ApplySpaceLifecycleFenceRequest) (*realtimev1.ApplySpaceLifecycleFenceResponse, error) {
	if req == nil || req.GetFence() == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	if err := requireLifecycleCaller(ctx, req, realtimev1.RealtimeLifecycleService_ApplySpaceLifecycleFence_FullMethodName, req.GetFence().GetDeletionOperationId()); err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, lifecycleRPCError(errors.New("no store"))
	}
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	response, err := s.store.Apply(call, req)
	return response, lifecycleRPCError(err)
}
func (s *realtimeLifecycleGRPC) PurgeSpace(ctx context.Context, req *realtimev1.PurgeSpaceRequest) (*realtimev1.PurgeSpaceResponse, error) {
	if req == nil || req.GetPurge() == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	if err := requireLifecycleCaller(ctx, req, realtimev1.RealtimeLifecycleService_PurgeSpace_FullMethodName, req.GetPurge().GetDeletionOperationId()); err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, lifecycleRPCError(errors.New("no store"))
	}
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	response, err := s.store.Purge(call, req)
	return response, lifecycleRPCError(err)
}
