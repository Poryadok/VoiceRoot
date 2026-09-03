package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/guestguard"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

// JoinAccountBlockChecker reports whether two accounts must not co-join a space.
type JoinAccountBlockChecker interface {
	AccountPairBlocked(ctx context.Context, viewerAccountID, otherAccountID uuid.UUID) (bool, error)
}

func (s *SpaceGRPC) JoinSpace(ctx context.Context, req *spacev1.JoinSpaceRequest) (*spacev1.JoinSpaceResponse, error) {
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
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing account")
	}
	if guestguard.IsGuest(ctx) {
		return nil, status.Error(codes.PermissionDenied, "guests cannot join spaces directly")
	}

	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if strings.TrimSpace(row.Visibility) != "public" {
		return nil, status.Error(codes.PermissionDenied, "space is not public")
	}
	reqType := strings.TrimSpace(row.EntryRequirement)
	if reqType != "" && reqType != "none" {
		return nil, status.Error(codes.FailedPrecondition, "space requires invite or approval to join")
	}
	if err := s.ensureJoinNotBlocked(ctx, accountID, row.OwnerProfileID); err != nil {
		return nil, err
	}

	wasMember, err := s.Store.IsSpaceMember(ctx, spaceID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	member, err := s.Store.JoinSpace(ctx, spaceID, profileID, accountID)
	if err != nil {
		return nil, mapInviteStoreErr(err)
	}
	protoMember, err := s.finalizeMembership(ctx, member, !wasMember)
	if err != nil {
		return nil, err
	}
	return &spacev1.JoinSpaceResponse{SpaceMembership: protoMember}, nil
}

func (s *SpaceGRPC) LeaveSpace(ctx context.Context, req *spacev1.LeaveSpaceRequest) (*spacev1.LeaveSpaceResponse, error) {
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
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if row.OwnerProfileID == profileID {
		return nil, status.Error(codes.FailedPrecondition, "space owner cannot leave; transfer ownership first")
	}
	if err := s.Store.RemoveMember(ctx, spaceID, profileID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "member not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.revokeAllMemberRoles(ctx, spaceID, profileID)
	if s.SpaceEvents != nil {
		if pubErr := s.SpaceEvents.PublishMemberLeft(ctx, spaceID.String(), profileID.String()); pubErr != nil {
			logInviteEventFailure(pubErr)
		}
	}
	return &spacev1.LeaveSpaceResponse{}, nil
}

func (s *SpaceGRPC) SyncSpaceProSubscription(ctx context.Context, req *spacev1.SyncSpaceProSubscriptionRequest) (*spacev1.SyncSpaceProSubscriptionResponse, error) {
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
	purchaserID, err := parseUUIDField("purchaser_account_id", req.GetPurchaserAccountId())
	if err != nil {
		return nil, err
	}
	if err := s.Store.SyncSpaceProSubscription(ctx, spaceID, purchaserID, req.GetStatus()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.SyncSpaceProSubscriptionResponse{}, nil
}

func (s *SpaceGRPC) finalizeMembership(ctx context.Context, member *store.MembershipRow, isNew bool) (*spacev1.SpaceMembership, error) {
	if isNew {
		if err := s.assignDefaultMemberRole(ctx, member.SpaceID, member.ProfileID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if s.SpaceEvents != nil {
			if pubErr := s.SpaceEvents.PublishMemberJoined(ctx, member.SpaceID.String(), member.ProfileID.String()); pubErr != nil {
				logInviteEventFailure(pubErr)
			}
		}
	}
	protoMember := membershipRowToProto(member)
	protoMember.RoleNames = s.memberRoleNames(ctx, member.SpaceID, member.ProfileID)
	return protoMember, nil
}

func (s *SpaceGRPC) ensureJoinNotBlocked(ctx context.Context, joinerAccountID, ownerProfileID uuid.UUID) error {
	if s == nil || s.Blocks == nil || s.ProfileAccounts == nil {
		return status.Error(codes.FailedPrecondition, "social block check not configured")
	}
	ownerAccountID, err := s.ProfileAccounts.AccountIDByProfileID(ctx, ownerProfileID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	blocked, err := s.Blocks.AccountPairBlocked(ctx, joinerAccountID, ownerAccountID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if blocked {
		return status.Error(codes.PermissionDenied, "cannot join space due to block")
	}
	return nil
}
