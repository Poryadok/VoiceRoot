package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/internal/store"
)

type voiceRoomGrantDecisionStore interface {
	ResolveVoiceRoomGrants(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (store.VoiceRoomGrantDecision, error)
}

// ResolveVoiceRoomGrants serves the Voice-only decision on Role's authenticated
// private listener. The ordinary listener continues to reject this method.
func (s *RoleGRPC) ResolveVoiceRoomGrants(ctx context.Context, request *rolev1.ResolveVoiceRoomGrantsRequest) (*rolev1.ResolveVoiceRoomGrantsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "voice room grant request required")
	}
	spaceID, err := parseCanonicalGrantUUID("space_id", request.SpaceId)
	if err != nil {
		return nil, err
	}
	voiceRoomID, err := parseCanonicalGrantUUID("voice_room_id", request.VoiceRoomId)
	if err != nil {
		return nil, err
	}
	profileID, err := parseCanonicalGrantUUID("profile_id", request.ProfileId)
	if err != nil {
		return nil, err
	}
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "voice" || verified.Subject != "service:voice" ||
		verified.Audience != "role" || verified.RPC != rolev1.RoleService_ResolveVoiceRoomGrants_FullMethodName || verified.RequestID == "" ||
		verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "exact verified Voice principal required")
	}
	if _, err := parseCanonicalGrantUUID("request_id", verified.RequestID); err != nil {
		return nil, status.Error(codes.PermissionDenied, "fresh canonical Voice request ID required")
	}
	if _, err := gameGrantPrincipal(ctx, rolev1.RoleService_ResolveVoiceRoomGrants_FullMethodName,
		"voice", "service:voice", verified.RequestID, request); err != nil {
		return nil, err
	}
	var grantStore voiceRoomGrantDecisionStore
	if s != nil {
		grantStore = s.voiceRoomGrantStore
		if grantStore == nil && s.Store != nil {
			grantStore = s.Store
		}
	}
	if grantStore == nil {
		return nil, status.Error(codes.Unavailable, "Voice room grant persistence unavailable")
	}
	if s.Store != nil && s.voiceRoomGrantStore == nil {
		native, err := s.Store.NativeMembershipRequired(ctx)
		if err != nil {
			return nil, memberAdmissionError(ctx, err)
		}
		if native {
			if s.MemberAuthority == nil {
				return nil, status.Error(codes.Unavailable, "native membership authority unavailable")
			}
			// Complete the owner call before ResolveVoiceRoomGrants takes the
			// local Role fence; the immutable observation retains its deadline.
			observation, err := s.MemberAuthority.ObserveMember(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: request.SpaceId, ProfileId: request.ProfileId})
			if err != nil {
				return nil, memberAdmissionError(ctx, err)
			}
			ctx, err = memberauthority.WithObservations(ctx, observation)
			if err != nil {
				return nil, memberAdmissionError(ctx, err)
			}
		}
	}
	decision, err := grantStore.ResolveVoiceRoomGrants(ctx, spaceID, voiceRoomID, profileID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice room grant decision unavailable")
	}
	if decision.PolicyEpoch == 0 {
		return nil, status.Error(codes.Unavailable, "Voice room grant decision incomplete")
	}
	return &rolev1.ResolveVoiceRoomGrantsResponse{
		Grants: &rolev1.VoiceRoomGrants{
			CanJoin: decision.CanJoin, CanPublishAudio: decision.CanPublishAudio,
			CanPublishVideo: decision.CanPublishVideo, CanPublishScreenShare: decision.CanPublishScreenShare,
			CanSubscribe: decision.CanSubscribe, CanMuteOthers: decision.CanMuteOthers,
			CanDeafenOthers: decision.CanDeafenOthers, CanMoveOthers: decision.CanMoveOthers,
			CanUsePtt: decision.CanUsePTT, PrioritySpeaker: decision.PrioritySpeaker,
		},
		PolicyEpoch: decision.PolicyEpoch,
	}, nil
}
