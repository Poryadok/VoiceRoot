package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
	"voice/backend/space/internal/store"
)

func (s *SpaceGRPC) UpdateMemberNickname(ctx context.Context, req *spacev1.UpdateMemberNicknameRequest) (*spacev1.UpdateMemberNicknameResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "delegated_user" || p.Issuer != "gateway" || p.Audience != "space" || p.RPC != spacev1.SpaceService_UpdateMemberNickname_FullMethodName || p.Subject != p.AccountID || p.SessionEpoch <= 0 {
		return nil, status.Error(codes.Unauthenticated, "verified nickname actor required")
	}
	actor, err := uuid.Parse(p.ProfileID)
	account, accountErr := uuid.Parse(p.AccountID)
	if err != nil || actor == uuid.Nil || actor.String() != p.ProfileID || accountErr != nil || account == uuid.Nil || account.String() != p.AccountID {
		return nil, status.Error(codes.Unauthenticated, "invalid nickname actor")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != p.RequestHash {
		return nil, status.Error(codes.Unauthenticated, "nickname request binding mismatch")
	}
	if req == nil || req.Nickname == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "nickname presence required")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	target, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if spaceID == uuid.Nil || target == uuid.Nil || spaceID.String() != req.SpaceId || target.String() != req.ProfileId {
		return nil, status.Error(codes.InvalidArgument, "canonical nickname IDs required")
	}
	if _, err = store.CanonicalMemberNickname(req.GetNickname()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid nickname")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "space persistence unavailable")
	}
	release, err := s.lockSpaceMutation(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	defer release()
	member, err := s.Store.IsSpaceMember(ctx, spaceID, actor)
	if err != nil {
		return nil, mapSpaceStoreError(err)
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a space member")
	}
	if err = s.requireNicknamePermission(ctx, spaceID, actor); err != nil {
		return nil, err
	}
	row, err := s.Store.UpdateMemberNickname(ctx, spaceID, target, actor, req.GetNickname())
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, status.Error(codes.NotFound, "member not found")
	case errors.Is(err, store.ErrNicknameOwnerProtected), errors.Is(err, store.ErrNicknameExternallyManaged):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, store.ErrNicknameActorNotMember):
		return nil, status.Error(codes.PermissionDenied, "not a space member")
	case err != nil:
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.UpdateMemberNicknameResponse{SpaceMembership: membershipRowToProto(row)}, nil
}

func (s *SpaceGRPC) requireNicknamePermission(ctx context.Context, spaceID, actor uuid.UUID) error {
	if s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "signed nickname permission dependency unavailable")
	}
	req := &rolev1.CheckPermissionRequest{SpaceId: spaceID.String(), ProfileId: actor.String(), PermissionName: permissions.MemberManageNicknames}
	signed, err := s.ownershipRoleContext(ctx, req, rolev1.RoleService_CheckPermission_FullMethodName, uuid.New())
	if err != nil {
		return err
	}
	resp, err := s.OwnershipRoles.CheckPermission(signed, req)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return status.Error(codes.PermissionDenied, "nickname permission denied")
		}
		return status.Error(codes.Unavailable, "nickname permission unavailable")
	}
	if !resp.GetAllowed() {
		return status.Error(codes.PermissionDenied, "nickname permission denied")
	}
	return nil
}
