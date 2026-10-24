package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/ordinarycall"
	voicestore "voice/backend/voice/internal/store"
)

type ordinaryProjection struct {
	Calls   voicestore.OrdinaryProjectionStore
	Journal ordinarycall.Journal
}

func NewOrdinaryProjection(calls voicestore.OrdinaryProjectionStore, journal ordinarycall.Journal) ordinarycall.Projection {
	return &ordinaryProjection{Calls: calls, Journal: journal}
}

func (p *ordinaryProjection) Apply(ctx context.Context, room *ordinarycall.Room) error {
	if p == nil || p.Calls == nil || p.Journal == nil || room == nil {
		return ordinarycall.ErrUnavailable
	}
	call, err := ordinaryCallProjection(room)
	if err != nil {
		return err
	}
	_, err = p.Calls.ApplyOrdinaryProjection(ctx, call, func(ctx context.Context, roomID string) error {
		id, err := uuid.Parse(roomID)
		if err != nil || id.String() != roomID || id == uuid.Nil {
			return ordinarycall.ErrUnavailable
		}
		prior, err := p.Journal.Load(ctx, id)
		if err != nil {
			return err
		}
		if prior.Stage != ordinarycall.StageClosed || prior.HasPendingClose() {
			return ordinarycall.ErrConflict
		}
		return nil
	})
	return err
}

func ordinaryCallProjection(r *ordinarycall.Room) (voicestore.Call, error) {
	if r == nil || r.Stage == ordinarycall.StagePreparing {
		return voicestore.Call{}, ordinarycall.ErrEffectPending
	}
	call := voicestore.Call{
		RoomID: r.Intent.RoomID.String(), LivekitRoomName: r.LiveKitRoomName, ChatID: r.Intent.ChatID.String(),
		OrdinaryOperationID: r.Intent.OperationID.String(), OrdinaryVersion: r.Version,
		OrdinaryMediaIdentities: map[string]string{}, States: map[string]voicestore.ParticipantState{},
		InitiatorProfileID: r.Intent.Initiator.ProfileID.String(), MediaKind: callsv1.CallMediaKind(r.Intent.MediaKind),
		SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_CALL,
		Status:      callsv1.CallStatus_CALL_STATUS_ENDED, StartedAt: r.ReadyAt, ExpiresAt: r.ExpiresAt, EndedAt: r.EndedAt,
	}
	if r.Intent.Group {
		call.SessionKind = callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
	}
	if r.Intent.Callee != nil {
		call.CalleeProfileID = r.Intent.Callee.ProfileID.String()
	}
	if r.Stage == ordinarycall.StageRinging {
		call.Status = callsv1.CallStatus_CALL_STATUS_RINGING
	}
	if r.Stage == ordinarycall.StageActive {
		call.Status = callsv1.CallStatus_CALL_STATUS_ACTIVE
	}
	if r.Stage == ordinarycall.StageClosing || r.Stage == ordinarycall.StageClosed {
		switch r.TerminalReason {
		case "declined":
			call.Status = callsv1.CallStatus_CALL_STATUS_DECLINED
		case "missed":
			call.Status = callsv1.CallStatus_CALL_STATUS_MISSED
		}
	}
	for profile, member := range r.Members {
		call.OrdinaryMediaIdentities[profile] = member.MediaIdentity
		if member.State == ordinarycall.MemberJoined || r.Stage == ordinarycall.StageRinging && member.State == ordinarycall.MemberReserved {
			call.States[profile] = voicestore.ParticipantState{ProfileID: profile, IsVideoOn: r.Intent.MediaKind == int32(callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO)}
		}
	}
	return call, nil
}
