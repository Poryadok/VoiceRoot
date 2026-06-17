package grpcsvc

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/voice/internal/authctx"
	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"
	"voice/backend/voice/internal/voiceevents"
	"voice/backend/pkg/guestguard"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
)

type SpaceProLookup interface {
	HasSpacePro(ctx context.Context, spaceID string) (bool, error)
}

type VoiceGRPC struct {
	callsv1.UnimplementedVoiceServiceServer

	Calls        voicestore.CallStore
	ChatMembers  ChatMembership
	SpaceMembers SpaceMembership
	SpacePro     SpaceProLookup
	Roles        RolePermissionChecker
	Tokens       livekit.TokenIssuer
	Events       voiceevents.Publisher
	Now          func() time.Time
	RingTimeout  time.Duration
}

func (s *VoiceGRPC) StartCall(ctx context.Context, req *callsv1.StartCallRequest) (*callsv1.StartCallResponse, error) {
	if err := guestguard.RequireRegular(ctx); err != nil {
		return nil, err
	}
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Calls == nil {
		return nil, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	if sessionKind(req) == callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE {
		return s.startGroupVoice(ctx, req, profileID)
	}
	chatID := strings.TrimSpace(req.GetLinkedChat().GetId())
	calleeID := strings.TrimSpace(req.GetCalleeProfileId())
	if chatID == "" {
		return nil, status.Error(codes.InvalidArgument, "linked_chat.id is required")
	}
	if calleeID == "" {
		return nil, status.Error(codes.InvalidArgument, "callee_profile_id is required")
	}
	if calleeID == profileID {
		return nil, status.Error(codes.InvalidArgument, "cannot call self")
	}
	media := req.GetMediaKind()
	if media == callsv1.CallMediaKind_CALL_MEDIA_KIND_UNSPECIFIED {
		media = callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO
	}
	if media != callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO && media != callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO {
		return nil, status.Error(codes.InvalidArgument, "unsupported media_kind")
	}

	now := s.now()
	roomID := uuid.NewString()
	call, err := s.Calls.CreateCall(ctx, voicestore.Call{
		RoomID:             roomID,
		LivekitRoomName:    "voice-dm-" + roomID,
		ChatID:             chatID,
		InitiatorProfileID: profileID,
		CalleeProfileID:    calleeID,
		MediaKind:          media,
		Status:             callsv1.CallStatus_CALL_STATUS_RINGING,
		StartedAt:          now,
		ExpiresAt:          now.Add(s.ringTimeout()),
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishIncoming(ctx, call)
	return &callsv1.StartCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) AcceptCall(ctx context.Context, req *callsv1.AcceptCallRequest) (*callsv1.AcceptCallResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	if profileID != call.CalleeProfileID {
		return nil, status.Error(codes.PermissionDenied, "only callee can accept call")
	}
	if call.Status != callsv1.CallStatus_CALL_STATUS_RINGING {
		return nil, status.Error(codes.FailedPrecondition, "call is not ringing")
	}
	if !call.ExpiresAt.IsZero() && s.now().After(call.ExpiresAt) {
		return nil, status.Error(codes.FailedPrecondition, "call expired")
	}
	call, err = s.Calls.SetStatus(ctx, call.RoomID, callsv1.CallStatus_CALL_STATUS_ACTIVE, time.Time{})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishAccepted(ctx, call, profileID)
	return &callsv1.AcceptCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) DeclineCall(ctx context.Context, req *callsv1.DeclineCallRequest) (*callsv1.DeclineCallResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	if profileID != call.CalleeProfileID {
		return nil, status.Error(codes.PermissionDenied, "only callee can decline call")
	}
	call, err = s.Calls.SetStatus(ctx, call.RoomID, callsv1.CallStatus_CALL_STATUS_DECLINED, s.now())
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishDeclined(ctx, call, profileID)
	s.publishEnded(ctx, call, "declined", profileID)
	return &callsv1.DeclineCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) JoinCall(ctx context.Context, req *callsv1.JoinCallRequest) (*callsv1.JoinCallResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Calls == nil {
		return nil, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	roomID := strings.TrimSpace(req.GetRoomId())
	call, err := s.Calls.GetCall(ctx, roomID)
	if err != nil {
		return nil, storeErr(err)
	}
	if call.IsGroupVoice() {
		if err := s.ensureChatMember(ctx, call.ChatID, profileID); err != nil {
			return nil, err
		}
		call, err = s.Calls.AddParticipant(ctx, roomID, profileID, voicestore.MaxGroupVoiceParticipants)
		if err != nil {
			return nil, storeErr(err)
		}
		return &callsv1.JoinCallResponse{CallSession: callToProto(call)}, nil
	}
	if !call.IsParticipant(profileID) {
		return nil, status.Error(codes.PermissionDenied, "not a call participant")
	}
	return &callsv1.JoinCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) LeaveCall(ctx context.Context, req *callsv1.LeaveCallRequest) (*callsv1.LeaveCallResponse, error) {
	_, err := s.EndCall(ctx, &callsv1.EndCallRequest{RoomId: req.GetRoomId()})
	return &callsv1.LeaveCallResponse{}, err
}

func (s *VoiceGRPC) EndCall(ctx context.Context, req *callsv1.EndCallRequest) (*callsv1.EndCallResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	call, err = s.Calls.SetStatus(ctx, call.RoomID, callsv1.CallStatus_CALL_STATUS_ENDED, s.now())
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishEnded(ctx, call, "hangup", profileID)
	return &callsv1.EndCallResponse{}, nil
}

func (s *VoiceGRPC) GetJoinToken(ctx context.Context, req *callsv1.GetJoinTokenRequest) (*callsv1.GetJoinTokenResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	if call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return nil, status.Error(codes.FailedPrecondition, "call is not active")
	}
	if s.Tokens == nil {
		return nil, status.Error(codes.FailedPrecondition, "livekit token issuer not configured")
	}
	jwt, expiresAt, err := s.Tokens.JoinToken(profileID, call.LivekitRoomName, s.now())
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &callsv1.GetJoinTokenResponse{
		Jwt:         jwt,
		ExpiresAt:   timestamppb.New(expiresAt),
		LivekitUrl:  s.Tokens.LivekitURL(),
	}, nil
}

func (s *VoiceGRPC) UpdateVoiceState(ctx context.Context, req *callsv1.UpdateVoiceStateRequest) (*callsv1.UpdateVoiceStateResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, state, err := s.Calls.UpdateVoiceState(ctx, req.GetRoomId(), profileID, voicestore.VoiceStatePatch{
		IsMuted:    req.IsMuted,
		IsDeafened: req.IsDeafened,
		IsVideoOn:  req.IsVideoOn,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.UpdateVoiceStateResponse{}, nil
}

func (s *VoiceGRPC) GetVoiceStates(ctx context.Context, req *callsv1.GetVoiceStatesRequest) (*callsv1.GetVoiceStatesResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	var call voicestore.Call
	voiceRoomID := strings.TrimSpace(req.GetVoiceRoomId())
	if voiceRoomID == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-voice-room-id"); len(vals) > 0 {
				voiceRoomID = strings.TrimSpace(vals[0])
			}
		}
	}
	if voiceRoomID != "" {
		if s == nil || s.Calls == nil {
			return nil, status.Error(codes.FailedPrecondition, "voice persistence not configured")
		}
		call, err = s.Calls.GetCallByVoiceRoomID(ctx, voiceRoomID)
		if errors.Is(err, voicestore.ErrNotFound) {
			return &callsv1.GetVoiceStatesResponse{}, nil
		}
		if err != nil {
			return nil, storeErr(err)
		}
		if call.SpaceID != "" {
			if err := s.ensureSpaceMember(ctx, call.SpaceID, profileID); err != nil {
				return nil, err
			}
		}
	} else {
		call, err = s.requireCall(ctx, req.GetRoomId(), profileID)
		if err != nil {
			return nil, err
		}
	}
	var participants []*callsv1.VoiceParticipantState
	for _, id := range call.ProfileIDs() {
		state := call.States[id]
		participants = append(participants, &callsv1.VoiceParticipantState{
			ProfileId:       id,
			IsMuted:         state.IsMuted,
			IsDeafened:      state.IsDeafened,
			IsVideoOn:       state.IsVideoOn,
			IsScreenSharing: state.IsScreenSharing,
		})
	}
	return &callsv1.GetVoiceStatesResponse{Participants: participants}, nil
}

func (s *VoiceGRPC) GetActiveCall(ctx context.Context, _ *callsv1.GetActiveCallRequest) (*callsv1.GetActiveCallResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	chatID, _ := authctx.ActiveChatID(ctx)
	if chatID != "" {
		if err := s.ensureChatMember(ctx, chatID, profileID); err != nil {
			return nil, err
		}
		call, err := s.Calls.GetActiveGroupCallForChat(ctx, chatID)
		if err != nil {
			return nil, storeErr(err)
		}
		return &callsv1.GetActiveCallResponse{CallSession: callToProto(call)}, nil
	}
	call, err := s.Calls.GetActiveCall(ctx, profileID)
	if err != nil {
		return nil, storeErr(err)
	}
	return &callsv1.GetActiveCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) MarkExpiredCallsMissed(ctx context.Context) (int, error) {
	if s == nil || s.Calls == nil {
		return 0, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	expired, err := s.Calls.ListExpiredRinging(ctx, s.now())
	if err != nil {
		return 0, err
	}
	for _, call := range expired {
		updated, err := s.Calls.SetStatus(ctx, call.RoomID, callsv1.CallStatus_CALL_STATUS_MISSED, s.now())
		if err != nil {
			return 0, err
		}
		s.publishMissed(ctx, updated)
		s.publishEnded(ctx, updated, "missed", "")
	}
	return len(expired), nil
}

func (s *VoiceGRPC) requireCall(ctx context.Context, roomID, profileID string) (voicestore.Call, error) {
	if s == nil || s.Calls == nil {
		return voicestore.Call{}, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	call, err := s.Calls.GetCall(ctx, strings.TrimSpace(roomID))
	if err != nil {
		return voicestore.Call{}, storeErr(err)
	}
	if !call.IsParticipant(profileID) {
		return voicestore.Call{}, status.Error(codes.PermissionDenied, "not a call participant")
	}
	return call, nil
}

func callerProfile(ctx context.Context) (string, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing profile")
	}
	return profileID, nil
}

func (s *VoiceGRPC) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *VoiceGRPC) ringTimeout() time.Duration {
	if s != nil && s.RingTimeout > 0 {
		return s.RingTimeout
	}
	return 30 * time.Second
}

func storeErr(err error) error {
	switch {
	case errors.Is(err, voicestore.ErrNotFound):
		return status.Error(codes.NotFound, "call not found")
	case errors.Is(err, voicestore.ErrActiveCall):
		return status.Error(codes.FailedPrecondition, "profile already has active call")
	case errors.Is(err, voicestore.ErrNotParticipant):
		return status.Error(codes.PermissionDenied, "not a call participant")
	case errors.Is(err, voicestore.ErrInvalidState):
		return status.Error(codes.FailedPrecondition, "invalid call state")
	case errors.Is(err, voicestore.ErrRoomFull):
		return status.Error(codes.ResourceExhausted, "voice room is full")
	case errors.Is(err, voicestore.ErrScreenShareLimit):
		return status.Error(codes.ResourceExhausted, "screen share limit reached")
	case errors.Is(err, voicestore.ErrNotScreenSharing):
		return status.Error(codes.FailedPrecondition, "not screen sharing")
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func sessionKind(req *callsv1.StartCallRequest) callsv1.VoiceSessionKind {
	if req.GetRoomTypeEnum() != callsv1.VoiceSessionKind_VOICE_SESSION_KIND_UNSPECIFIED {
		return req.GetRoomTypeEnum()
	}
	switch strings.TrimSpace(req.GetRoomType()) {
	case "group_voice":
		return callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
	case "voice_room":
		return callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM
	case "call":
		return callsv1.VoiceSessionKind_VOICE_SESSION_KIND_CALL
	default:
		return callsv1.VoiceSessionKind_VOICE_SESSION_KIND_UNSPECIFIED
	}
}

func (s *VoiceGRPC) startGroupVoice(ctx context.Context, req *callsv1.StartCallRequest, profileID string) (*callsv1.StartCallResponse, error) {
	chatID := strings.TrimSpace(req.GetLinkedChat().GetId())
	if chatID == "" {
		return nil, status.Error(codes.InvalidArgument, "linked_chat.id is required")
	}
	if req.GetLinkedChat().GetType() != chatv1.ChatType_CHAT_TYPE_GROUP {
		return nil, status.Error(codes.InvalidArgument, "group voice requires CHAT_TYPE_GROUP")
	}
	if err := s.ensureChatMember(ctx, chatID, profileID); err != nil {
		return nil, err
	}
	media := req.GetMediaKind()
	if media == callsv1.CallMediaKind_CALL_MEDIA_KIND_UNSPECIFIED {
		media = callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO
	}
	if media != callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO && media != callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO {
		return nil, status.Error(codes.InvalidArgument, "unsupported media_kind")
	}

	now := s.now()
	roomID := uuid.NewString()
	call, err := s.Calls.CreateCall(ctx, voicestore.Call{
		RoomID:             roomID,
		LivekitRoomName:    "voice-group-" + roomID,
		ChatID:             chatID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		InitiatorProfileID: profileID,
		MediaKind:          media,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishAccepted(ctx, call, profileID)
	return &callsv1.StartCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) ensureChatMember(ctx context.Context, chatID, profileID string) error {
	if s.ChatMembers == nil {
		return nil
	}
	if err := s.ChatMembers.EnsureMember(ctx, chatID, profileID); err != nil {
		if errors.Is(err, ErrNotChatMember) {
			return status.Error(codes.PermissionDenied, "not a chat member")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func callToProto(call voicestore.Call) *callsv1.CallSession {
	roomType := "call"
	kind := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_CALL
	if call.IsGroupVoice() {
		roomType = "group_voice"
		kind = callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
	} else if call.IsVoiceRoom() {
		roomType = "voice_room"
		kind = callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM
	}
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	linkedChat := &chatv1.ChatRef{Id: call.ChatID}
	if call.IsGroupVoice() {
		linkedChat.Type = &group
	}
	out := &callsv1.CallSession{
		RoomId:             call.RoomID,
		LivekitRoomName:    call.LivekitRoomName,
		RoomType:           roomType,
		LinkedChat:         linkedChat,
		RoomTypeEnum:       kind.Enum(),
		InitiatorProfileId: call.InitiatorProfileID,
		CalleeProfileId:    call.CalleeProfileID,
		MediaKind:          call.MediaKind,
		Status:             call.Status,
	}
	if call.VoiceRoomID != "" {
		vr := call.VoiceRoomID
		out.VoiceRoomId = &vr
	}
	if !call.StartedAt.IsZero() {
		out.StartedAt = timestamppb.New(call.StartedAt)
	}
	if !call.ExpiresAt.IsZero() {
		out.ExpiresAt = timestamppb.New(call.ExpiresAt)
	}
	if !call.EndedAt.IsZero() {
		out.EndedAt = timestamppb.New(call.EndedAt)
	}
	return out
}

func (s *VoiceGRPC) publishIncoming(ctx context.Context, call voicestore.Call) {
	if s.Events == nil {
		return
	}
	if err := s.Events.PublishCallIncoming(ctx, &eventsv1.CallIncoming{
		RoomId:             call.RoomID,
		ChatId:             call.ChatID,
		InitiatorProfileId: call.InitiatorProfileID,
		CalleeProfileId:    call.CalleeProfileID,
		MediaKind:          mediaKindString(call.MediaKind),
		LivekitRoomName:    call.LivekitRoomName,
		ExpiresAt:          timestamppb.New(call.ExpiresAt),
	}); err != nil {
		log.Printf("voice: publish call_incoming: %v", err)
	}
}

func (s *VoiceGRPC) publishAccepted(ctx context.Context, call voicestore.Call, by string) {
	if s.Events == nil {
		return
	}
	if err := s.Events.PublishCallAccepted(ctx, &eventsv1.CallAccepted{
		RoomId:              call.RoomID,
		ChatId:              call.ChatID,
		AcceptedByProfileId: by,
		ProfileIds:          call.ProfileIDs(),
		MediaKind:           mediaKindString(call.MediaKind),
		LivekitRoomName:     call.LivekitRoomName,
	}); err != nil {
		log.Printf("voice: publish call_accepted: %v", err)
	}
}

func (s *VoiceGRPC) publishDeclined(ctx context.Context, call voicestore.Call, by string) {
	if s.Events == nil {
		return
	}
	if err := s.Events.PublishCallDeclined(ctx, &eventsv1.CallDeclined{
		RoomId:              call.RoomID,
		ChatId:              call.ChatID,
		DeclinedByProfileId: by,
		ProfileIds:          call.ProfileIDs(),
	}); err != nil {
		log.Printf("voice: publish call_declined: %v", err)
	}
}

func (s *VoiceGRPC) publishMissed(ctx context.Context, call voicestore.Call) {
	if s.Events == nil {
		return
	}
	if err := s.Events.PublishCallMissed(ctx, &eventsv1.CallMissed{
		RoomId:             call.RoomID,
		ChatId:             call.ChatID,
		InitiatorProfileId: call.InitiatorProfileID,
		CalleeProfileId:    call.CalleeProfileID,
	}); err != nil {
		log.Printf("voice: publish call_missed: %v", err)
	}
}

func (s *VoiceGRPC) publishEnded(ctx context.Context, call voicestore.Call, reason, by string) {
	if s.Events == nil {
		return
	}
	duration := int32(0)
	if !call.StartedAt.IsZero() && !call.EndedAt.IsZero() && call.EndedAt.After(call.StartedAt) {
		duration = int32(call.EndedAt.Sub(call.StartedAt).Seconds())
	}
	if err := s.Events.PublishCallEnded(ctx, &eventsv1.CallEnded{
		RoomId:           call.RoomID,
		DurationSeconds:  duration,
		ProfileIds:       call.ProfileIDs(),
		Reason:           reason,
		EndedByProfileId: by,
	}); err != nil {
		log.Printf("voice: publish call_ended: %v", err)
	}
}

func (s *VoiceGRPC) publishState(ctx context.Context, call voicestore.Call, state voicestore.ParticipantState) {
	if s.Events == nil {
		return
	}
	if err := s.Events.PublishVoiceStateChanged(ctx, &eventsv1.VoiceStateChanged{
		RoomId:     call.RoomID,
		ProfileId:  state.ProfileID,
		IsMuted:    &state.IsMuted,
		IsDeafened: &state.IsDeafened,
		IsVideoOn:  &state.IsVideoOn,
		ProfileIds: call.ProfileIDs(),
	}); err != nil {
		log.Printf("voice: publish state_changed: %v", err)
	}
}

func mediaKindString(kind callsv1.CallMediaKind) string {
	if kind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO {
		return "video"
	}
	return "audio"
}
