package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/socialprincipal"
)

func (s *SpaceGRPC) AreCoMembers(ctx context.Context, req *spacev1.AreCoMembersRequest) (*spacev1.AreCoMembersResponse, error) {
	if _, err := socialprincipal.CheckDomain(ctx, "space", req); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	profileA, err := parseUUIDField("profile_id_a", req.GetProfileIdA())
	if err != nil {
		return nil, err
	}
	profileB, err := parseUUIDField("profile_id_b", req.GetProfileIdB())
	if err != nil {
		return nil, err
	}
	var spaceIDs []uuid.UUID
	for _, sid := range req.GetSpaceIds() {
		id, parseErr := parseUUIDField("space_ids", sid)
		if parseErr != nil {
			return nil, parseErr
		}
		spaceIDs = append(spaceIDs, id)
	}
	if s.ProfileAccounts == nil {
		return nil, status.Error(codes.Unavailable, "profile ownership lookup unavailable")
	}
	accountA, err := s.ProfileAccounts.AccountIDByProfileID(ctx, profileA)
	if err != nil || accountA == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "profile ownership lookup unavailable")
	}
	accountB, err := s.ProfileAccounts.AccountIDByProfileID(ctx, profileB)
	if err != nil || accountB == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "profile ownership lookup unavailable")
	}
	ok, err := s.Store.AreCoMembersWithAccountBans(ctx, profileA, profileB, []uuid.UUID{accountA, accountB}, spaceIDs)
	if err != nil {
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.AreCoMembersResponse{CoMembers: ok}, nil
}
