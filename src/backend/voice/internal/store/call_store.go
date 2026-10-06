package store

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	callsv1 "voice.app/voice/calls/v1"
)

const (
	MaxGroupVoiceParticipants    = 32
	MaxVoiceRoomParticipants     = 32
	MaxSpaceProVoiceParticipants = 128
	MaxScreenSharesPerRoom       = 3
)

var (
	ErrNotFound             = errors.New("call not found")
	ErrActiveCall           = errors.New("profile already has active call")
	ErrInvalidState         = errors.New("invalid call state")
	ErrNotParticipant       = errors.New("profile is not a call participant")
	ErrRoomFull             = errors.New("voice room is full")
	ErrScreenShareLimit     = errors.New("screen share limit reached")
	ErrNotScreenSharing     = errors.New("profile is not screen sharing")
	ErrScreenShareDenied    = errors.New("screen share not permitted")
	ErrOperationConflict    = errors.New("operation id conflicts with a different request")
	ErrSpaceMediaStaleGrant = errors.New("space media grant is below the current authority floor")
	ErrSpaceMediaTransition = errors.New("space media participant transition is in progress")
	// ErrMoveContention means the bounded Redis CAS retry budget was exhausted.
	// It is deliberately distinct from a dependency outage so the transport can
	// expose the same safe retryable result without leaking Redis details.
	ErrMoveContention = errors.New("voice room move contention")
)

// VoiceRoomMoveRequest is the canonical, actor-bound mutation for moving one
// participant between two already-authorized Space voice rooms.  The service
// performs all discovery and permission checks before it reaches the store.
type VoiceRoomMoveRequest struct {
	ActorProfileID       string
	ParticipantProfileID string
	OperationID          string
	FromVoiceRoomID      string
	ToVoiceRoomID        string
	SpaceID              string
	MaxParticipants      int
	DestinationRoomID    string
	Now                  time.Time
}

// VoiceRoomMoveResult retains enough data for an idempotent caller to build
// the same lifecycle receipt without repeating the roster mutation.
type VoiceRoomMoveResult struct {
	Source      Call
	Destination Call
	Replayed    bool
}

type ScreenShareEntry struct {
	ProfileID string `json:"profile_id"`
	StreamID  string `json:"stream_id"`
}

type ParticipantState struct {
	ProfileID       string `json:"profile_id"`
	IsMuted         bool   `json:"is_muted"`
	IsDeafened      bool   `json:"is_deafened"`
	IsVideoOn       bool   `json:"is_video_on"`
	IsScreenSharing bool   `json:"is_screen_sharing"`
	IsCommander     bool   `json:"is_commander"`
	HandRaised      bool   `json:"hand_raised"`
	HasFloor        bool   `json:"has_floor"`
	IsBroadcasting  bool   `json:"is_broadcasting"`
}

type SpaceMediaGrant struct {
	SessionEpoch    uint64 `json:"session_epoch"`
	AccessEpoch     uint64 `json:"access_epoch"`
	PolicyEpoch     uint64 `json:"policy_epoch"`
	CanJoin         bool   `json:"can_join"`
	CanPublishAudio bool   `json:"can_publish_audio"`
	CanSubscribe    bool   `json:"can_subscribe"`
}

type SpaceMediaParticipant struct {
	ProfileID  string          `json:"profile_id"`
	Identity   string          `json:"identity"`
	Generation string          `json:"generation"`
	Issued     SpaceMediaGrant `json:"issued"`
	Reconciled SpaceMediaGrant `json:"reconciled"`
	Revoking   bool            `json:"revoking"`
}

type SpaceMediaEpochKind uint8

const (
	SpaceAccessEpoch SpaceMediaEpochKind = iota + 1
	RolePolicyEpoch
)

type SpaceMediaEpochFloors struct {
	AccessEpoch uint64
	PolicyEpoch uint64
}

// SpaceMediaEpochProgress distinguishes an observed authority fence from a
// completed reconciliation pass. An observed epoch must never be treated as
// fully applied until every indexed active room has been scanned.
type SpaceMediaEpochProgress struct {
	Observed   SpaceMediaEpochFloors
	Reconciled SpaceMediaEpochFloors
}

type Call struct {
	RoomID             string                           `json:"room_id"`
	LivekitRoomName    string                           `json:"livekit_room_name"`
	ChatID             string                           `json:"chat_id"`
	ManagedGameSession bool                             `json:"managed_game_session,omitempty"`
	ApplicationID      string                           `json:"application_id,omitempty"`
	EnvironmentID      string                           `json:"environment_id,omitempty"`
	SessionID          string                           `json:"session_id,omitempty"`
	VoiceRoomID        string                           `json:"voice_room_id,omitempty"`
	SpaceID            string                           `json:"space_id,omitempty"`
	SessionKind        callsv1.VoiceSessionKind         `json:"session_kind,omitempty"`
	InitiatorProfileID string                           `json:"initiator_profile_id"`
	CalleeProfileID    string                           `json:"callee_profile_id"`
	MediaKind          callsv1.CallMediaKind            `json:"media_kind"`
	Status             callsv1.CallStatus               `json:"status"`
	StartedAt          time.Time                        `json:"started_at"`
	ExpiresAt          time.Time                        `json:"expires_at"`
	EndedAt            time.Time                        `json:"ended_at,omitempty"`
	States             map[string]ParticipantState      `json:"states"`
	ScreenShares       []ScreenShareEntry               `json:"screen_shares,omitempty"`
	SpaceMedia         map[string]SpaceMediaParticipant `json:"space_media,omitempty"`
}

func (c Call) IsGroupVoice() bool {
	return c.SessionKind == callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
}

func (c Call) IsVoiceRoom() bool {
	return c.SessionKind == callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM
}

func (c Call) isOpenVoiceSession() bool {
	return c.IsGroupVoice() || c.IsVoiceRoom()
}

func (c Call) ProfileIDs() []string {
	if c.isOpenVoiceSession() {
		ids := make([]string, 0, len(c.States))
		for id := range c.States {
			ids = append(ids, id)
		}
		return ids
	}
	return []string{c.InitiatorProfileID, c.CalleeProfileID}
}

func (c Call) IsParticipant(profileID string) bool {
	if profileID == "" {
		return false
	}
	if c.isOpenVoiceSession() {
		_, ok := c.States[profileID]
		return ok
	}
	return profileID == c.InitiatorProfileID || profileID == c.CalleeProfileID
}

func (c Call) IsActiveForProfile(profileID string) bool {
	if !c.IsParticipant(profileID) {
		return false
	}
	if c.isOpenVoiceSession() {
		return c.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE
	}
	return c.Status == callsv1.CallStatus_CALL_STATUS_RINGING || c.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE
}

type VoiceStatePatch struct {
	IsMuted        *bool
	IsDeafened     *bool
	IsVideoOn      *bool
	IsCommander    *bool
	HandRaised     *bool
	HasFloor       *bool
	IsBroadcasting *bool
}

type CallStore interface {
	CreateCall(ctx context.Context, call Call) (Call, error)
	GetCall(ctx context.Context, roomID string) (Call, error)
	GetActiveCall(ctx context.Context, profileID string) (Call, error)
	GetActiveGroupCallForChat(ctx context.Context, chatID string) (Call, error)
	SetStatus(ctx context.Context, roomID string, status callsv1.CallStatus, endedAt time.Time) (Call, error)
	AddParticipant(ctx context.Context, roomID, profileID string, maxParticipants int) (Call, error)
	RemoveParticipant(ctx context.Context, roomID, profileID string) (Call, error)
	FindVoiceRoomMove(ctx context.Context, req VoiceRoomMoveRequest) (VoiceRoomMoveResult, bool, error)
	MoveVoiceRoomParticipant(ctx context.Context, req VoiceRoomMoveRequest) (VoiceRoomMoveResult, error)
	GetCallByVoiceRoomID(ctx context.Context, voiceRoomID string) (Call, error)
	UpdateVoiceState(ctx context.Context, roomID, profileID string, patch VoiceStatePatch) (Call, ParticipantState, error)
	StartScreenShare(ctx context.Context, roomID, profileID, streamID string) (Call, ScreenShareEntry, error)
	StopScreenShare(ctx context.Context, roomID, profileID, streamID string) (Call, error)
	StopScreenSharesForProfile(ctx context.Context, roomID, profileID string) (Call, error)
	ListExpiredRinging(ctx context.Context, now time.Time) ([]Call, error)
	ListActiveSpaceVoiceCalls(ctx context.Context, spaceID string) ([]Call, error)
	ListActiveSpaceIDs(ctx context.Context) ([]string, error)
	RaiseSpaceMediaEpochFloor(ctx context.Context, spaceID string, kind SpaceMediaEpochKind, epoch uint64) (SpaceMediaEpochFloors, error)
	GetSpaceMediaEpochFloors(ctx context.Context, spaceID string) (SpaceMediaEpochFloors, error)
	GetSpaceMediaEpochProgress(ctx context.Context, spaceID string) (SpaceMediaEpochProgress, error)
	MarkSpaceMediaEpochReconciled(ctx context.Context, spaceID string, kind SpaceMediaEpochKind, epoch uint64) (SpaceMediaEpochProgress, error)
	AdmitSpaceMediaParticipant(ctx context.Context, roomID string, participant SpaceMediaParticipant, maxParticipants int) (Call, error)
	BeginSpaceMediaRevocation(ctx context.Context, roomID, profileID, identity, generation string) (Call, bool, error)
	CompleteSpaceMediaRevocation(ctx context.Context, roomID, profileID, identity, generation string) (Call, bool, error)
	ReconcileSpaceMediaParticipant(ctx context.Context, roomID, profileID, identity, generation string, grant SpaceMediaGrant) (Call, bool, error)
}

type MemoryCallStore struct {
	mu         sync.Mutex
	calls      map[string]Call
	moves      map[string]memoryVoiceRoomMove
	floors     map[string]SpaceMediaEpochFloors
	reconciled map[string]SpaceMediaEpochFloors
}

type memoryVoiceRoomMove struct {
	req    VoiceRoomMoveRequest
	result VoiceRoomMoveResult
}

func NewMemoryCallStore() *MemoryCallStore {
	return &MemoryCallStore{calls: map[string]Call{}, moves: map[string]memoryVoiceRoomMove{}, floors: map[string]SpaceMediaEpochFloors{}, reconciled: map[string]SpaceMediaEpochFloors{}}
}

func (s *MemoryCallStore) CreateCall(_ context.Context, call Call) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, exists := s.calls[call.RoomID]; exists && (call.ManagedGameSession || existing.ManagedGameSession) {
		return Call{}, ErrInvalidState
	}
	if err := s.ensureNoActiveCallLocked(call.InitiatorProfileID); err != nil {
		return Call{}, err
	}
	if !call.isOpenVoiceSession() && call.CalleeProfileID != "" {
		if err := s.ensureNoActiveCallLocked(call.CalleeProfileID); err != nil {
			return Call{}, err
		}
	}
	if call.States == nil {
		call.States = defaultStates(call)
	}
	s.calls[call.RoomID] = call
	return call, nil
}

func (s *MemoryCallStore) ensureNoActiveCallLocked(profileID string) error {
	for _, existing := range s.calls {
		if existing.IsActiveForProfile(profileID) {
			return ErrActiveCall
		}
	}
	return nil
}

func (s *MemoryCallStore) ensureNoActiveCallExceptLocked(profileID, roomID string) error {
	for _, existing := range s.calls {
		if existing.RoomID != roomID && existing.IsActiveForProfile(profileID) {
			return ErrActiveCall
		}
	}
	return nil
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

func (s *MemoryCallStore) GetActiveGroupCallForChat(_ context.Context, chatID string) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, call := range s.calls {
		if call.IsGroupVoice() &&
			!call.ManagedGameSession &&
			call.ChatID == chatID &&
			call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			return call, nil
		}
	}
	return Call{}, ErrNotFound
}

func (s *MemoryCallStore) AddParticipant(_ context.Context, roomID, profileID string, maxParticipants int) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ErrNotFound
	}
	if !call.isOpenVoiceSession() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return Call{}, ErrInvalidState
	}
	if call.IsParticipant(profileID) {
		return call, nil
	}
	if err := s.ensureNoActiveCallLocked(profileID); err != nil {
		return Call{}, err
	}
	if len(call.States) >= maxParticipants {
		return Call{}, ErrRoomFull
	}
	if call.States == nil {
		call.States = map[string]ParticipantState{}
	}
	call.States[profileID] = ParticipantState{
		ProfileID: profileID,
		IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO,
	}
	s.calls[roomID] = call
	return call, nil
}

func (s *MemoryCallStore) RemoveParticipant(_ context.Context, roomID, profileID string) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ErrNotFound
	}
	if !call.isOpenVoiceSession() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return Call{}, ErrInvalidState
	}
	if !call.IsParticipant(profileID) {
		return Call{}, ErrNotParticipant
	}
	delete(call.States, profileID)
	delete(call.SpaceMedia, profileID)
	call = removeScreenSharesForProfile(call, profileID)
	s.calls[roomID] = call
	return call, nil
}

func (s *MemoryCallStore) MoveVoiceRoomParticipant(_ context.Context, req VoiceRoomMoveRequest) (VoiceRoomMoveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := req.ActorProfileID + ":" + req.OperationID
	if prior, ok := s.moves[key]; ok {
		if !sameVoiceRoomMoveRequest(prior.req, req) {
			return VoiceRoomMoveResult{}, ErrOperationConflict
		}
		prior.result.Replayed = true
		return prior.result, nil
	}
	source, ok := s.callsByVoiceRoomLocked(req.FromVoiceRoomID)
	if !ok {
		return VoiceRoomMoveResult{}, ErrNotFound
	}
	if !source.IsParticipant(req.ParticipantProfileID) {
		return VoiceRoomMoveResult{}, ErrNotParticipant
	}
	if source.SpaceID != req.SpaceID {
		return VoiceRoomMoveResult{}, ErrInvalidState
	}
	destination, exists := s.callsByVoiceRoomLocked(req.ToVoiceRoomID)
	if exists {
		if !destination.IsVoiceRoom() || destination.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE || destination.SpaceID != req.SpaceID {
			return VoiceRoomMoveResult{}, ErrInvalidState
		}
		if !destination.IsParticipant(req.ParticipantProfileID) && len(destination.States) >= req.MaxParticipants {
			return VoiceRoomMoveResult{}, ErrRoomFull
		}
	} else {
		destination = Call{
			RoomID:             req.DestinationRoomID,
			LivekitRoomName:    "voice-room-" + req.ToVoiceRoomID,
			VoiceRoomID:        req.ToVoiceRoomID,
			SpaceID:            req.SpaceID,
			SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
			InitiatorProfileID: req.ParticipantProfileID,
			MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
			Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
			States: map[string]ParticipantState{req.ParticipantProfileID: {
				ProfileID: req.ParticipantProfileID,
			}},
		}
	}
	delete(source.States, req.ParticipantProfileID)
	source = removeScreenSharesForProfile(source, req.ParticipantProfileID)
	if len(source.States) == 0 {
		source.Status = callsv1.CallStatus_CALL_STATUS_ENDED
		source.EndedAt = req.Now
	}
	if exists && !destination.IsParticipant(req.ParticipantProfileID) {
		destination.States[req.ParticipantProfileID] = ParticipantState{ProfileID: req.ParticipantProfileID}
	}
	s.calls[source.RoomID] = source
	s.calls[destination.RoomID] = destination
	result := VoiceRoomMoveResult{Source: source, Destination: destination}
	s.moves[key] = memoryVoiceRoomMove{req: req, result: result}
	return result, nil
}

func (s *MemoryCallStore) FindVoiceRoomMove(_ context.Context, req VoiceRoomMoveRequest) (VoiceRoomMoveResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prior, ok := s.moves[req.ActorProfileID+":"+req.OperationID]
	if !ok {
		return VoiceRoomMoveResult{}, false, nil
	}
	if !sameVoiceRoomMoveRequest(prior.req, req) {
		return VoiceRoomMoveResult{}, false, ErrOperationConflict
	}
	prior.result.Replayed = true
	return prior.result, true, nil
}

func sameVoiceRoomMoveRequest(a, b VoiceRoomMoveRequest) bool {
	return a.ActorProfileID == b.ActorProfileID &&
		a.ParticipantProfileID == b.ParticipantProfileID &&
		a.OperationID == b.OperationID &&
		a.FromVoiceRoomID == b.FromVoiceRoomID &&
		a.ToVoiceRoomID == b.ToVoiceRoomID &&
		a.SpaceID == b.SpaceID
}

func (s *MemoryCallStore) callsByVoiceRoomLocked(voiceRoomID string) (Call, bool) {
	for _, call := range s.calls {
		if call.IsVoiceRoom() && call.VoiceRoomID == voiceRoomID && call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			return call, true
		}
	}
	return Call{}, false
}

func (s *MemoryCallStore) GetCallByVoiceRoomID(_ context.Context, voiceRoomID string) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, call := range s.calls {
		if call.IsVoiceRoom() &&
			call.VoiceRoomID == voiceRoomID &&
			call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
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
	if patch.IsCommander != nil {
		state.IsCommander = *patch.IsCommander
		if !*patch.IsCommander {
			state.IsBroadcasting = false
		}
	}
	if patch.HandRaised != nil {
		state.HandRaised = *patch.HandRaised
	}
	if patch.HasFloor != nil {
		state.HasFloor = *patch.HasFloor
	}
	if patch.IsBroadcasting != nil {
		state.IsBroadcasting = *patch.IsBroadcasting
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

func (s *MemoryCallStore) ListActiveSpaceVoiceCalls(_ context.Context, spaceID string) ([]Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var calls []Call
	for _, call := range s.calls {
		if call.IsVoiceRoom() && call.SpaceID == spaceID && call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			calls = append(calls, call)
		}
	}
	return calls, nil
}

func (s *MemoryCallStore) ListActiveSpaceIDs(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{})
	for _, call := range s.calls {
		if call.IsVoiceRoom() && call.SpaceID != "" && call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			seen[call.SpaceID] = struct{}{}
		}
	}
	spaces := make([]string, 0, len(seen))
	for id := range seen {
		spaces = append(spaces, id)
	}
	sort.Strings(spaces)
	return spaces, nil
}

func (s *MemoryCallStore) RaiseSpaceMediaEpochFloor(_ context.Context, spaceID string, kind SpaceMediaEpochKind, epoch uint64) (SpaceMediaEpochFloors, error) {
	if spaceID == "" || epoch == 0 || (kind != SpaceAccessEpoch && kind != RolePolicyEpoch) {
		return SpaceMediaEpochFloors{}, ErrInvalidState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	floors := s.floors[spaceID]
	if kind == SpaceAccessEpoch && epoch > floors.AccessEpoch {
		floors.AccessEpoch = epoch
	}
	if kind == RolePolicyEpoch && epoch > floors.PolicyEpoch {
		floors.PolicyEpoch = epoch
	}
	s.floors[spaceID] = floors
	return floors, nil
}

func (s *MemoryCallStore) GetSpaceMediaEpochFloors(_ context.Context, spaceID string) (SpaceMediaEpochFloors, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.floors[spaceID], nil
}

func (s *MemoryCallStore) GetSpaceMediaEpochProgress(_ context.Context, spaceID string) (SpaceMediaEpochProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SpaceMediaEpochProgress{Observed: s.floors[spaceID], Reconciled: s.reconciled[spaceID]}, nil
}

func (s *MemoryCallStore) MarkSpaceMediaEpochReconciled(_ context.Context, spaceID string, kind SpaceMediaEpochKind, epoch uint64) (SpaceMediaEpochProgress, error) {
	if spaceID == "" || epoch == 0 || (kind != SpaceAccessEpoch && kind != RolePolicyEpoch) {
		return SpaceMediaEpochProgress{}, ErrInvalidState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	observed := s.floors[spaceID]
	reconciled := s.reconciled[spaceID]
	observedEpoch, reconciledEpoch := observed.AccessEpoch, &reconciled.AccessEpoch
	if kind == RolePolicyEpoch {
		observedEpoch, reconciledEpoch = observed.PolicyEpoch, &reconciled.PolicyEpoch
	}
	if epoch > observedEpoch {
		return SpaceMediaEpochProgress{}, ErrSpaceMediaStaleGrant
	}
	if epoch > *reconciledEpoch {
		*reconciledEpoch = epoch
	}
	s.reconciled[spaceID] = reconciled
	return SpaceMediaEpochProgress{Observed: observed, Reconciled: reconciled}, nil
}

func (s *MemoryCallStore) AdmitSpaceMediaParticipant(_ context.Context, roomID string, participant SpaceMediaParticipant, maxParticipants int) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ErrNotFound
	}
	if !call.IsVoiceRoom() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE || participant.ProfileID == "" ||
		participant.Identity == "" || participant.Generation == "" || participant.Issued.SessionEpoch == 0 ||
		participant.Issued.AccessEpoch == 0 || participant.Issued.PolicyEpoch == 0 || !participant.Issued.CanJoin || !participant.Issued.CanSubscribe {
		return Call{}, ErrInvalidState
	}
	floors := s.floors[call.SpaceID]
	if participant.Issued.AccessEpoch < floors.AccessEpoch || participant.Issued.PolicyEpoch < floors.PolicyEpoch {
		return Call{}, ErrSpaceMediaStaleGrant
	}
	if current, exists := call.SpaceMedia[participant.ProfileID]; exists {
		if current.Generation == participant.Generation && current.Identity == participant.Identity && !current.Revoking {
			return call, nil
		}
		return Call{}, ErrSpaceMediaTransition
	}
	if err := s.ensureNoActiveCallExceptLocked(participant.ProfileID, roomID); err != nil {
		return Call{}, err
	}
	if len(call.States) >= maxParticipants {
		return Call{}, ErrRoomFull
	}
	if call.States == nil {
		call.States = map[string]ParticipantState{}
	}
	call.States[participant.ProfileID] = ParticipantState{ProfileID: participant.ProfileID}
	if call.SpaceMedia == nil {
		call.SpaceMedia = map[string]SpaceMediaParticipant{}
	}
	participant.Reconciled = participant.Issued
	call.SpaceMedia[participant.ProfileID] = participant
	s.calls[roomID] = call
	return call, nil
}

func (s *MemoryCallStore) BeginSpaceMediaRevocation(_ context.Context, roomID, profileID, identity, generation string) (Call, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, false, ErrNotFound
	}
	participant, ok := call.SpaceMedia[profileID]
	if !ok || participant.Identity != identity || participant.Generation != generation {
		return call, false, nil
	}
	participant.Revoking = true
	call.SpaceMedia[profileID] = participant
	s.calls[roomID] = call
	return call, true, nil
}

func (s *MemoryCallStore) CompleteSpaceMediaRevocation(_ context.Context, roomID, profileID, identity, generation string) (Call, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, false, ErrNotFound
	}
	participant, ok := call.SpaceMedia[profileID]
	if !ok || !participant.Revoking || participant.Identity != identity || participant.Generation != generation {
		return call, false, nil
	}
	delete(call.SpaceMedia, profileID)
	delete(call.States, profileID)
	call = removeScreenSharesForProfile(call, profileID)
	if len(call.States) == 0 {
		call.Status, call.EndedAt = callsv1.CallStatus_CALL_STATUS_ENDED, time.Now().UTC()
	}
	s.calls[roomID] = call
	return call, true, nil
}

func (s *MemoryCallStore) ReconcileSpaceMediaParticipant(_ context.Context, roomID, profileID, identity, generation string, grant SpaceMediaGrant) (Call, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, false, ErrNotFound
	}
	participant, ok := call.SpaceMedia[profileID]
	if !ok || participant.Identity != identity || participant.Generation != generation || participant.Revoking {
		return call, false, nil
	}
	floors := s.floors[call.SpaceID]
	if grant.AccessEpoch < floors.AccessEpoch || grant.PolicyEpoch < floors.PolicyEpoch {
		return Call{}, false, ErrSpaceMediaStaleGrant
	}
	participant.Reconciled = grant
	call.SpaceMedia[profileID] = participant
	s.calls[roomID] = call
	return call, true, nil
}

func (s *MemoryCallStore) StartScreenShare(_ context.Context, roomID, profileID, streamID string) (Call, ScreenShareEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ScreenShareEntry{}, ErrNotFound
	}
	if !call.IsParticipant(profileID) {
		return Call{}, ScreenShareEntry{}, ErrNotParticipant
	}
	if call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return Call{}, ScreenShareEntry{}, ErrInvalidState
	}
	call, entry, err := startScreenShareLocked(call, profileID, streamID)
	if err != nil {
		return Call{}, ScreenShareEntry{}, err
	}
	s.calls[roomID] = call
	return call, entry, nil
}

func (s *MemoryCallStore) StopScreenShare(_ context.Context, roomID, profileID, streamID string) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ErrNotFound
	}
	if !call.IsParticipant(profileID) {
		return Call{}, ErrNotParticipant
	}
	call, err := stopScreenShareLocked(call, profileID, streamID)
	if err != nil {
		return Call{}, err
	}
	s.calls[roomID] = call
	return call, nil
}

func (s *MemoryCallStore) StopScreenSharesForProfile(_ context.Context, roomID, profileID string) (Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[roomID]
	if !ok {
		return Call{}, ErrNotFound
	}
	call = removeScreenSharesForProfile(call, profileID)
	s.calls[roomID] = call
	return call, nil
}

func startScreenShareLocked(call Call, profileID, streamID string) (Call, ScreenShareEntry, error) {
	call.ScreenShares = removeProfileScreenShares(call.ScreenShares, profileID)
	if len(call.ScreenShares) >= MaxScreenSharesPerRoom {
		return Call{}, ScreenShareEntry{}, ErrScreenShareLimit
	}
	entry := ScreenShareEntry{ProfileID: profileID, StreamID: streamID}
	call.ScreenShares = append(call.ScreenShares, entry)
	if call.States == nil {
		call.States = defaultStates(call)
	}
	state := call.States[profileID]
	state.IsScreenSharing = true
	call.States[profileID] = state
	return call, entry, nil
}

func stopScreenShareLocked(call Call, profileID, streamID string) (Call, error) {
	if !hasScreenShare(call, profileID, streamID) {
		return Call{}, ErrNotScreenSharing
	}
	call.ScreenShares = removeProfileScreenShare(call.ScreenShares, profileID, streamID)
	if call.States == nil {
		call.States = defaultStates(call)
	}
	state := call.States[profileID]
	state.IsScreenSharing = hasProfileScreenShare(call, profileID)
	call.States[profileID] = state
	return call, nil
}

func removeScreenSharesForProfile(call Call, profileID string) Call {
	call.ScreenShares = removeProfileScreenShares(call.ScreenShares, profileID)
	if call.States == nil {
		return call
	}
	if state, ok := call.States[profileID]; ok {
		state.IsScreenSharing = false
		call.States[profileID] = state
	}
	return call
}

func removeProfileScreenShares(shares []ScreenShareEntry, profileID string) []ScreenShareEntry {
	if len(shares) == 0 {
		return shares
	}
	out := shares[:0]
	for _, share := range shares {
		if share.ProfileID != profileID {
			out = append(out, share)
		}
	}
	return out
}

func removeProfileScreenShare(shares []ScreenShareEntry, profileID, streamID string) []ScreenShareEntry {
	if len(shares) == 0 {
		return shares
	}
	out := shares[:0]
	for _, share := range shares {
		if share.ProfileID == profileID && (streamID == "" || share.StreamID == streamID) {
			continue
		}
		out = append(out, share)
	}
	return out
}

func hasScreenShare(call Call, profileID, streamID string) bool {
	for _, share := range call.ScreenShares {
		if share.ProfileID == profileID && (streamID == "" || share.StreamID == streamID) {
			return true
		}
	}
	return false
}

func hasProfileScreenShare(call Call, profileID string) bool {
	for _, share := range call.ScreenShares {
		if share.ProfileID == profileID {
			return true
		}
	}
	return false
}

func defaultStates(call Call) map[string]ParticipantState {
	if call.isOpenVoiceSession() {
		return map[string]ParticipantState{
			call.InitiatorProfileID: {
				ProfileID: call.InitiatorProfileID,
				IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO,
			},
		}
	}
	return map[string]ParticipantState{
		call.InitiatorProfileID: {ProfileID: call.InitiatorProfileID, IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO},
		call.CalleeProfileID:    {ProfileID: call.CalleeProfileID, IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO},
	}
}
