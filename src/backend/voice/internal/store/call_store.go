package store

import (
	"context"
	"errors"
	"sync"
	"time"

	callsv1 "voice.app/voice/calls/v1"
)

var (
	ErrNotFound       = errors.New("call not found")
	ErrActiveCall     = errors.New("profile already has active call")
	ErrInvalidState   = errors.New("invalid call state")
	ErrNotParticipant = errors.New("profile is not a call participant")
)

type ParticipantState struct {
	ProfileID       string `json:"profile_id"`
	IsMuted         bool   `json:"is_muted"`
	IsDeafened      bool   `json:"is_deafened"`
	IsVideoOn       bool   `json:"is_video_on"`
	IsScreenSharing bool   `json:"is_screen_sharing"`
}

type Call struct {
	RoomID             string                      `json:"room_id"`
	LivekitRoomName    string                      `json:"livekit_room_name"`
	ChatID             string                      `json:"chat_id"`
	InitiatorProfileID string                      `json:"initiator_profile_id"`
	CalleeProfileID    string                      `json:"callee_profile_id"`
	MediaKind          callsv1.CallMediaKind       `json:"media_kind"`
	Status             callsv1.CallStatus          `json:"status"`
	StartedAt          time.Time                   `json:"started_at"`
	ExpiresAt          time.Time                   `json:"expires_at"`
	EndedAt            time.Time                   `json:"ended_at,omitempty"`
	States             map[string]ParticipantState `json:"states"`
}

func (c Call) ProfileIDs() []string {
	return []string{c.InitiatorProfileID, c.CalleeProfileID}
}

func (c Call) IsParticipant(profileID string) bool {
	return profileID != "" && (profileID == c.InitiatorProfileID || profileID == c.CalleeProfileID)
}

func (c Call) IsActiveForProfile(profileID string) bool {
	return c.IsParticipant(profileID) && (c.Status == callsv1.CallStatus_CALL_STATUS_RINGING || c.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE)
}

type VoiceStatePatch struct {
	IsMuted    *bool
	IsDeafened *bool
	IsVideoOn  *bool
}

type CallStore interface {
	CreateCall(ctx context.Context, call Call) (Call, error)
	GetCall(ctx context.Context, roomID string) (Call, error)
	GetActiveCall(ctx context.Context, profileID string) (Call, error)
	SetStatus(ctx context.Context, roomID string, status callsv1.CallStatus, endedAt time.Time) (Call, error)
	UpdateVoiceState(ctx context.Context, roomID, profileID string, patch VoiceStatePatch) (Call, ParticipantState, error)
	ListExpiredRinging(ctx context.Context, now time.Time) ([]Call, error)
}

type MemoryCallStore struct {
	mu    sync.Mutex
	calls map[string]Call
}

func NewMemoryCallStore() *MemoryCallStore {
	return &MemoryCallStore{calls: map[string]Call{}}
}

func (s *MemoryCallStore) CreateCall(_ context.Context, call Call) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.calls {
		if existing.IsActiveForProfile(call.InitiatorProfileID) || existing.IsActiveForProfile(call.CalleeProfileID) {
			return Call{}, ErrActiveCall
		}
	}
	if call.States == nil {
		call.States = defaultStates(call)
	}
	s.calls[call.RoomID] = call
	return call, nil
}

func (s *MemoryCallStore) GetCall(_ context.Context, roomID string) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ErrNotFound
	}
	return call, nil
}

func (s *MemoryCallStore) GetActiveCall(_ context.Context, profileID string) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, call := range s.calls {
		if call.IsActiveForProfile(profileID) {
			return call, nil
		}
	}
	return Call{}, ErrNotFound
}

func (s *MemoryCallStore) SetStatus(_ context.Context, roomID string, status callsv1.CallStatus, endedAt time.Time) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ErrNotFound
	}
	call.Status = status
	call.EndedAt = endedAt
	s.calls[roomID] = call
	return call, nil
}

func (s *MemoryCallStore) UpdateVoiceState(_ context.Context, roomID, profileID string, patch VoiceStatePatch) (Call, ParticipantState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ParticipantState{}, ErrNotFound
	}
	if !call.IsParticipant(profileID) {
		return Call{}, ParticipantState{}, ErrNotParticipant
	}
	if call.States == nil {
		call.States = defaultStates(call)
	}
	state := call.States[profileID]
	if patch.IsMuted != nil {
		state.IsMuted = *patch.IsMuted
	}
	if patch.IsDeafened != nil {
		state.IsDeafened = *patch.IsDeafened
	}
	if patch.IsVideoOn != nil {
		state.IsVideoOn = *patch.IsVideoOn
	}
	call.States[profileID] = state
	s.calls[roomID] = call
	return call, state, nil
}

func (s *MemoryCallStore) ListExpiredRinging(_ context.Context, now time.Time) ([]Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Call
	for _, call := range s.calls {
		if call.Status == callsv1.CallStatus_CALL_STATUS_RINGING && !call.ExpiresAt.IsZero() && now.After(call.ExpiresAt) {
			out = append(out, call)
		}
	}
	return out, nil
}

func defaultStates(call Call) map[string]ParticipantState {
	return map[string]ParticipantState{
		call.InitiatorProfileID: {ProfileID: call.InitiatorProfileID, IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO},
		call.CalleeProfileID:    {ProfileID: call.CalleeProfileID, IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO},
	}
}
