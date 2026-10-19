package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/ordinarycall"
	voicestore "voice/backend/voice/internal/store"
)

func foreignOrdinaryResource(call voicestore.Call) bool {
	return callHasSpaceMediaMarker(call) || call.ManagedGameSession || call.MatchSquadMatchID != "" ||
		call.ApplicationID != "" || call.EnvironmentID != "" || call.SessionID != ""
}

// A foreign marker dispatches to its existing owner, never grants access here.
// An ordinary legacy projection without a native journal cannot be adopted by
// inference; deployment must finish the explicit legacy cutover first.
func (s *VoiceGRPC) findOrdinaryRoom(ctx context.Context, roomID string) (*ordinarycall.Room, bool, error) {
	if s == nil || s.OrdinaryCalls == nil {
		return nil, false, nil
	}
	if s.Calls == nil || s.OrdinaryCalls.Journal == nil || s.OrdinaryCalls.Projection == nil {
		return nil, true, status.Error(codes.Unavailable, "ordinary authority is incomplete")
	}
	roomID = strings.TrimSpace(roomID)
	cached, cacheErr := s.Calls.GetCall(ctx, roomID)
	if cacheErr == nil && foreignOrdinaryResource(cached) {
		return nil, false, nil
	}
	id, err := uuid.Parse(roomID)
	if err != nil || id == uuid.Nil || id.String() != roomID {
		if cacheErr == nil {
			return nil, true, status.Error(codes.Unavailable, "legacy ordinary call requires controlled cutover")
		}
		return nil, false, nil
	}
	r, err := s.OrdinaryCalls.Journal.Load(ctx, id)
	if errors.Is(err, ordinarycall.ErrNotFound) {
		if cacheErr == nil {
			return nil, true, status.Error(codes.Unavailable, "legacy ordinary call requires controlled cutover")
		}
		if !errors.Is(cacheErr, voicestore.ErrNotFound) {
			return nil, true, ordinaryErr(cacheErr)
		}
		return nil, false, nil
	}
	if err != nil {
		return nil, true, ordinaryErr(err)
	}
	if cacheErr == nil && cached.OrdinaryOperationID != "" && cached.OrdinaryOperationID != r.Intent.OperationID.String() {
		return nil, true, status.Error(codes.Unavailable, "ordinary projection ownership mismatch")
	}
	return r, true, nil
}

func ordinaryDecisionAt(s *VoiceGRPC, r *ordinarycall.Room) time.Time {
	if !r.TransactionTime.IsZero() {
		return r.TransactionTime
	}
	return s.now()
}

func (s *VoiceGRPC) ordinaryProjectedCall(ctx context.Context, r *ordinarycall.Room) (voicestore.Call, error) {
	if r.Stage == ordinarycall.StageClosed {
		return voicestore.Call{}, ordinarycall.ErrAdmissionAborted
	}
	if r.Stage != ordinarycall.StageActive && r.Stage != ordinarycall.StageRinging {
		return voicestore.Call{}, ordinarycall.ErrCleanupPending
	}
	if err := s.OrdinaryCalls.Projection.Apply(ctx, r); err != nil {
		return voicestore.Call{}, err
	}
	if err := s.OrdinaryCalls.Journal.MarkProjected(ctx, r.Intent.RoomID, r.Version); err != nil {
		return voicestore.Call{}, err
	}
	call, err := s.Calls.GetCall(ctx, r.Intent.RoomID.String())
	if err != nil {
		return voicestore.Call{}, err
	}
	if call.OrdinaryOperationID != r.Intent.OperationID.String() || call.OrdinaryVersion < r.Version || foreignOrdinaryResource(call) {
		return voicestore.Call{}, ordinarycall.ErrUnavailable
	}
	expected, err := ordinaryCallProjection(r)
	if err != nil {
		return voicestore.Call{}, err
	}
	if call.Status != expected.Status || call.LivekitRoomName != expected.LivekitRoomName || call.ChatID != expected.ChatID || call.SessionKind != expected.SessionKind || len(call.States) != len(expected.States) {
		return voicestore.Call{}, ordinarycall.ErrUnavailable
	}
	for profile := range expected.States {
		if _, ok := call.States[profile]; !ok || call.OrdinaryMediaIdentities[profile] != expected.OrdinaryMediaIdentities[profile] {
			return voicestore.Call{}, ordinarycall.ErrUnavailable
		}
	}
	return call, nil
}

func (s *VoiceGRPC) ordinaryActor(ctx context.Context, r *ordinarycall.Room, profile string) (ordinarycall.Identity, error) {
	id, err := s.ordinaryIdentity(ctx, profile, true)
	if err != nil {
		return id, err
	}
	m, exists := r.Members[profile]
	if !exists || m.AccountID != id.AccountID {
		return id, status.Error(codes.PermissionDenied, "not an ordinary call participant")
	}
	if err := s.requireOrdinaryEpoch(ctx, id); err != nil {
		return id, err
	}
	return id, nil
}

func (s *VoiceGRPC) acceptOrdinaryCall(ctx context.Context, r *ordinarycall.Room, profile string) (*callsv1.AcceptCallResponse, error) {
	id, err := s.ordinaryActor(ctx, r, profile)
	if err != nil {
		return nil, err
	}
	if r.Intent.Group || r.Intent.Callee == nil || r.Intent.Callee.ProfileID != id.ProfileID {
		return nil, status.Error(codes.PermissionDenied, "only callee can accept call")
	}
	check := func(ctx context.Context) error {
		if err := s.ensureSdkConversionAdmission(ctx, profile); err != nil {
			return err
		}
		if err := s.requireOrdinaryEpoch(ctx, id); err != nil {
			return err
		}
		caller := r.Members[r.Intent.Initiator.ProfileID.String()]
		current, err := s.ordinaryIdentity(ctx, caller.ProfileID.String(), false)
		if err != nil {
			return err
		}
		if current.AccountID != caller.AccountID {
			return status.Error(codes.PermissionDenied, "ordinary caller account changed")
		}
		if err := s.ensureSdkConversionAdmission(ctx, caller.ProfileID.String()); err != nil {
			return err
		}
		return s.requireOrdinaryEpoch(ctx, caller.Identity)
	}
	if err := check(ctx); err != nil {
		return nil, ordinaryErr(err)
	}
	op := uuid.New()
	committed, err := s.OrdinaryCalls.Admit(ctx, r.Intent.RoomID, op, func(r *ordinarycall.Room) error { return r.BeginVerifiedAccept(op, id, ordinaryDecisionAt(s, r)) }, check)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	call, err := ordinaryCallProjection(committed)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	return &callsv1.AcceptCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) joinOrdinaryCall(ctx context.Context, r *ordinarycall.Room, profile string) (*callsv1.JoinCallResponse, error) {
	id, err := s.ordinaryIdentity(ctx, profile, true)
	if err != nil {
		return nil, err
	}
	if !r.Intent.Group {
		if _, err := s.ordinaryActor(ctx, r, profile); err != nil {
			return nil, err
		}
		call, err := s.ordinaryProjectedCall(ctx, r)
		if err != nil {
			return nil, ordinaryErr(err)
		}
		return &callsv1.JoinCallResponse{CallSession: callToProto(call)}, nil
	}
	if s.ChatMembers == nil {
		return nil, status.Error(codes.Unavailable, "Chat membership authority unavailable")
	}
	check := func(ctx context.Context) error {
		if err := s.ensureSdkConversionAdmission(ctx, profile); err != nil {
			return err
		}
		if err := s.ensureChatMember(ctx, r.Intent.ChatID.String(), profile); err != nil {
			return err
		}
		if err := s.requireOrdinaryEpoch(ctx, id); err != nil {
			return err
		}
		if existing, ok := r.Members[profile]; ok && existing.State == ordinarycall.MemberJoined {
			return s.requireOrdinaryEpoch(ctx, existing.Identity)
		}
		return nil
	}
	if err := check(ctx); err != nil {
		return nil, ordinaryErr(err)
	}
	op := uuid.New()
	committed, err := s.OrdinaryCalls.Admit(ctx, r.Intent.RoomID, op, func(r *ordinarycall.Room) error { return r.BeginJoin(op, id, ordinaryDecisionAt(s, r)) }, check)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	call, err := ordinaryCallProjection(committed)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	return &callsv1.JoinCallResponse{CallSession: callToProto(call)}, nil
}

func (s *VoiceGRPC) terminateOrdinaryCall(ctx context.Context, r *ordinarycall.Room, profile, kind string) (*ordinarycall.Room, error) {
	id, err := s.ordinaryActor(ctx, r, profile)
	if err != nil {
		return nil, err
	}
	result, err := s.OrdinaryCalls.Change(ctx, r.Intent.RoomID, func(r *ordinarycall.Room) error {
		now := ordinaryDecisionAt(s, r)
		switch kind {
		case "decline":
			return r.DecideDecline(id.ProfileID, now)
		case "leave":
			return r.DecideLeave(id.ProfileID, now)
		default:
			return r.DecideEnd(id.ProfileID, "hangup", now)
		}
	})
	return result, ordinaryErr(err)
}

func (s *VoiceGRPC) ordinaryJoinToken(ctx context.Context, call voicestore.Call, profile string) (*callsv1.GetJoinTokenResponse, error) {
	roomID, err := uuid.Parse(call.RoomID)
	if err != nil || roomID == uuid.Nil || roomID.String() != call.RoomID {
		return nil, status.Error(codes.Unavailable, "invalid ordinary projection identity")
	}
	r, err := s.OrdinaryCalls.Journal.Load(ctx, roomID)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	id, err := s.ordinaryActor(ctx, r, profile)
	if err != nil {
		return nil, err
	}
	binding, err := r.MediaBinding(id.ProfileID)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	if err := s.requireOrdinaryEpoch(ctx, r.Members[profile].Identity); err != nil {
		return nil, ordinaryErr(err)
	}
	if call.OrdinaryOperationID != binding.RoomOperationID.String() {
		return nil, status.Error(codes.Unavailable, "ordinary media ownership mismatch")
	}
	canPublish, err := s.voicePublishGrant(ctx, call, profile)
	if err != nil {
		return nil, err
	}
	if s.Tokens == nil {
		return nil, status.Error(codes.Unavailable, "LiveKit token issuer unavailable")
	}
	authorizer, ok := s.OrdinaryCalls.Journal.(ordinarycall.MediaAuthorizer)
	if !ok {
		return nil, status.Error(codes.Unavailable, "ordinary media authority unavailable")
	}
	roomName, err := authorizer.AuthorizeMedia(ctx, binding)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	jwt, expiresAt, err := s.Tokens.JoinToken(binding.MediaIdentity, roomName, canPublish, s.now())
	if err != nil {
		return nil, status.Error(codes.Unavailable, "ordinary media token unavailable")
	}
	if err := s.ensureSdkConversionAdmission(ctx, profile); err != nil {
		return nil, err
	}
	return &callsv1.GetJoinTokenResponse{Jwt: jwt, ExpiresAt: timestamppb.New(expiresAt), LivekitUrl: s.Tokens.LivekitURL()}, nil
}
