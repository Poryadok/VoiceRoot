package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/memberadmissionrole"
	"voice/backend/space/internal/store"
)

func (s *PublicLifecycleSpace) JoinSpace(ctx context.Context, req *spacev1.JoinSpaceRequest) (*spacev1.JoinSpaceResponse, error) {
	call, err := lifecycleprincipal.RequireAdmissionJoinSpace(ctx, req)
	if err != nil {
		return nil, err
	}
	member, err := s.joinNonInteractive(ctx, req, call)
	if err != nil {
		return nil, err
	}
	return &spacev1.JoinSpaceResponse{SpaceMembership: member}, nil
}

func (s *PublicLifecycleSpace) JoinByInvite(ctx context.Context, req *spacev1.JoinByInviteRequest) (*spacev1.JoinByInviteResponse, error) {
	call, err := lifecycleprincipal.RequireAdmissionJoinInvite(ctx, req)
	if err != nil {
		return nil, err
	}
	member, err := s.joinNonInteractive(ctx, req, call)
	if err != nil {
		return nil, err
	}
	return &spacev1.JoinByInviteResponse{SpaceMembership: member}, nil
}

func (s *PublicLifecycleSpace) joinNonInteractive(ctx context.Context, request proto.Message, call *lifecycleprincipal.VerifiedAdmissionCall) (*spacev1.SpaceMembership, error) {
	if s == nil || s.SpaceGRPC == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "space persistence unavailable")
	}
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	remaining, err := call.Remaining(request)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	input := store.MemberAdmissionInput{OperationID: uuid.New(), AccountID: uuid.MustParse(actor.AccountID), ProfileID: uuid.MustParse(actor.ProfileID), Guest: actor.AccountType == "guest"}
	switch req := request.(type) {
	case *spacev1.JoinSpaceRequest:
		input.SpaceID, err = parseUUIDField("space_id", req.GetSpaceId())
		if err != nil {
			return nil, err
		}
		if input.Guest {
			return nil, status.Error(codes.PermissionDenied, "guests cannot join spaces directly")
		}
		space, err := s.Store.GetSpace(ctx, input.SpaceID)
		if err != nil {
			return nil, mapSpaceStoreError(err)
		}
		if space == nil {
			return nil, status.Error(codes.NotFound, "space not found")
		}
		if strings.TrimSpace(space.Visibility) != "public" {
			return nil, status.Error(codes.PermissionDenied, "space is not public")
		}
		if requirement := strings.TrimSpace(space.EntryRequirement); requirement != "" && requirement != "none" {
			return nil, status.Error(codes.FailedPrecondition, "space requires invite or approval to join")
		}
		if err = s.ensureJoinNotBlocked(ctx, input.AccountID, space.OwnerProfileID); err != nil {
			return nil, err
		}
	case *spacev1.JoinByInviteRequest:
		code := strings.TrimSpace(req.GetCode())
		if code == "" {
			return nil, status.Error(codes.InvalidArgument, "code is required")
		}
		invite, err := s.Store.GetInviteByCode(ctx, code)
		if err != nil {
			return nil, mapSpaceStoreError(err)
		}
		if invite == nil || invite.RevokedAt != nil {
			return nil, status.Error(codes.NotFound, "invite not found")
		}
		if invite.ExpiresAt != nil && !invite.ExpiresAt.After(time.Now().UTC()) {
			return nil, mapInviteStoreErr(store.ErrInviteExpired)
		}
		if invite.MaxUses != nil && invite.UseCount >= *invite.MaxUses {
			return nil, mapInviteStoreErr(store.ErrInviteMaxUses)
		}
		input.SpaceID, input.InviteID = invite.SpaceID, &invite.ID
		if err = s.ensureJoinInvitePrivacyForActor(ctx, input.ProfileID, invite.CreatorProfileID, input.Guest); err != nil {
			return nil, err
		}
		if err = s.ensureJoinNotBlocked(ctx, input.AccountID, invite.CreatorProfileID); err != nil {
			return nil, err
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported admission request")
	}
	banned, err := s.Store.IsAccountBanned(ctx, input.SpaceID, input.AccountID)
	if err != nil {
		return nil, mapSpaceStoreError(err)
	}
	if banned {
		return nil, mapInviteStoreErr(store.ErrAccountBanned)
	}
	member, err := s.Store.GetMembership(ctx, input.SpaceID, input.ProfileID)
	if err == nil {
		if _, err = call.ActorFor(request); err != nil {
			return nil, err
		}
		return membershipRowToProto(member), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, mapSpaceStoreError(err)
	}
	intent, err := s.Store.FindCurrent(ctx, input.SpaceID, input.ProfileID)
	if errors.Is(err, pgx.ErrNoRows) {
		intent, err = s.Store.PrepareMemberAdmission(ctx, input)
	}
	if err != nil {
		return nil, mapPublicAdmissionError(err)
	}
	// A retry may continue only its exact saved candidate, never another account,
	// invite, direct-join kind or interactive decision.
	old := intent.Input
	if old.AccountID != input.AccountID || old.Guest != input.Guest || old.JoinAttempt != nil || len(old.DecisionBasisSHA256) != 0 ||
		(old.InviteID == nil) != (input.InviteID == nil) || (old.InviteID != nil && *old.InviteID != *input.InviteID) {
		return nil, mapPublicAdmissionError(store.ErrMemberAdmissionConflict)
	}
	member, err = s.Store.CompleteNonInteractiveMemberAdmission(ctx, old.OperationID,
		&memberadmissionrole.Client{Role: s.OwnershipRoles, Issuer: s.PrincipalIssuer}, call, request)
	if err != nil {
		return nil, mapPublicAdmissionError(err)
	}
	return membershipRowToProto(member), nil
}

func mapPublicAdmissionError(err error) error {
	if errors.Is(err, memberadmissionrole.ErrUnavailable) {
		return status.Error(codes.Unavailable, "default member role preparation unavailable")
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return status.Error(codes.Unavailable, "admission observation expired")
	}
	if errors.Is(err, store.ErrMemberAdmissionConflict) || errors.Is(err, store.ErrMemberAdmissionStale) {
		return status.Error(codes.FailedPrecondition, "admission candidate conflicts with current request")
	}
	return mapInviteStoreErr(err)
}
