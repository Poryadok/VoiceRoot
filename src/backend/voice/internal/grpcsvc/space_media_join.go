package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	eventsv1 "voice.app/voice/events/v1"
)

func (s *VoiceGRPC) ensureConfirmedSpaceMediaCall(ctx context.Context, call voicestore.Call) error {
	if !call.IsVoiceRoom() || call.SpaceID == "" {
		return nil
	}
	if !s.spaceMediaOperational() || s.SpaceMediaAdmissions == nil {
		return status.Error(codes.Unavailable, "Space media reconciliation is not ready")
	}
	if call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE || len(call.States) == 0 || len(call.SpaceMedia) != len(call.States) {
		return status.Error(codes.Unavailable, "Space media projection is not confirmed")
	}
	spaceID, err := uuid.Parse(call.SpaceID)
	if err != nil || spaceID == uuid.Nil || spaceID.String() != call.SpaceID {
		return status.Error(codes.Unavailable, "Space media projection is not confirmed")
	}
	if err := s.SpaceMediaAdmissions.CheckSchema(ctx); err != nil {
		return status.Error(codes.Unavailable, "Space media admission journal is not ready")
	}
	for profileID := range call.States {
		participant, exists := call.SpaceMedia[profileID]
		if !exists || participant.ProfileID != profileID || participant.Revoking || participant.AccountID == "" ||
			participant.Generation == "" || participant.RoomGeneration == 0 || participant.Issued.SessionEpoch == 0 || participant.Issued.AccessEpoch == 0 || participant.Issued.PolicyEpoch == 0 ||
			!participant.Issued.CanJoin || !participant.Issued.CanSubscribe {
			return status.Error(codes.Unavailable, "Space media projection is not confirmed")
		}
		operationID, operationErr := uuid.Parse(participant.AdmissionOperationID)
		accountID, accountErr := uuid.Parse(participant.AccountID)
		profileUUID, profileErr := uuid.Parse(profileID)
		identity, identityErr := livekit.SpaceParticipantIdentity(profileID, participant.Generation)
		if operationErr != nil || operationID == uuid.Nil || accountErr != nil || accountID == uuid.Nil || accountID.String() != participant.AccountID ||
			profileErr != nil || profileUUID == uuid.Nil || profileUUID.String() != profileID || identityErr != nil || participant.Identity != identity {
			return status.Error(codes.Unavailable, "Space media projection is not confirmed")
		}
		if err := s.SpaceMediaAdmissions.ConfirmProjection(ctx, operationID, participant.Generation); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return status.Error(codes.Unavailable, "Space media projection is not confirmed")
		}
		if err := s.SpaceMediaAdmissions.ConfirmRoomHeadOpen(ctx, voicestore.SpaceMediaAdmission{
			OperationID: operationID, Generation: participant.Generation, RoomGeneration: participant.RoomGeneration,
			AccountID: accountID, ProfileID: profileUUID, SpaceID: spaceID,
			RoomID: call.RoomID, VoiceRoomID: call.VoiceRoomID, CreatedRoom: participant.CreatedRoom,
		}); err != nil {
			return status.Error(codes.Unavailable, "Space voice-room incarnation is closing or unconfirmed")
		}
	}
	return nil
}

func (s *VoiceGRPC) admitSpaceVoiceRoom(ctx context.Context, call voicestore.Call, profileID string, access CanonicalVoiceRoomAccess, maxParticipants int, created bool) (voicestore.Call, error) {
	if s == nil || s.Calls == nil || s.SpaceMediaAdmissions == nil || s.SpaceVoiceRoomGrants == nil || s.SessionEpochChecker == nil || !s.spaceMediaOperational() {
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission is not ready")
	}
	if err := s.SpaceMediaAdmissions.CheckSchema(ctx); err != nil {
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission journal is not ready")
	}
	verifiedProfileID, accountID, sessionEpoch, identityErr := callerVoiceUser(ctx)
	parsedAccountID, parseErr := uuid.Parse(accountID)
	if identityErr != nil || verifiedProfileID != profileID || accountID == "" || parseErr != nil || parsedAccountID == uuid.Nil || parsedAccountID.String() != accountID || sessionEpoch <= 0 {
		return voicestore.Call{}, status.Error(codes.Unauthenticated, "verified media identity required")
	}
	parsedProfileID, profileErr := uuid.Parse(profileID)
	parsedSpaceID, spaceErr := uuid.Parse(call.SpaceID)
	parsedVoiceRoomID, voiceRoomErr := uuid.Parse(call.VoiceRoomID)
	if profileErr != nil || parsedProfileID == uuid.Nil || parsedProfileID.String() != profileID || spaceErr != nil || parsedSpaceID == uuid.Nil || parsedSpaceID.String() != call.SpaceID || voiceRoomErr != nil || parsedVoiceRoomID == uuid.Nil || parsedVoiceRoomID.String() != call.VoiceRoomID {
		return voicestore.Call{}, status.Error(codes.InvalidArgument, "invalid Space media identity")
	}
	if err := s.SessionEpochChecker.RequireCurrent(ctx, accountID, sessionEpoch); err != nil {
		if status.Code(err) == codes.Unauthenticated {
			return voicestore.Call{}, status.Error(codes.Unauthenticated, "verified media identity is stale")
		}
		return voicestore.Call{}, status.Error(codes.Unavailable, "Auth session authority unavailable")
	}
	if access.SpaceID != call.SpaceID || access.AccessEpoch == 0 {
		return voicestore.Call{}, status.Error(codes.PermissionDenied, "canonical Space media authority is incomplete")
	}
	grants, err := s.SpaceVoiceRoomGrants.ResolveVoiceRoomGrants(ctx, call.SpaceID, call.VoiceRoomID, profileID)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.NotFound {
			return voicestore.Call{}, status.Error(codes.PermissionDenied, "Space voice media permission denied")
		}
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space voice media authority unavailable")
	}
	if grants.PolicyEpoch == 0 || !grants.CanJoin || !grants.CanSubscribe {
		return voicestore.Call{}, status.Error(codes.PermissionDenied, "Space voice media permission denied")
	}
	accessFloors, err := s.Calls.RaiseSpaceMediaEpochFloor(ctx, call.SpaceID, voicestore.SpaceAccessEpoch, access.AccessEpoch)
	if err != nil {
		return voicestore.Call{}, storeErr(err)
	}
	policyFloors, err := s.Calls.RaiseSpaceMediaEpochFloor(ctx, call.SpaceID, voicestore.RolePolicyEpoch, grants.PolicyEpoch)
	if err != nil {
		return voicestore.Call{}, storeErr(err)
	}
	if accessFloors.AccessEpoch > access.AccessEpoch || policyFloors.PolicyEpoch > grants.PolicyEpoch {
		return voicestore.Call{}, status.Error(codes.FailedPrecondition, "Space media authority changed; retry join")
	}
	current := call
	if !created {
		current, err = s.Calls.GetCall(ctx, call.RoomID)
		if err != nil {
			return voicestore.Call{}, storeErr(err)
		}
	}
	if current.SpaceID != call.SpaceID || current.VoiceRoomID != call.VoiceRoomID ||
		(!created && current.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE) ||
		(created && current.Status != callsv1.CallStatus_CALL_STATUS_UNSPECIFIED) {
		return voicestore.Call{}, status.Error(codes.FailedPrecondition, "Space voice room is no longer active")
	}
	if participant, exists := current.SpaceMedia[profileID]; exists {
		if participant.Revoking || participant.AccountID != accountID || participant.AdmissionOperationID == "" ||
			participant.Issued.SessionEpoch != uint64(sessionEpoch) || participant.Issued.AccessEpoch != access.AccessEpoch || participant.Issued.PolicyEpoch != grants.PolicyEpoch {
			return voicestore.Call{}, status.Error(codes.FailedPrecondition, "Space media session must leave and rejoin after an authority change")
		}
		operationID, err := uuid.Parse(participant.AdmissionOperationID)
		if err != nil || s.SpaceMediaAdmissions.ConfirmProjection(ctx, operationID, participant.Generation) != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission is not confirmed")
		}
		identity, identityErr := livekit.SpaceParticipantIdentity(profileID, participant.Generation)
		if identityErr != nil || participant.Identity != identity {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission is not confirmed")
		}
		return current, nil
	}
	if current.IsParticipant(profileID) {
		return voicestore.Call{}, status.Error(codes.FailedPrecondition, "Space voice membership has no confirmed media admission")
	}
	generation := uuid.NewString()
	identity, err := livekit.SpaceParticipantIdentity(profileID, generation)
	if err != nil {
		return voicestore.Call{}, status.Error(codes.Internal, "Space media identity could not be created")
	}
	operationID := uuid.New()
	roomHead, err := s.SpaceMediaAdmissions.ClaimRoomHead(ctx, call.VoiceRoomID, call.SpaceID, call.RoomID, operationID, created)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return voicestore.Call{}, err
		}
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space voice-room incarnation is not ready")
	}
	abandonClaim := func() {
		if created {
			_ = s.SpaceMediaAdmissions.AbandonRoomClaim(ctx, roomHead.VoiceRoomID, roomHead.RoomGeneration, operationID)
		}
	}
	if roomHead.CreatorOperationID == operationID {
		created = true
	} else {
		created = false
	}
	if roomHead.Ready && call.RoomID != roomHead.RoomID {
		call, err = s.Calls.GetCall(ctx, roomHead.RoomID)
		if err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space voice-room projection is being recovered")
		}
	}
	if call.RoomID != roomHead.RoomID || call.SpaceID != roomHead.SpaceID || call.VoiceRoomID != roomHead.VoiceRoomID ||
		(!roomHead.Ready && roomHead.CreatorOperationID != operationID) || (roomHead.Ready && created) {
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space voice-room incarnation is not ready")
	}
	call.RoomID = roomHead.RoomID
	current = call
	callStartedID, joinedID := uuid.New(), uuid.New()
	profiles := append([]string(nil), current.ProfileIDs()...)
	profiles = append(profiles, profileID)
	roomType := "voice_room"
	voiceRoomID, spaceID := current.VoiceRoomID, current.SpaceID
	joined := &eventsv1.VoiceMemberJoined{RoomId: current.RoomID, VoiceRoomId: current.VoiceRoomID, SpaceId: current.SpaceID, JoinedProfileId: profileID}
	for _, existing := range current.ProfileIDs() {
		if existing != profileID {
			joined.NotifyProfileIds = append(joined.NotifyProfileIds, existing)
		}
	}
	joinedEnvelope, err := proto.Marshal(&eventsv1.VoiceStreamEvent{EventId: joinedID.String(), OccurredAt: timestamppb.New(s.now()), Payload: &eventsv1.VoiceStreamEvent_VoiceMemberJoined{VoiceMemberJoined: joined}})
	if err != nil {
		abandonClaim()
		return voicestore.Call{}, status.Error(codes.Internal, "Space media event could not be prepared")
	}
	issued := voicestore.SpaceMediaGrant{SessionEpoch: uint64(sessionEpoch), AccessEpoch: access.AccessEpoch, PolicyEpoch: grants.PolicyEpoch,
		CanJoin: grants.CanJoin, CanPublishAudio: grants.CanPublishAudio, CanSubscribe: grants.CanSubscribe}
	events := []voicestore.SpaceMediaAdmissionEvent{{ID: joinedID, Subject: "voice.member_joined", Payload: joinedEnvelope}}
	if created {
		started := &eventsv1.CallStarted{
			RoomId: current.RoomID, ProfileIds: profiles, ChatId: current.ChatID,
			InitiatorProfileId: current.InitiatorProfileID, CalleeProfileId: current.CalleeProfileID,
			MediaKind: mediaKindString(current.MediaKind), LivekitRoomName: current.LivekitRoomName,
			VoiceRoomId: &voiceRoomID, SpaceId: &spaceID, RoomType: &roomType,
		}
		startedEnvelope, marshalErr := proto.Marshal(&eventsv1.VoiceStreamEvent{EventId: callStartedID.String(), OccurredAt: timestamppb.New(s.now()), Payload: &eventsv1.VoiceStreamEvent_CallStarted{CallStarted: started}})
		if marshalErr != nil {
			abandonClaim()
			return voicestore.Call{}, status.Error(codes.Internal, "Space media event could not be prepared")
		}
		events = append([]voicestore.SpaceMediaAdmissionEvent{{ID: callStartedID, Subject: "voice.call_started", Payload: startedEnvelope}}, events...)
	}
	admission := voicestore.SpaceMediaAdmission{OperationID: operationID, Generation: generation, RoomGeneration: roomHead.RoomGeneration, AccountID: parsedAccountID,
		ProfileID: parsedProfileID, SpaceID: parsedSpaceID, RoomID: current.RoomID, VoiceRoomID: current.VoiceRoomID, Identity: identity,
		CreatedRoom: created, CallStartedAt: current.StartedAt, MaxParticipants: maxParticipants,
		SessionEpoch: issued.SessionEpoch, AccessEpoch: issued.AccessEpoch, PolicyEpoch: issued.PolicyEpoch,
		CanJoin: issued.CanJoin, CanPublishAudio: issued.CanPublishAudio, CanSubscribe: issued.CanSubscribe,
		Events: events}
	if err := s.SpaceMediaAdmissions.Prepare(ctx, admission); err != nil {
		abandonClaim()
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission could not be prepared")
	}
	if err := s.SpaceMediaAdmissions.Fence(ctx, admission); err != nil {
		// The transaction may have committed even when its acknowledgement was
		// lost. Make the intent recoverable and release only this exact fence;
		// never infer that a failed RPC left no durable side effect.
		if abortErr := s.SpaceMediaAdmissions.MarkAborting(ctx, operationID, generation); abortErr != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if releaseErr := s.SpaceMediaAdmissions.ReleaseFence(ctx, admission); releaseErr != nil && status.Code(releaseErr) != codes.NotFound {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if cleanupErr := s.SpaceMediaAdmissions.MarkCleanupCompleted(ctx, operationID, generation); cleanupErr != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		return voicestore.Call{}, status.Error(codes.FailedPrecondition, "account already has an active Voice session")
	}
	latestAccess, accessErr := s.VoiceRoomAccessResolver.ResolveVoiceRoomAccess(ctx, current.VoiceRoomID, profileID)
	latestGrants, grantErr := s.SpaceVoiceRoomGrants.ResolveVoiceRoomGrants(ctx, current.SpaceID, current.VoiceRoomID, profileID)
	if accessErr != nil || grantErr != nil || latestAccess.SpaceID != current.SpaceID || !latestAccess.Active || !latestAccess.Member ||
		latestAccess.AccessEpoch != access.AccessEpoch || latestGrants.PolicyEpoch != grants.PolicyEpoch ||
		!latestGrants.CanJoin || !latestGrants.CanSubscribe || latestGrants.CanPublishAudio != grants.CanPublishAudio {
		if err := s.SpaceMediaAdmissions.MarkAborting(ctx, operationID, generation); err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if err := s.SpaceMediaAdmissions.ReleaseFence(ctx, admission); err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if err := s.SpaceMediaAdmissions.MarkCleanupCompleted(ctx, operationID, generation); err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if accessErr != nil && status.Code(accessErr) != codes.NotFound && status.Code(accessErr) != codes.PermissionDenied ||
			grantErr != nil && status.Code(grantErr) != codes.NotFound && status.Code(grantErr) != codes.PermissionDenied {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media authority unavailable")
		}
		return voicestore.Call{}, status.Error(codes.PermissionDenied, "Space media authority changed; retry join")
	}
	if err := s.SessionEpochChecker.RequireCurrent(ctx, accountID, sessionEpoch); err != nil {
		if abortErr := s.SpaceMediaAdmissions.MarkAborting(ctx, operationID, generation); abortErr != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if abortErr := s.SpaceMediaAdmissions.ReleaseFence(ctx, admission); abortErr != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if abortErr := s.SpaceMediaAdmissions.MarkCleanupCompleted(ctx, operationID, generation); abortErr != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if status.Code(err) == codes.Unauthenticated {
			return voicestore.Call{}, status.Error(codes.Unauthenticated, "verified media identity is stale")
		}
		return voicestore.Call{}, status.Error(codes.Unavailable, "Auth session authority unavailable")
	}
	latestAccessFloors, accessErr := s.Calls.RaiseSpaceMediaEpochFloor(ctx, current.SpaceID, voicestore.SpaceAccessEpoch, latestAccess.AccessEpoch)
	latestPolicyFloors, grantErr := s.Calls.RaiseSpaceMediaEpochFloor(ctx, current.SpaceID, voicestore.RolePolicyEpoch, latestGrants.PolicyEpoch)
	if accessErr != nil || grantErr != nil || latestAccessFloors.AccessEpoch > latestAccess.AccessEpoch || latestPolicyFloors.PolicyEpoch > latestGrants.PolicyEpoch {
		if err := s.SpaceMediaAdmissions.MarkAborting(ctx, operationID, generation); err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if err := s.SpaceMediaAdmissions.ReleaseFence(ctx, admission); err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		if err := s.SpaceMediaAdmissions.MarkCleanupCompleted(ctx, operationID, generation); err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission cleanup is pending")
		}
		return voicestore.Call{}, status.Error(codes.FailedPrecondition, "Space media authority changed; retry join")
	}
	if err := s.SpaceMediaAdmissions.MarkCommitted(ctx, operationID, generation); err != nil {
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space media admission commit is pending recovery")
	}
	if created {
		current, err = s.Calls.CreateCall(ctx, current)
		if err != nil {
			return voicestore.Call{}, status.Error(codes.Unavailable, "Space media projection is pending recovery")
		}
	}
	current, err = s.Calls.AdmitSpaceMediaParticipant(ctx, current.RoomID, voicestore.SpaceMediaParticipant{
		AccountID: accountID, AdmissionOperationID: operationID.String(), ProfileID: profileID,
		RoomGeneration: roomHead.RoomGeneration, CreatedRoom: created,
		Identity: identity, Generation: generation, Issued: issued,
	}, maxParticipants)
	if err != nil {
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space media projection is pending recovery")
	}
	if err := s.SpaceMediaAdmissions.MarkProjectionApplied(ctx, operationID, generation); err != nil {
		return voicestore.Call{}, status.Error(codes.Unavailable, "Space media projection confirmation is pending recovery")
	}
	return current, nil
}

func (s *VoiceGRPC) getSpaceMediaJoinToken(ctx context.Context, call voicestore.Call, profileID string, access CanonicalVoiceRoomAccess) (*callsv1.GetJoinTokenResponse, error) {
	if !s.spaceMediaOperational() {
		return nil, status.Error(codes.Unavailable, "Space media reconciliation is not ready")
	}
	if s.SpaceMediaAdmissions == nil || s.SpaceMediaAdmissions.CheckSchema(ctx) != nil {
		return nil, status.Error(codes.Unavailable, "Space media admission journal is not ready")
	}
	if s.SpaceTokens == nil || s.SpaceVoiceRoomGrants == nil {
		return nil, status.Error(codes.FailedPrecondition, "Space media grant issuer is not configured")
	}
	verifiedProfileID, accountID, sessionEpoch, identityErr := callerVoiceUser(ctx)
	parsedAccountID, parseErr := uuid.Parse(accountID)
	if identityErr != nil || verifiedProfileID != profileID || accountID == "" || parseErr != nil || parsedAccountID == uuid.Nil || parsedAccountID.String() != accountID || sessionEpoch <= 0 {
		return nil, status.Error(codes.Unauthenticated, "verified media identity required")
	}
	if s.SessionEpochChecker == nil {
		return nil, status.Error(codes.Unavailable, "Auth session authority unavailable")
	}
	if err := s.SessionEpochChecker.RequireCurrent(ctx, accountID, sessionEpoch); err != nil {
		if status.Code(err) == codes.Unauthenticated {
			return nil, status.Error(codes.Unauthenticated, "verified media identity is stale")
		}
		return nil, status.Error(codes.Unavailable, "Auth session authority unavailable")
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
	current, err := s.Calls.GetCall(ctx, call.RoomID)
	if err != nil {
		return nil, storeErr(err)
	}
	if current.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE || current.SpaceID != call.SpaceID || current.VoiceRoomID != call.VoiceRoomID || !current.IsParticipant(profileID) {
		return nil, status.Error(codes.FailedPrecondition, "Space media session is no longer active")
	}
	participant, exists := current.SpaceMedia[profileID]
	if !exists || participant.AdmissionOperationID == "" || participant.AccountID != accountID {
		return nil, status.Error(codes.FailedPrecondition, "Space media admission is not confirmed")
	}
	operationID, err := uuid.Parse(participant.AdmissionOperationID)
	if err != nil || s.SpaceMediaAdmissions.ConfirmProjection(ctx, operationID, participant.Generation) != nil {
		return nil, status.Error(codes.Unavailable, "Space media admission is not confirmed")
	}
	if err := s.ensureConfirmedSpaceMediaCall(ctx, current); err != nil {
		return nil, err
	}
	var generation, identity string
	if exists {
		if participant.Revoking {
			return nil, status.Error(codes.FailedPrecondition, "Space media session is being revoked")
		}
		if participant.Issued.SessionEpoch != uint64(sessionEpoch) || participant.Issued.AccessEpoch != access.AccessEpoch || participant.Issued.PolicyEpoch != grants.PolicyEpoch {
			return nil, status.Error(codes.FailedPrecondition, "Space media session must leave and rejoin after an authority change")
		}
		generation, identity = participant.Generation, participant.Identity
	}
	issued := participant.Issued
	latestAccess, err := s.resolveCanonicalVoiceRoomAccess(ctx, current.VoiceRoomID, profileID)
	if err != nil {
		return nil, err
	}
	latestGrants, err := s.SpaceVoiceRoomGrants.ResolveVoiceRoomGrants(ctx, current.SpaceID, current.VoiceRoomID, profileID)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.NotFound {
			return nil, status.Error(codes.PermissionDenied, "Space voice media permission denied")
		}
		return nil, status.Error(codes.Unavailable, "Space voice media authority unavailable")
	}
	if latestAccess.SpaceID != current.SpaceID || !latestAccess.Active || !latestAccess.Member || latestAccess.AccessEpoch != issued.AccessEpoch ||
		latestGrants.PolicyEpoch != issued.PolicyEpoch || !latestGrants.CanJoin || !latestGrants.CanSubscribe || latestGrants.CanPublishAudio != issued.CanPublishAudio {
		return nil, status.Error(codes.PermissionDenied, "Space media authority changed; retry join")
	}
	latestAccessFloors, err := s.Calls.RaiseSpaceMediaEpochFloor(ctx, current.SpaceID, voicestore.SpaceAccessEpoch, latestAccess.AccessEpoch)
	if err != nil {
		return nil, storeErr(err)
	}
	latestPolicyFloors, err := s.Calls.RaiseSpaceMediaEpochFloor(ctx, current.SpaceID, voicestore.RolePolicyEpoch, latestGrants.PolicyEpoch)
	if err != nil {
		return nil, storeErr(err)
	}
	if latestAccessFloors.AccessEpoch > latestAccess.AccessEpoch || latestPolicyFloors.PolicyEpoch > latestGrants.PolicyEpoch {
		return nil, status.Error(codes.FailedPrecondition, "Space media authority changed; retry join")
	}
	if err := s.SessionEpochChecker.RequireCurrent(ctx, accountID, sessionEpoch); err != nil {
		if status.Code(err) == codes.Unauthenticated {
			return nil, status.Error(codes.Unauthenticated, "verified media identity is stale")
		}
		return nil, status.Error(codes.Unavailable, "Auth session authority unavailable")
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
	return &callsv1.GetJoinTokenResponse{Jwt: jwt, LivekitUrl: s.SpaceTokens.LivekitURL(), ExpiresAt: timestamppb.New(expiresAt)}, nil
}
