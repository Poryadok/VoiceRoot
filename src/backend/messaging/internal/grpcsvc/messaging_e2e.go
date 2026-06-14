package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/store"

	messagingv1 "voice.app/voice/messaging/v1"
)

func (s *MessagingGRPC) UploadPreKeyBundle(ctx context.Context, req *messagingv1.UploadPreKeyBundleRequest) (*messagingv1.UploadPreKeyBundleResponse, error) {
	if s == nil || s.PreKeyBundles == nil {
		return nil, status.Error(codes.FailedPrecondition, "e2e pre-key store not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	bundle := strings.TrimSpace(req.GetBundle())
	if bundle == "" {
		return nil, status.Error(codes.InvalidArgument, "bundle is required")
	}
	if err := s.PreKeyBundles.UpsertBundle(ctx, profileID, bundle); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &messagingv1.UploadPreKeyBundleResponse{}, nil
}

func (s *MessagingGRPC) GetPreKeyBundle(ctx context.Context, req *messagingv1.GetPreKeyBundleRequest) (*messagingv1.GetPreKeyBundleResponse, error) {
	if s == nil || s.PreKeyBundles == nil {
		return nil, status.Error(codes.FailedPrecondition, "e2e pre-key store not configured")
	}
	if _, ok := authctx.ProfileID(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	targetID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	bundle, err := s.PreKeyBundles.GetBundle(ctx, targetID)
	if errors.Is(err, store.ErrPreKeyBundleNotFound) {
		return nil, status.Error(codes.NotFound, "pre-key bundle not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &messagingv1.GetPreKeyBundleResponse{Bundle: bundle}, nil
}

func (s *MessagingGRPC) validateE2ESend(ctx context.Context, chatID uuid.UUID, isE2E bool) error {
	if !isE2E {
		return nil
	}
	if s.ChatThreadPolicy == nil {
		return status.Error(codes.FailedPrecondition, "chat policy not configured")
	}
	pol, err := s.ChatThreadPolicy.Load(ctx, chatID)
	if errors.Is(err, store.ErrChatNotFound) {
		return status.Error(codes.NotFound, "chat not found")
	}
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if pol.ChatType != "dm" {
		return status.Error(codes.FailedPrecondition, "e2e is dm-only")
	}
	if !pol.E2EEnabled {
		return status.Error(codes.FailedPrecondition, "e2e not enabled for chat")
	}
	return nil
}
