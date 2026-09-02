package grpcsvc

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
	"voice/backend/pkg/guestguard"

	spacev1 "voice.app/voice/space/v1"
)

func (s *SpaceGRPC) CreateSpace(ctx context.Context, req *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
	if err := guestguard.RequireRegular(ctx); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	row, err := s.Store.CreateSpace(ctx, caller, name, req.GetDescription(), req.GetVisibility())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if accountID, ok := authctx.AccountID(ctx); ok && s.SeedSpaceProActive {
		_ = s.Store.UpsertSpaceSubscription(ctx, row.ID, accountID, "active")
	}
	if err := s.bootstrapSpaceRoles(ctx, row.ID, caller); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.SpaceEvents != nil {
		if err := s.SpaceEvents.PublishSpaceCreated(ctx, row.ID.String(), row.OwnerProfileID.String()); err != nil {
			s.logPublishError(ctx, "space.created", err, slog.String("space_id", row.ID.String()))
		}
	}
	return &spacev1.CreateSpaceResponse{Space: spaceRowToProto(row)}, nil
}

func (s *SpaceGRPC) UpdateSpace(ctx context.Context, req *spacev1.UpdateSpaceRequest) (*spacev1.UpdateSpaceResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.SpaceManageSettings); err != nil {
		return nil, err
	}

	var in store.UpdateSpaceInput
	if req.Name != nil {
		n := strings.TrimSpace(req.GetName())
		in.Name = &n
	}
	if req.Description != nil {
		d := req.GetDescription()
		in.Description = &d
	}
	if req.IconUrl != nil {
		i := req.GetIconUrl()
		in.IconURL = &i
	}
	if req.BannerUrl != nil {
		b := req.GetBannerUrl()
		in.BannerURL = &b
	}
	if req.Visibility != nil {
		v := req.GetVisibility()
		in.Visibility = &v
	}
	if req.EntryRequirement != nil {
		e := req.GetEntryRequirement()
		in.EntryRequirement = &e
	}
	if req.EntryQuestionsJson != nil {
		q := req.GetEntryQuestionsJson()
		in.EntryQuestionsJSON = &q
	}
	if req.MmConfigJson != nil {
		m := req.GetMmConfigJson()
		in.MMConfigJSON = &m
	}
	updated, err := s.Store.UpdateSpace(ctx, spaceID, in)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if s.SpaceEvents != nil {
		if pubErr := s.SpaceEvents.PublishSpaceUpdated(ctx, spaceID.String()); pubErr != nil {
			s.logPublishError(ctx, "space.updated", pubErr, slog.String("space_id", spaceID.String()))
		}
	}
	return &spacev1.UpdateSpaceResponse{Space: spaceRowToProto(updated)}, nil
}

func (s *SpaceGRPC) UpdateSpaceMmConfig(ctx context.Context, req *spacev1.UpdateSpaceMmConfigRequest) (*spacev1.UpdateSpaceMmConfigResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.SpaceManageSettings); err != nil {
		return nil, err
	}
	cfg := req.GetMmConfigJson()
	updated, err := s.Store.UpdateSpace(ctx, spaceID, store.UpdateSpaceInput{MMConfigJSON: &cfg})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if s.SpaceEvents != nil {
		if pubErr := s.SpaceEvents.PublishSpaceUpdated(ctx, spaceID.String()); pubErr != nil {
			s.logPublishError(ctx, "space.updated", pubErr, slog.String("space_id", spaceID.String()))
		}
	}
	return &spacev1.UpdateSpaceMmConfigResponse{Space: spaceRowToProto(updated)}, nil
}

func (s *SpaceGRPC) DeleteSpace(ctx context.Context, req *spacev1.DeleteSpaceRequest) (*spacev1.DeleteSpaceResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteSpace(ctx, spaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "space not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.SpaceEvents != nil {
		if pubErr := s.SpaceEvents.PublishSpaceDeleted(ctx, spaceID.String()); pubErr != nil {
			s.logPublishError(ctx, "space.deleted", pubErr, slog.String("space_id", spaceID.String()))
		}
	}
	return &spacev1.DeleteSpaceResponse{}, nil
}

func (s *SpaceGRPC) TransferOwnership(ctx context.Context, req *spacev1.TransferOwnershipRequest) (*spacev1.TransferOwnershipResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	newOwnerID, err := parseUUIDField("new_owner_profile_id", req.GetNewOwnerProfileId())
	if err != nil {
		return nil, err
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	if err := s.Store.TransferOwnership(ctx, spaceID, caller, newOwnerID); err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, status.Error(codes.NotFound, "space not found")
		case errors.Is(err, store.ErrTransferToSelf):
			return nil, status.Error(codes.FailedPrecondition, "cannot transfer ownership to current owner")
		case errors.Is(err, store.ErrMemberNotFound):
			return nil, status.Error(codes.FailedPrecondition, "new owner must be a space member")
		case errors.Is(err, store.ErrNotSpaceOwner):
			return nil, status.Error(codes.PermissionDenied, "space owner required")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if err := s.reassignOwnerRole(ctx, spaceID, caller, newOwnerID); err != nil {
		if rbErr := s.Store.TransferOwnership(ctx, spaceID, newOwnerID, caller); rbErr != nil {
			return nil, status.Errorf(codes.Internal, "owner role reassignment failed (%v); ownership rollback failed: %v", err, rbErr)
		}
		switch status.Code(err) {
		case codes.Unavailable:
			return nil, status.Error(codes.Unavailable, "role service unavailable")
		case codes.FailedPrecondition:
			return nil, err
		default:
			return nil, status.Errorf(codes.Internal, "owner role reassignment failed: %v", err)
		}
	}
	if s.SpaceEvents != nil {
		if pubErr := s.SpaceEvents.PublishSpaceUpdated(ctx, spaceID.String()); pubErr != nil {
			s.logPublishError(ctx, "space.updated", pubErr, slog.String("space_id", spaceID.String()))
		}
	}
	return &spacev1.TransferOwnershipResponse{}, nil
}

func (s *SpaceGRPC) GetSpace(ctx context.Context, req *spacev1.GetSpaceRequest) (*spacev1.GetSpaceResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	member, err := s.Store.IsSpaceMember(ctx, spaceID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a space member")
	}
	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	return &spacev1.GetSpaceResponse{Space: spaceRowToProto(row)}, nil
}

func (s *SpaceGRPC) ListMySpaces(ctx context.Context, req *spacev1.ListMySpacesRequest) (*spacev1.ListMySpacesResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}

	limit := 50
	cursor := ""
	if req != nil {
		if p := req.GetPage(); p != nil {
			cursor = p.GetCursor()
			if ps := int(p.GetPageSize()); ps > 0 {
				limit = ps
			}
		}
	}
	if limit > 100 {
		limit = 100
	}

	page, err := s.Store.ListMySpacesPage(ctx, caller, cursor, limit)
	if errors.Is(err, store.ErrInvalidListCursor) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	spaces := make([]*spacev1.Space, 0, len(page.Rows))
	for _, row := range page.Rows {
		spaces = append(spaces, spaceRowToProto(row))
	}
	return &spacev1.ListMySpacesResponse{
		SpaceList: &spacev1.SpaceList{
			Spaces:     spaces,
			NextCursor: page.NextCursor,
		},
	}, nil
}
