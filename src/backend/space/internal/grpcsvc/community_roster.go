package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

func (s *SpaceGRPC) ApplyCommunityRoster(ctx context.Context, req *spacev1.ApplyCommunityRosterRequest) (*spacev1.ApplyCommunityRosterResponse, error) {
	identity, ok := authctx.VerifiedServiceIdentity(ctx)
	if !ok || identity != authctx.ServiceIdentityGameIntegration {
		return nil, status.Error(codes.Unauthenticated, "verified GIS service identity required")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Space roster authority unavailable")
	}
	parse := func(field, raw string) (uuid.UUID, error) { return parseUUIDField(field, raw) }
	var in store.CommunityRosterInput
	var err error
	if in.OperationID, err = parse("operation_id", req.GetOperationId()); err != nil {
		return nil, err
	}
	if in.ApplicationID, err = parse("application_id", req.GetApplicationId()); err != nil {
		return nil, err
	}
	if in.EnvironmentID, err = parse("environment_id", req.GetEnvironmentId()); err != nil {
		return nil, err
	}
	if in.SpaceID, err = parse("space_id", req.GetSpaceId()); err != nil {
		return nil, err
	}
	in.CorporationKey, in.OwnerGeneration, in.SourceRevision = req.GetCorporationKey(), req.GetOwnerGeneration(), req.GetSourceRevision()
	if req.GetLeaseExpiresAt() == nil || !req.GetLeaseExpiresAt().IsValid() {
		return nil, status.Error(codes.InvalidArgument, "valid roster lease expiry required")
	}
	in.LeaseExpiresAt = req.GetLeaseExpiresAt().AsTime()
	in.SnapshotSHA256 = append([]byte(nil), req.GetSnapshotSha256()...)
	if len(in.SnapshotSHA256) != 32 || in.OwnerGeneration <= 0 || in.SourceRevision <= 0 || in.CorporationKey == "" || len(in.CorporationKey) > 256 {
		return nil, status.Error(codes.InvalidArgument, "invalid community roster fence")
	}
	in.ProfileIDs = make([]uuid.UUID, len(req.GetProfileIds()))
	for index, raw := range req.GetProfileIds() {
		if in.ProfileIDs[index], err = parse("profile_id", raw); err != nil {
			return nil, err
		}
	}
	receipt, err := s.Store.ApplyCommunityRoster(ctx, in)
	if err != nil {
		if errors.Is(err, store.ErrCommunityRosterConflict) {
			return nil, status.Error(codes.FailedPrecondition, "community roster revision or owner generation conflicts")
		}
		return nil, status.Error(codes.Unavailable, "Space roster projection persistence unavailable")
	}
	if receipt.Current {
		for _, profileID := range receipt.ProfileIDs {
			if err := s.ensureCommunityMemberRole(ctx, in.SpaceID, profileID); err != nil {
				return nil, status.Error(codes.Unavailable, "Space baseline member Role projection unavailable")
			}
		}
		for _, profileID := range receipt.RemovedProfileIDs {
			if err := s.revokeCommunityMemberRole(ctx, in.SpaceID, profileID); err != nil {
				return nil, status.Error(codes.Unavailable, "Space baseline member Role revocation unavailable")
			}
		}
	}
	return &spacev1.ApplyCommunityRosterResponse{OperationId: receipt.OperationID.String(), SpaceId: receipt.SpaceID.String(),
		OwnerGeneration: receipt.OwnerGeneration, SourceRevision: receipt.SourceRevision, SnapshotSha256: receipt.SnapshotSHA256,
		ReceiptId: receipt.ReceiptID.String(), MemberCount: receipt.MemberCount, Replayed: receipt.Replayed, RequestSha256: receipt.RequestSHA256}, nil
}
