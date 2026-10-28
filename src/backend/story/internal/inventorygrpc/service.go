// Package inventorygrpc serves the private, protective Story inventory to File.
package inventorygrpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/referenceinventory"
)

const (
	BeginMethod   = storyv1.StoryReferenceInventoryService_BeginReferenceInventory_FullMethodName
	PageMethod    = storyv1.StoryReferenceInventoryService_ReadReferenceInventoryPage_FullMethodName
	ChangesMethod = storyv1.StoryReferenceInventoryService_ReadReferenceInventoryChanges_FullMethodName
	SealMethod    = storyv1.StoryReferenceInventoryService_SealReferenceInventory_FullMethodName
)

// Store reads immutable owner snapshots and complete committed journal revisions.
// It does not supply writer coverage or global File activation authority.
type Store interface {
	BeginReferenceInventory(context.Context, *commonv1.BeginReferenceInventoryRequest) (*commonv1.ReferenceInventoryDescriptor, error)
	ReadReferenceInventoryPage(context.Context, *commonv1.ReferenceInventoryIdentity, uint32) (*commonv1.ReferenceInventoryPage, error)
	ReadReferenceInventoryChanges(context.Context, *commonv1.ReferenceInventoryIdentity, uint64, uint32) (*commonv1.ReadReferenceInventoryChangesResponse, error)
}

type Service struct {
	storyv1.UnimplementedStoryReferenceInventoryServiceServer
	store Store
}

func New(store Store) *Service { return &Service{store: store} }

func allowedMethod(method string) bool {
	switch method {
	case BeginMethod, PageMethod, ChangesMethod, SealMethod:
		return true
	}
	return false
}

func requireFile(ctx context.Context, method string, req proto.Message) error {
	if referenceinventory.Strict(req) != nil {
		return status.Error(codes.InvalidArgument, "invalid reference inventory request")
	}
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(req)
	if !ok || err != nil || p.Kind != "service" || p.Issuer != "file" || p.Subject != "service:file" || p.Audience != "story" || p.RPC != method || !referenceinventory.UUID(p.RequestID) || p.RequestHash != hash || !p.ExpiresAt.After(time.Now()) || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return status.Error(codes.Unauthenticated, "invalid File inventory principal")
	}
	return nil
}

func invalidIdentity(id *commonv1.ReferenceInventoryIdentity) error {
	if referenceinventory.ValidateIdentity(id) != nil {
		return status.Error(codes.InvalidArgument, "invalid reference inventory identity")
	}
	return nil
}

func storeError(err error) error {
	if errors.Is(err, referenceinventory.ErrConflict) {
		return status.Error(codes.FailedPrecondition, "reference inventory conflict")
	}
	if errors.Is(err, referenceinventory.ErrInvalid) {
		return status.Error(codes.InvalidArgument, "invalid reference inventory request")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "reference inventory canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "reference inventory deadline exceeded")
	}
	return status.Error(codes.Unavailable, "reference inventory unavailable")
}

func (s *Service) BeginReferenceInventory(ctx context.Context, req *commonv1.BeginReferenceInventoryRequest) (*commonv1.BeginReferenceInventoryResponse, error) {
	if err := requireFile(ctx, BeginMethod, req); err != nil {
		return nil, err
	}
	if err := invalidIdentity(req.Identity); err != nil {
		return nil, err
	}
	if !referenceinventory.UUID(req.OperationId) {
		return nil, status.Error(codes.InvalidArgument, "invalid inventory operation")
	}
	if s == nil || s.store == nil {
		return nil, storeError(nil)
	}
	descriptor, err := s.store.BeginReferenceInventory(ctx, req)
	if err != nil {
		return nil, storeError(err)
	}
	if referenceinventory.ValidateDescriptor(descriptor) != nil || !proto.Equal(descriptor.GetManifest().GetIdentity(), req.Identity) {
		return nil, storeError(nil)
	}
	return &commonv1.BeginReferenceInventoryResponse{Descriptor_: descriptor}, nil
}

func (s *Service) ReadReferenceInventoryPage(ctx context.Context, req *commonv1.ReadReferenceInventoryPageRequest) (*commonv1.ReadReferenceInventoryPageResponse, error) {
	if err := requireFile(ctx, PageMethod, req); err != nil {
		return nil, err
	}
	if err := invalidIdentity(req.Identity); err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, storeError(nil)
	}
	page, err := s.store.ReadReferenceInventoryPage(ctx, req.Identity, req.PageIndex)
	if err != nil {
		return nil, storeError(err)
	}
	if referenceinventory.ValidateStoryPage(page) != nil || !proto.Equal(page.Identity, req.Identity) || page.PageIndex != req.PageIndex {
		return nil, storeError(nil)
	}
	return &commonv1.ReadReferenceInventoryPageResponse{Page: page}, nil
}

func (s *Service) ReadReferenceInventoryChanges(ctx context.Context, req *commonv1.ReadReferenceInventoryChangesRequest) (*commonv1.ReadReferenceInventoryChangesResponse, error) {
	if err := requireFile(ctx, ChangesMethod, req); err != nil {
		return nil, err
	}
	if err := invalidIdentity(req.Identity); err != nil {
		return nil, err
	}
	if req.Limit == 0 || req.Limit > referenceinventory.PageSize {
		return nil, status.Error(codes.InvalidArgument, "invalid inventory change limit")
	}
	if s == nil || s.store == nil {
		return nil, storeError(nil)
	}
	changes, err := s.store.ReadReferenceInventoryChanges(ctx, req.Identity, req.AfterRevision, req.Limit)
	if err != nil {
		return nil, storeError(err)
	}
	if referenceinventory.ValidateChanges(changes, req.Identity, req.AfterRevision) != nil || len(changes.Revisions) > int(req.Limit) {
		return nil, storeError(nil)
	}
	return changes, nil
}

func (s *Service) SealReferenceInventory(ctx context.Context, req *commonv1.SealReferenceInventoryRequest) (*commonv1.SealReferenceInventoryResponse, error) {
	if err := requireFile(ctx, SealMethod, req); err != nil {
		return nil, err
	}
	if err := invalidIdentity(req.Identity); err != nil {
		return nil, err
	}
	if !referenceinventory.UUID(req.OperationId) {
		return nil, status.Error(codes.InvalidArgument, "invalid inventory operation")
	}
	return nil, status.Error(codes.FailedPrecondition, "Story writer coverage authority unavailable")
}
