package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

// ProfileAccountLookup resolves profile_id to account_id for ban eviction.
type ProfileAccountLookup interface {
	AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error)
}

func (s *SpaceGRPC) BanMember(ctx context.Context, req *spacev1.BanMemberRequest) (*spacev1.BanMemberResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	accountID, err := parseUUIDField("account_id", req.GetAccountId())
	if err != nil {
		return nil, err
	}
	call, err := lifecycleprincipal.RequireModerationBan(ctx, req)
	if err != nil {
		return nil, err
	}
	actor, err := call.ActorFor(req)
	if err != nil {
		return nil, err
	}
	caller := uuid.MustParse(actor.ProfileID)
	// The store first resolves authenticated completed replay. Only fresh work
	// invokes current Role authorization, outside every Space mutation fence.
	authorize := func(permissionCtx context.Context) error {
		return s.requireSpacePermissionForProfile(permissionCtx, spaceID, caller, permissions.MemberBan)
	}
	var hint *uuid.UUID
	if req.ProfileId != nil {
		id, err := parseUUIDField("profile_id", req.GetProfileId())
		if err != nil {
			return nil, err
		}
		hint = &id
	}
	var reason *string
	if req.Reason != nil {
		value := req.GetReason()
		reason = &value
	}
	err = s.Store.BanAccount(ctx, store.AccountBanInput{SpaceID: spaceID, AccountID: accountID, ActorProfileID: caller, Reason: reason, ProfileHint: hint, InitiatingCall: call, Request: req, Authorize: authorize})
	if errors.Is(err, store.ErrCannotBanOwner) {
		return nil, status.Error(codes.FailedPrecondition, "cannot ban space owner")
	}
	if errors.Is(err, store.ErrAccountBanUnavailable) {
		return nil, status.Error(codes.Unavailable, "complete account ban authority unavailable")
	}
	if err != nil {
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.BanMemberResponse{}, nil
}

func (s *SpaceGRPC) UnbanMember(ctx context.Context, req *spacev1.UnbanMemberRequest) (*spacev1.UnbanMemberResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if _, err := parseUUIDField("account_id", req.GetAccountId()); err != nil {
		return nil, err
	}
	call, err := lifecycleprincipal.RequireModerationUnban(ctx, req)
	if err != nil {
		return nil, err
	}
	actor, err := call.ActorFor(req)
	if err != nil {
		return nil, err
	}
	caller := uuid.MustParse(actor.ProfileID)
	authorize := func(permissionCtx context.Context) error {
		return s.requireSpacePermissionForProfile(permissionCtx, spaceID, caller, permissions.MemberBan)
	}
	if err := s.Store.UnbanAccount(ctx, call, req, authorize); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "ban not found")
		}
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		if errors.Is(err, store.ErrAccountBanUnavailable) {
			return nil, status.Error(codes.Unavailable, "complete account ban authority unavailable")
		}
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.UnbanMemberResponse{}, nil
}

func (s *SpaceGRPC) ListBans(ctx context.Context, req *spacev1.ListBansRequest) (*spacev1.ListBansResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	release, err := s.lockSpaceMutation(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := s.requireSpacePermission(ctx, spaceID, permissions.MemberBan); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListBans(ctx, spaceID)
	if err != nil {
		return nil, mapSpaceStoreError(err)
	}
	out := make([]*spacev1.SpaceBan, 0, len(rows))
	for _, r := range rows {
		b := &spacev1.SpaceBan{
			SpaceId:           r.SpaceID.String(),
			AccountId:         r.AccountID.String(),
			BannedByProfileId: r.BannedByProfileID.String(),
			BannedAt:          timestamppb.New(r.BannedAt),
		}
		if r.Reason != nil {
			b.Reason = r.Reason
		}
		out = append(out, b)
	}
	return &spacev1.ListBansResponse{BanList: &spacev1.BanList{Bans: out}}, nil
}

func (s *SpaceGRPC) TimeoutMember(ctx context.Context, req *spacev1.TimeoutMemberRequest) (*spacev1.TimeoutMemberResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	release, err := s.lockSpaceMutation(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	defer release()
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.ModerationTimeoutMembers); err != nil {
		return nil, err
	}
	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil || row == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if row.OwnerProfileID == profileID {
		return nil, status.Error(codes.FailedPrecondition, "cannot timeout space owner")
	}
	var reason *string
	if req.Reason != nil {
		r := req.GetReason()
		reason = &r
	}
	if err := s.Store.SetMemberTimeout(ctx, spaceID, profileID, caller, req.GetDurationSeconds(), reason); err != nil {
		if errors.Is(err, store.ErrInvalidTimeoutDuration) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.TimeoutMemberResponse{}, nil
}

func (s *SpaceGRPC) RemoveMemberTimeout(ctx context.Context, req *spacev1.RemoveMemberTimeoutRequest) (*spacev1.RemoveMemberTimeoutResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	release, err := s.lockSpaceMutation(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	defer release()
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.ModerationTimeoutMembers); err != nil {
		return nil, err
	}
	if err := s.Store.RemoveMemberTimeout(ctx, spaceID, profileID, caller); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "timeout not found")
		}
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.RemoveMemberTimeoutResponse{}, nil
}
