package grpcsvc

import (
	"context"
	"errors"
	"log"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

func (s *SpaceGRPC) CreateSpace(ctx context.Context, req *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
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
	if err := s.bootstrapSpaceRoles(ctx, row.ID, caller); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.SpaceEvents != nil {
		if err := s.SpaceEvents.PublishSpaceCreated(ctx, row.ID.String(), row.OwnerProfileID.String()); err != nil {
			log.Printf("space: publish space.created: %v", err)
		}
	}
	return &spacev1.CreateSpaceResponse{Space: spaceRowToProto(row)}, nil
}

func (s *SpaceGRPC) UpdateSpace(ctx context.Context, req *spacev1.UpdateSpaceRequest) (*spacev1.UpdateSpaceResponse, error) {
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
	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if row.OwnerProfileID != caller {
		return nil, status.Error(codes.PermissionDenied, "only the space owner can update the space")
	}

	var name, description, iconURL, bannerURL *string
	if req.Name != nil {
		n := strings.TrimSpace(req.GetName())
		name = &n
	}
	if req.Description != nil {
		d := req.GetDescription()
		description = &d
	}
	if req.IconUrl != nil {
		i := req.GetIconUrl()
		iconURL = &i
	}
	if req.BannerUrl != nil {
		b := req.GetBannerUrl()
		bannerURL = &b
	}
	updated, err := s.Store.UpdateSpace(ctx, spaceID, name, description, iconURL, bannerURL)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	return &spacev1.UpdateSpaceResponse{Space: spaceRowToProto(updated)}, nil
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
