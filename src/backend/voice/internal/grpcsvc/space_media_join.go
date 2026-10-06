package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
)

func (s *VoiceGRPC) getSpaceMediaJoinToken(ctx context.Context, call voicestore.Call, profileID string, access CanonicalVoiceRoomAccess) (*callsv1.GetJoinTokenResponse, error) {
	if !s.SpaceMediaReady {
		return nil, status.Error(codes.Unavailable, "Space media reconciliation is not ready")
	}
	if s.SpaceTokens == nil || s.SpaceVoiceRoomGrants == nil {
		return nil, status.Error(codes.FailedPrecondition, "Space media grant issuer is not configured")
	}
	verifiedProfileID, accountID, sessionEpoch, identityErr := callerVoiceUser(ctx)
	parsedAccountID, parseErr := uuid.Parse(accountID)
	if identityErr != nil || verifiedProfileID != profileID || accountID == "" || parseErr != nil || parsedAccountID == uuid.Nil || parsedAccountID.String() != accountID || sessionEpoch <= 0 {
		return nil, status.Error(codes.Unauthenticated, "verified media identity required")
	}
	if access.SpaceID != call.SpaceID || access.AccessEpoch == 0 {
		return nil, status.Error(codes.PermissionDenied, "canonical Space media authority is incomplete")
	}
	grants, err := s.SpaceVoiceRoomGrants.ResolveVoiceRoomGrants(ctx, call.SpaceID, call.VoiceRoomID, profileID)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.NotFound {
			return nil, status.Error(codes.PermissionDenied, "Space voice media permission denied")
		}
		return nil, status.Error(codes.Unavailable, "Space voice media authority unavailable")
	}
	if grants.PolicyEpoch == 0 {
		return nil, status.Error(codes.Unavailable, "Space voice media authority incomplete")
	}
	if !grants.CanJoin || !grants.CanSubscribe {
		return nil, status.Error(codes.PermissionDenied, "Space voice media permission denied")
	}
	accessFloors, err := s.Calls.RaiseSpaceMediaEpochFloor(ctx, call.SpaceID, voicestore.SpaceAccessEpoch, access.AccessEpoch)
	if err != nil {
		return nil, storeErr(err)
	}
	policyFloors, err := s.Calls.RaiseSpaceMediaEpochFloor(ctx, call.SpaceID, voicestore.RolePolicyEpoch, grants.PolicyEpoch)
	if err != nil {
		return nil, storeErr(err)
	}
	if accessFloors.AccessEpoch > access.AccessEpoch || policyFloors.PolicyEpoch > grants.PolicyEpoch {
		return nil, status.Error(codes.FailedPrecondition, "Space media authority changed; retry join")
	}
	maxParticipants := voicestore.MaxVoiceRoomParticipants
	if s.SpacePro != nil {
		pro, err := s.SpacePro.HasSpacePro(ctx, call.SpaceID)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "Space subscription check unavailable")
		}
		if pro {
			maxParticipants = voicestore.MaxSpaceProVoiceParticipants
		}
	}

	current, err := s.Calls.GetCall(ctx, call.RoomID)
	if err != nil {
		return nil, storeErr(err)
	}
	participant, exists := current.SpaceMedia[profileID]
	var generation, identity string
	if exists {
		if participant.Revoking {
			return nil, status.Error(codes.FailedPrecondition, "Space media session is being revoked")
		}
		if participant.Issued.SessionEpoch != uint64(sessionEpoch) || participant.Issued.AccessEpoch != access.AccessEpoch || participant.Issued.PolicyEpoch != grants.PolicyEpoch {
			return nil, status.Error(codes.FailedPrecondition, "Space media session must leave and rejoin after an authority change")
		}
		generation, identity = participant.Generation, participant.Identity
	} else {
		generation = uuid.NewString()
		identity, err = livekit.SpaceParticipantIdentity(profileID, generation)
		if err != nil {
			return nil, status.Error(codes.Internal, "Space media identity could not be created")
		}
	}
	issued := voicestore.SpaceMediaGrant{
		SessionEpoch: uint64(sessionEpoch), AccessEpoch: access.AccessEpoch, PolicyEpoch: grants.PolicyEpoch,
		CanJoin: grants.CanJoin, CanPublishAudio: grants.CanPublishAudio, CanSubscribe: grants.CanSubscribe,
	}
	grant := livekit.SpaceRoomGrant{
		AccountID: accountID, ProfileID: profileID, SpaceID: call.SpaceID, VoiceRoomID: call.VoiceRoomID,
		LiveKitRoomName: call.LivekitRoomName, MediaGeneration: generation,
		SessionEpoch: issued.SessionEpoch, AccessEpoch: issued.AccessEpoch, PolicyEpoch: issued.PolicyEpoch,
		CanJoin: issued.CanJoin, CanPublishAudio: issued.CanPublishAudio, CanSubscribe: issued.CanSubscribe,
	}
	jwt, expiresAt, err := s.SpaceTokens.JoinToken(grant, s.now())
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Space media token could not be issued")
	}
	fences, err := s.reserveAccountVoiceProfiles(ctx, call.RoomID, []string{profileID}, profileID)
	if err != nil {
		return nil, err
	}
	if err := s.commitAccountVoiceReservations(ctx, fences); err != nil {
		s.releaseAccountVoiceReservations(ctx, fences)
		return nil, err
	}
	_, err = s.Calls.AdmitSpaceMediaParticipant(ctx, call.RoomID, voicestore.SpaceMediaParticipant{
		ProfileID: profileID, Identity: identity, Generation: generation, Issued: issued,
	}, maxParticipants)
	if err != nil {
		s.releaseAccountVoiceReservations(ctx, fences)
		if errors.Is(err, voicestore.ErrSpaceMediaStaleGrant) || errors.Is(err, voicestore.ErrSpaceMediaTransition) {
			return nil, status.Error(codes.FailedPrecondition, "Space media authority changed; retry join")
		}
		return nil, storeErr(err)
	}
	return &callsv1.GetJoinTokenResponse{Jwt: jwt, LivekitUrl: s.SpaceTokens.LivekitURL(), ExpiresAt: timestamppb.New(expiresAt)}, nil
}
