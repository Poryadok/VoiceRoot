package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
	"voice/backend/pkg/guestguard"

	spacev1 "voice.app/voice/space/v1"
)

func inviteRowToProto(row *store.InviteRow) *spacev1.Invite {
	if row == nil {
		return nil
	}
	inv := &spacev1.Invite{
		Id:               row.ID.String(),
		SpaceId:          row.SpaceID.String(),
		Code:             row.Code,
		CreatorProfileId: row.CreatorProfileID.String(),
		UseCount:         row.UseCount,
		CreatedAt:        timestamppb.New(row.CreatedAt),
	}
	if row.MaxUses != nil {
		inv.MaxUses = row.MaxUses
	}
	if row.ExpiresAt != nil {
		inv.ExpiresAt = timestamppb.New(*row.ExpiresAt)
	}
	if row.RevokedAt != nil {
		inv.RevokedAt = timestamppb.New(*row.RevokedAt)
	}
	return inv
}

func membershipRowToProto(row *store.MembershipRow) *spacev1.SpaceMembership {
	if row == nil {
		return nil
	}
	m := &spacev1.SpaceMembership{
		SpaceId:   row.SpaceID.String(),
		ProfileId: row.ProfileID.String(),
		JoinedAt:  timestamppb.New(row.JoinedAt),
	}
	if row.Nickname != nil {
		m.Nickname = row.Nickname
	}
	return m
}

func mapInviteStoreErr(err error) error {
	switch {
	case errors.Is(err, store.ErrInviteNotFound):
		return status.Error(codes.NotFound, "invite not found")
	case errors.Is(err, store.ErrInviteRevoked):
		return status.Error(codes.NotFound, "invite not found")
	case errors.Is(err, store.ErrInviteExpired):
		return status.Error(codes.FailedPrecondition, "invite expired")
	case errors.Is(err, store.ErrInviteMaxUses):
		return status.Error(codes.FailedPrecondition, "invite max uses reached")
	case errors.Is(err, store.ErrMemberCapReached):
		return status.Error(codes.ResourceExhausted, "space member cap reached")
	case errors.Is(err, store.ErrAccountBanned):
		return status.Error(codes.PermissionDenied, "account is banned from this space")
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *SpaceGRPC) CreateInvite(ctx context.Context, req *spacev1.CreateInviteRequest) (*spacev1.CreateInviteResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.SpaceManageInvites); err != nil {
		return nil, err
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	var maxUses *int32
	if req.MaxUses != nil {
		maxUses = req.MaxUses
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := req.ExpiresAt.AsTime()
		expiresAt = &t
	}
	row, err := s.Store.CreateInvite(ctx, store.CreateInviteInput{
		SpaceID:          spaceID,
		CreatorProfileID: caller,
		MaxUses:          maxUses,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		if strings.Contains(err.Error(), "max_uses") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.SpaceEvents != nil {
		if pubErr := s.SpaceEvents.PublishInviteCreated(ctx, row.SpaceID.String(), row.Code); pubErr != nil {
			// degraded: invite persisted even when event publish fails
			logInviteEventFailure(pubErr)
		}
	}
	return &spacev1.CreateInviteResponse{Invite: inviteRowToProto(row)}, nil
}

func (s *SpaceGRPC) RevokeInvite(ctx context.Context, req *spacev1.RevokeInviteRequest) (*spacev1.RevokeInviteResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	inviteID, err := parseUUIDField("invite_id", req.GetInviteId())
	if err != nil {
		return nil, err
	}
	inv, err := s.Store.GetInviteByID(ctx, inviteID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if inv == nil {
		return nil, status.Error(codes.NotFound, "invite not found")
	}
	if err := s.requireSpaceOwner(ctx, inv.SpaceID); err != nil {
		return nil, err
	}
	if err := s.Store.RevokeInvite(ctx, inviteID); err != nil {
		return nil, mapInviteStoreErr(err)
	}
	return &spacev1.RevokeInviteResponse{}, nil
}

func (s *SpaceGRPC) GetInvite(ctx context.Context, req *spacev1.GetInviteRequest) (*spacev1.GetInviteResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	code := strings.TrimSpace(req.GetCode())
	if code == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}
	if _, ok := authctx.ProfileID(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	row, err := s.Store.GetInviteByCode(ctx, code)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil || row.RevokedAt != nil {
		return nil, status.Error(codes.NotFound, "invite not found")
	}
	if row.ExpiresAt != nil && !row.ExpiresAt.After(time.Now().UTC()) {
		return nil, status.Error(codes.FailedPrecondition, "invite expired")
	}
	if row.MaxUses != nil && row.UseCount >= *row.MaxUses {
		return nil, status.Error(codes.FailedPrecondition, "invite max uses reached")
	}
	return &spacev1.GetInviteResponse{Invite: inviteRowToProto(row)}, nil
}

func (s *SpaceGRPC) ListInvites(ctx context.Context, req *spacev1.ListInvitesRequest) (*spacev1.ListInvitesResponse, error) {
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
	rows, err := s.Store.ListInvites(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*spacev1.Invite, 0, len(rows))
	for _, row := range rows {
		out = append(out, inviteRowToProto(row))
	}
	return &spacev1.ListInvitesResponse{
		InviteList: &spacev1.InviteList{Invites: out},
	}, nil
}

func (s *SpaceGRPC) JoinByInvite(ctx context.Context, req *spacev1.JoinByInviteRequest) (*spacev1.JoinByInviteResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	code := strings.TrimSpace(req.GetCode())
	if code == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing account")
	}
	if guestguard.IsGuest(ctx) {
		allow, err := s.Store.AllowGuestsForInvite(ctx, code)
		if err != nil {
			return nil, mapInviteStoreErr(err)
		}
		if !allow {
			return nil, status.Error(codes.PermissionDenied, "guests not allowed in this space")
		}
	}
	inv, err := s.Store.GetInviteByCode(ctx, code)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if inv == nil || inv.RevokedAt != nil {
		return nil, status.Error(codes.NotFound, "invite not found")
	}
	if inv.ExpiresAt != nil && !inv.ExpiresAt.After(time.Now().UTC()) {
		return nil, status.Error(codes.FailedPrecondition, "invite expired")
	}
	if inv.MaxUses != nil && inv.UseCount >= *inv.MaxUses {
		return nil, status.Error(codes.FailedPrecondition, "invite max uses reached")
	}
	if err := s.ensureJoinInvitePrivacy(ctx, profileID, inv.CreatorProfileID); err != nil {
		return nil, err
	}
	member, err := s.Store.JoinByInvite(ctx, code, profileID, accountID)
	if err != nil {
		return nil, mapInviteStoreErr(err)
	}
	if err := s.assignDefaultMemberRole(ctx, member.SpaceID, profileID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	protoMember := membershipRowToProto(member)
	protoMember.RoleNames = s.memberRoleNames(ctx, member.SpaceID, profileID)
	return &spacev1.JoinByInviteResponse{SpaceMembership: protoMember}, nil
}

func logInviteEventFailure(err error) {
	_ = err
}
