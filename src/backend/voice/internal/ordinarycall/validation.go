package ordinarycall

import "github.com/google/uuid"

// Persisted documents are authority, so missing/contradictory ownership is an
// unavailable journal rather than an empty room or guessed participant identity.
func validateRoom(r *Room) error {
	if r == nil || r.Version == 0 || r.CreatedAt.IsZero() || r.Members == nil || r.Admissions == nil {
		return ErrInvalidIntent
	}
	i := r.Intent
	if i.OperationID == uuid.Nil || i.RoomID == uuid.Nil || i.ChatID == uuid.Nil || !validIdentity(i.Initiator) ||
		(i.MediaKind != 1 && i.MediaKind != 2) || i.Group && i.Callee != nil || !i.Group && (i.Callee == nil || !validIdentity(*i.Callee) ||
		i.Callee.SessionEpoch != 0 || i.Callee.AccountID == i.Initiator.AccountID || i.Callee.ProfileID == i.Initiator.ProfileID) {
		return ErrInvalidIntent
	}
	name := "voice-dm-"
	if i.Group {
		name = "voice-group-"
	}
	if r.LiveKitRoomName != name+i.RoomID.String() {
		return ErrInvalidIntent
	}
	switch r.Stage {
	case StagePreparing, StageRinging, StageActive, StageAborting, StageClosing, StageClosed:
	default:
		return ErrInvalidIntent
	}
	if (r.Stage == StageRinging || r.Stage == StageActive) && r.ReadyAt.IsZero() {
		return ErrInvalidIntent
	}
	if r.Stage == StageRinging && (i.Group || r.ExpiresAt.IsZero() || !r.ExpiresAt.After(r.ReadyAt)) {
		return ErrInvalidIntent
	}
	for key, m := range r.Members {
		if key != m.ProfileID.String() || !validMember(m) {
			return ErrInvalidIntent
		}
	}
	for _, m := range r.RetiredMembers {
		if !validMember(m) {
			return ErrInvalidIntent
		}
	}
	start, ok := r.Admissions[i.OperationID.String()]
	if !ok || start.Kind != AdmissionStart || start.ActorID != i.Initiator.ProfileID {
		return ErrInvalidIntent
	}
	for key, a := range r.Admissions {
		if a.ID == uuid.Nil || key != a.ID.String() || a.ActorID == uuid.Nil || a.ActorEpoch < 0 || a.CreatedAt.IsZero() || len(a.Generations) == 0 {
			return ErrInvalidIntent
		}
		switch a.Kind {
		case AdmissionStart, AdmissionAccept, AdmissionJoin:
		default:
			return ErrInvalidIntent
		}
		switch a.State {
		case AdmissionPending, AdmissionCommitted, AdmissionAborted:
		default:
			return ErrInvalidIntent
		}
		for profile, generation := range a.Generations {
			p, err := uuid.Parse(profile)
			if err != nil || p == uuid.Nil || p.String() != profile || generation == uuid.Nil {
				return ErrInvalidIntent
			}
		}
	}
	ids := map[uuid.UUID]bool{}
	var roster uint64
	for _, e := range r.Events {
		if e.ID == uuid.Nil || ids[e.ID] || e.OccurredAt.IsZero() {
			return ErrInvalidIntent
		}
		ids[e.ID] = true
		switch e.Kind {
		case EventParticipantJoined, EventParticipantLeft:
			roster++
			if e.ProfileID == uuid.Nil || e.RosterVersion != roster {
				return ErrInvalidIntent
			}
		case EventIncoming, EventAccepted, EventDeclined, EventMissed, EventEnded, EventCallStarted:
			if e.RosterVersion != 0 {
				return ErrInvalidIntent
			}
		default:
			return ErrInvalidIntent
		}
		for _, profile := range e.AudienceIDs {
			if profile == uuid.Nil {
				return ErrInvalidIntent
			}
		}
	}
	if roster != r.RosterVersion {
		return ErrInvalidIntent
	}
	for _, e := range r.Effects {
		if e.ID == uuid.Nil || ids[e.ID] || e.OperationID == uuid.Nil || e.CreatedAt.IsZero() {
			return ErrInvalidIntent
		}
		ids[e.ID] = true
		switch e.Kind {
		case EffectEnsure, EffectClose:
			if e.OperationID != i.OperationID {
				return ErrInvalidIntent
			}
		case EffectRemoveMember, EffectReleaseFence:
			if e.ProfileID == uuid.Nil || e.Generation == uuid.Nil {
				return ErrInvalidIntent
			}
			if e.Kind == EffectRemoveMember && e.Participant != "ov:"+e.ProfileID.String()+":"+e.Generation.String() {
				return ErrInvalidIntent
			}
		default:
			return ErrInvalidIntent
		}
	}
	return nil
}

func validMember(m Member) bool {
	if !validIdentity(m.Identity) || m.Generation == uuid.Nil || m.FenceOperationID == uuid.Nil || m.MediaIdentity != "ov:"+m.ProfileID.String()+":"+m.Generation.String() {
		return false
	}
	switch m.State {
	case MemberPending, MemberReserved, MemberJoined, MemberLeaving, MemberLeft, MemberAborted:
		return true
	default:
		return false
	}
}
