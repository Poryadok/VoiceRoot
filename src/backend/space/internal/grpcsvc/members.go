package grpcsvc

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

func (s *SpaceGRPC) ListMembers(ctx context.Context, req *spacev1.ListMembersRequest) (*spacev1.ListMembersResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceMember(ctx, spaceID); err != nil {
		return nil, err
	}
	pageSize := int32(50)
	cursor := ""
	if req.GetPage() != nil {
		if req.GetPage().GetPageSize() > 0 {
			pageSize = req.GetPage().GetPageSize()
		}
		cursor = req.GetPage().GetCursor()
	}
	rows, next, err := s.Store.ListSpaceMembersPage(ctx, spaceID, pageSize, cursor)
	if err != nil {
		if err == store.ErrInvalidListCursor {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	members := make([]*spacev1.SpaceMembership, 0, len(rows))
	for _, row := range rows {
		m := membershipRowToProto(row)
		m.RoleNames = s.memberRoleNames(ctx, spaceID, row.ProfileID)
		if len(m.RoleNames) == 0 && s.Roles == nil {
			if row.ProfileID.String() == "" {
				continue
			}
			spaceRow, _ := s.Store.GetSpace(ctx, spaceID)
			if spaceRow != nil && spaceRow.OwnerProfileID == row.ProfileID {
				m.RoleNames = []string{permissions.RoleOwner}
			}
		}
		members = append(members, m)
	}
	return &spacev1.ListMembersResponse{
		SpaceMemberList: &spacev1.SpaceMemberList{
			Members:    members,
			NextCursor: next,
		},
	}, nil
}

func (s *SpaceGRPC) KickMember(ctx context.Context, req *spacev1.KickMemberRequest) (*spacev1.KickMemberResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.MemberKick); err != nil {
		return nil, err
	}
	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil || row == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if row.OwnerProfileID == profileID {
		return nil, status.Error(codes.FailedPrecondition, "cannot kick space owner")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if err := s.Store.RemoveMember(ctx, spaceID, profileID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "member not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.revokeAllMemberRoles(ctx, spaceID, profileID)
	_ = s.Store.RecordMemberKicked(ctx, spaceID, profileID, caller)
	return &spacev1.KickMemberResponse{}, nil
}
