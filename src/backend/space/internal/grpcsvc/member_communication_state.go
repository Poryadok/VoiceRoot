package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

func (s *SpaceGRPC) ResolveMemberCommunicationState(ctx context.Context, req *spacev1.ResolveMemberCommunicationStateRequest) (*spacev1.ResolveMemberCommunicationStateResponse, error) {
	identity, ok := authctx.VerifiedServiceIdentity(ctx)
	if !ok || identity != authctx.ServiceIdentityMessaging {
		return nil, status.Error(codes.Unauthenticated, "verified Messaging service identity required")
	}
	if req.GetProtocolVersion() != 1 {
		return nil, status.Error(codes.InvalidArgument, "unsupported communication protocol version")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if s == nil || (s.Store == nil && s.memberCommunicationResolver == nil) {
		return nil, status.Error(codes.Unavailable, "communication authority unavailable")
	}
	resolver := memberCommunicationResolver(s.Store)
	if s.memberCommunicationResolver != nil {
		resolver = s.memberCommunicationResolver
	}
	row, err := resolver.ResolveMemberCommunicationState(ctx, spaceID, profileID)
	if errors.Is(err, store.ErrMemberNotFound) {
		return nil, status.Error(codes.NotFound, "space member not found")
	}
	if err != nil || row == nil {
		return nil, status.Error(codes.Unavailable, "communication authority unavailable")
	}
	response := &spacev1.ResolveMemberCommunicationStateResponse{
		ProtocolVersion: 1, SpaceId: row.SpaceID.String(), ProfileId: row.ProfileID.String(),
		AccessEpoch: row.AccessEpoch, EvaluatedAt: timestamppb.New(row.EvaluatedAt),
		CommunicationState: spacev1.MemberCommunicationState_MEMBER_COMMUNICATION_STATE_CLEAR,
	}
	if row.TimedOutUntil != nil {
		response.CommunicationState = spacev1.MemberCommunicationState_MEMBER_COMMUNICATION_STATE_TIMED_OUT
		response.TimedOutUntil = timestamppb.New(*row.TimedOutUntil)
	}
	return response, nil
}
