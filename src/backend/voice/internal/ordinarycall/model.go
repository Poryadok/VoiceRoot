// Package ordinarycall owns the durable ordinary DM/group lifecycle. It does not
// authorize Space, MatchSquad or managed Game resources.
package ordinarycall

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidIntent    = errors.New("invalid ordinary call intent")
	ErrConflict         = errors.New("ordinary call admission conflict")
	ErrEffectPending    = errors.New("ordinary call SDK prerequisite pending")
	ErrAdmissionAborted = errors.New("ordinary call admission aborted")
	ErrCleanupPending   = errors.New("ordinary call cleanup pending")
	ErrStaleGeneration  = errors.New("ordinary call stale membership generation")
	ErrExpired          = errors.New("ordinary call ringing expired")
	ErrNotRinging       = errors.New("ordinary call is not ringing")
	ErrNotMember        = errors.New("not an ordinary call member")
	ErrCapacity         = errors.New("ordinary call capacity reached")
)

type Stage string

const (
	StagePreparing Stage = "PREPARING"
	StageRinging   Stage = "RINGING"
	StageActive    Stage = "ACTIVE"
	StageAborting  Stage = "ABORTING"
	StageClosing   Stage = "CLOSING"
	StageClosed    Stage = "CLOSED"
)

type MemberState string

const (
	MemberPending  MemberState = "PENDING"
	MemberReserved MemberState = "RESERVED"
	MemberJoined   MemberState = "JOINED"
	MemberLeaving  MemberState = "LEAVING"
	MemberLeft     MemberState = "LEFT"
	MemberAborted  MemberState = "ABORTED"
)

type AdmissionState string

const (
	AdmissionPending   AdmissionState = "PENDING"
	AdmissionCommitted AdmissionState = "COMMITTED"
	AdmissionAborted   AdmissionState = "ABORTED"
)

type AdmissionKind string

const (
	AdmissionStart  AdmissionKind = "START"
	AdmissionJoin   AdmissionKind = "JOIN"
	AdmissionAccept AdmissionKind = "ACCEPT"
)

type EventKind string

const (
	EventIncoming          EventKind = "incoming"
	EventAccepted          EventKind = "accepted"
	EventDeclined          EventKind = "declined"
	EventMissed            EventKind = "missed"
	EventEnded             EventKind = "ended"
	EventCallStarted       EventKind = "call_started"
	EventParticipantJoined EventKind = "participant_joined"
	EventParticipantLeft   EventKind = "participant_left"
)

type EffectKind string

const (
	EffectEnsure       EffectKind = "ENSURE"
	EffectClose        EffectKind = "CLOSE"
	EffectRemoveMember EffectKind = "REMOVE_MEMBER"
	EffectReleaseFence EffectKind = "RELEASE_FENCE"
)

type Identity struct {
	AccountID uuid.UUID `json:"account_id"`
	ProfileID uuid.UUID `json:"profile_id"`
	// Zero is unknown legacy provenance, never an Auth-floor substitute. A3
	// consumers must fail closed on it; only a verified admission supplies >0.
	SessionEpoch int64 `json:"session_epoch"`
}

type CreateIntent struct {
	OperationID uuid.UUID `json:"operation_id"`
	RoomID      uuid.UUID `json:"room_id"`
	ChatID      uuid.UUID `json:"chat_id"`
	Group       bool      `json:"group"`
	Initiator   Identity  `json:"initiator"`
	Callee      *Identity `json:"callee,omitempty"`
	MediaKind   int32     `json:"media_kind"`
}

type Member struct {
	Identity
	Generation       uuid.UUID   `json:"generation"`
	FenceOperationID uuid.UUID   `json:"fence_operation_id"`
	MediaIdentity    string      `json:"media_identity"`
	State            MemberState `json:"state"`
	JoinedAt         time.Time   `json:"joined_at"`
	LeftAt           time.Time   `json:"left_at"`
}

type Admission struct {
	ActorEpoch  int64                `json:"actor_epoch"`
	ID          uuid.UUID            `json:"id"`
	Kind        AdmissionKind        `json:"kind"`
	State       AdmissionState       `json:"state"`
	ActorID     uuid.UUID            `json:"actor_id"`
	Generations map[string]uuid.UUID `json:"generations"`
	CreatedAt   time.Time            `json:"created_at"`
}

// Event is a committed semantic intent, not proof of broker delivery. The
// outbox adapter freezes its corresponding compatibility payload/recipients.
type Event struct {
	RosterVersion uint64      `json:"roster_version,omitempty"`
	ID            uuid.UUID   `json:"id"`
	Kind          EventKind   `json:"kind"`
	ProfileID     uuid.UUID   `json:"profile_id"`
	ActorID       uuid.UUID   `json:"actor_id"`
	Reason        string      `json:"reason,omitempty"`
	OccurredAt    time.Time   `json:"occurred_at"`
	AudienceIDs   []uuid.UUID `json:"audience_ids"`
}

type Effect struct {
	ID          uuid.UUID  `json:"id"`
	Kind        EffectKind `json:"kind"`
	OperationID uuid.UUID  `json:"operation_id"`
	ProfileID   uuid.UUID  `json:"profile_id"`
	Generation  uuid.UUID  `json:"generation"`
	Participant string     `json:"participant,omitempty"`
	Applied     bool       `json:"applied"`
	Revision    uint64     `json:"revision"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Room is a transaction aggregate. Mutators are pure with respect to external
// systems: the store must persist the aggregate and exact fence/outbox effects
// atomically, then execute IO after commit. They require a store-held room lock.
type Room struct {
	// TransactionTime is storage-supplied database time, not client data.
	TransactionTime time.Time            `json:"-"`
	Intent          CreateIntent         `json:"intent"`
	Stage           Stage                `json:"stage"`
	Version         uint64               `json:"version"`
	RosterVersion   uint64               `json:"roster_version"`
	LiveKitRoomName string               `json:"livekit_room_name"`
	CreatedAt       time.Time            `json:"created_at"`
	ReadyAt         time.Time            `json:"ready_at"`
	ExpiresAt       time.Time            `json:"expires_at"`
	EndedAt         time.Time            `json:"ended_at"`
	TerminalReason  string               `json:"terminal_reason,omitempty"`
	TerminalActor   uuid.UUID            `json:"terminal_actor"`
	Members         map[string]Member    `json:"members"`
	RetiredMembers  []Member             `json:"retired_members,omitempty"`
	Admissions      map[string]Admission `json:"admissions"`
	Effects         []Effect             `json:"effects"`
	Events          []Event              `json:"events"`
}

func validIdentity(v Identity) bool {
	return v.AccountID != uuid.Nil && v.ProfileID != uuid.Nil && v.SessionEpoch >= 0
}

func NewPendingRoom(intent CreateIntent, now time.Time) (*Room, error) {
	if intent.OperationID == uuid.Nil || intent.RoomID == uuid.Nil || intent.ChatID == uuid.Nil ||
		!validIdentity(intent.Initiator) || now.IsZero() || (intent.MediaKind != 1 && intent.MediaKind != 2) ||
		(intent.Group && intent.Callee != nil) || (!intent.Group && (intent.Callee == nil ||
		!validIdentity(*intent.Callee) || intent.Callee.SessionEpoch != 0 || intent.Callee.ProfileID == intent.Initiator.ProfileID ||
		intent.Callee.AccountID == intent.Initiator.AccountID)) {
		return nil, ErrInvalidIntent
	}
	r := &Room{
		Intent: intent, Stage: StagePreparing, Version: 1, CreatedAt: now,
		Members: map[string]Member{}, Admissions: map[string]Admission{},
	}
	// Do not retain a caller-owned identity pointer in the durable binding.
	if intent.Callee != nil {
		callee := *intent.Callee
		r.Intent.Callee = &callee
	}
	prefix := "voice-dm-"
	if intent.Group {
		prefix = "voice-group-"
	}
	r.LiveKitRoomName = prefix + intent.RoomID.String()
	a := Admission{ID: intent.OperationID, Kind: AdmissionStart, State: AdmissionPending,
		ActorID: intent.Initiator.ProfileID, ActorEpoch: intent.Initiator.SessionEpoch, Generations: map[string]uuid.UUID{}, CreatedAt: now}
	r.addPending(&a, intent.Initiator)
	if intent.Callee != nil {
		r.addPending(&a, *intent.Callee)
	}
	r.Admissions[a.ID.String()] = a
	r.Effects = append(r.Effects, Effect{ID: uuid.New(), Kind: EffectEnsure,
		OperationID: a.ID, CreatedAt: now})
	return r, nil
}

func (r *Room) addPending(a *Admission, id Identity) {
	generation := uuid.New()
	r.Members[id.ProfileID.String()] = Member{Identity: id, Generation: generation,
		FenceOperationID: a.ID, State: MemberPending,
		MediaIdentity: "ov:" + id.ProfileID.String() + ":" + generation.String()}
	a.Generations[id.ProfileID.String()] = generation
}

func (r *Room) BeginJoin(operation uuid.UUID, id Identity, now time.Time) error {
	if operation == uuid.Nil || !validIdentity(id) || now.IsZero() {
		return ErrInvalidIntent
	}
	if prior, ok := r.Admissions[operation.String()]; ok {
		if prior.Kind != AdmissionJoin || prior.ActorID != id.ProfileID {
			return ErrConflict
		}
		if prior.State == AdmissionAborted {
			return ErrAdmissionAborted
		}
		if prior.ActorEpoch != id.SessionEpoch {
			return ErrConflict
		}
		member, exists := r.Members[id.ProfileID.String()]
		if !exists || member.AccountID != id.AccountID ||
			member.Generation != prior.Generations[id.ProfileID.String()] {
			return ErrConflict
		}
		return nil
	}
	if !r.Intent.Group || r.Stage != StageActive {
		return ErrAdmissionAborted
	}
	a := Admission{ID: operation, Kind: AdmissionJoin, State: AdmissionPending,
		ActorID: id.ProfileID, ActorEpoch: id.SessionEpoch, Generations: map[string]uuid.UUID{}, CreatedAt: now}
	prior, exists := r.Members[id.ProfileID.String()]
	if exists {
		if prior.AccountID != id.AccountID {
			return ErrConflict
		}
		switch prior.State {
		case MemberJoined:
			// Existing membership is a no-op, never a new incarnation/fact.
			a.State = AdmissionCommitted
			a.Generations[id.ProfileID.String()] = prior.Generation
			r.Admissions[operation.String()] = a
			r.Version++
			return nil
		case MemberLeaving:
			return ErrCleanupPending
		case MemberPending, MemberReserved:
			return ErrConflict
		}
	}
	count := 0
	for _, member := range r.Members {
		if member.State == MemberJoined || member.State == MemberPending || member.State == MemberLeaving {
			count++
		}
		if member.AccountID == id.AccountID && member.ProfileID != id.ProfileID &&
			member.State != MemberLeft && member.State != MemberAborted {
			return ErrConflict
		}
	}
	if count >= 32 {
		return ErrCapacity
	}
	if exists {
		r.RetiredMembers = append(r.RetiredMembers, prior)
	}
	r.addPending(&a, id)
	r.Admissions[operation.String()] = a
	r.Version++
	return nil
}

func (r *Room) BeginAccept(operation, actor uuid.UUID, now time.Time) error {
	if r.Intent.Callee == nil {
		return ErrNotMember
	}
	return r.BeginVerifiedAccept(operation, Identity{AccountID: r.Intent.Callee.AccountID, ProfileID: actor}, now)
}

func (r *Room) BeginVerifiedAccept(operation uuid.UUID, subject Identity, now time.Time) error {
	actor := subject.ProfileID
	if operation == uuid.Nil || now.IsZero() {
		return ErrInvalidIntent
	}
	if !validIdentity(subject) || r.Intent.Group || r.Intent.Callee == nil || actor != r.Intent.Callee.ProfileID || subject.AccountID != r.Intent.Callee.AccountID {
		return ErrNotMember
	}
	if prior, ok := r.Admissions[operation.String()]; ok {
		if prior.Kind != AdmissionAccept || prior.ActorID != actor || prior.ActorEpoch != subject.SessionEpoch {
			return ErrConflict
		}
		if prior.State == AdmissionAborted {
			return ErrAdmissionAborted
		}
		return nil
	}
	if r.Stage != StageRinging {
		return ErrNotRinging
	}
	if !now.Before(r.ExpiresAt) {
		return ErrExpired
	}
	for _, prior := range r.Admissions {
		if prior.Kind == AdmissionAccept && prior.State == AdmissionPending {
			return ErrConflict
		}
	}
	a := Admission{ID: operation, Kind: AdmissionAccept, State: AdmissionPending,
		ActorID: actor, ActorEpoch: subject.SessionEpoch, Generations: map[string]uuid.UUID{}, CreatedAt: now}
	for key, member := range r.Members {
		if member.State != MemberReserved {
			return ErrConflict
		}
		a.Generations[key] = member.Generation
	}
	member := r.Members[actor.String()]
	member.SessionEpoch = subject.SessionEpoch
	r.Members[actor.String()] = member
	r.Admissions[operation.String()] = a
	r.Version++
	return nil
}

func (r *Room) RecordEnsureResult(operation uuid.UUID, succeeded bool, now time.Time) error {
	if operation != r.Intent.OperationID {
		return ErrConflict
	}
	if !succeeded {
		return r.AbortAdmission(operation, now)
	}
	for index := range r.Effects {
		if r.Effects[index].Kind == EffectEnsure && r.Effects[index].OperationID == operation {
			r.Effects[index].Applied = true
		}
	}
	if r.Stage == StageAborting || r.Stage == StageClosing || r.Stage == StageClosed {
		// The SDK has no local generation fence: even a late success after a
		// confirmed delete must retain/requeue compensation for this exact name.
		r.requireClose(now)
	}
	r.Version++
	return nil
}

func (r *Room) CommitAdmission(operation uuid.UUID, now time.Time) error {
	a, ok := r.Admissions[operation.String()]
	if !ok || a.State == AdmissionAborted {
		return ErrAdmissionAborted
	}
	if a.State == AdmissionCommitted {
		return nil
	}
	if (a.Kind == AdmissionStart && r.Stage != StagePreparing) ||
		(a.Kind == AdmissionJoin && r.Stage != StageActive) ||
		(a.Kind == AdmissionAccept && r.Stage != StageRinging) {
		return ErrAdmissionAborted
	}
	if a.Kind == AdmissionStart {
		ensured := false
		for _, effect := range r.Effects {
			ensured = ensured || effect.Kind == EffectEnsure && effect.Applied
		}
		if !ensured {
			return ErrEffectPending
		}
	}
	if a.Kind == AdmissionAccept && !now.Before(r.ExpiresAt) {
		return ErrExpired
	}
	for key, generation := range a.Generations {
		member, exists := r.Members[key]
		if !exists || member.Generation != generation ||
			(member.State != MemberPending && member.State != MemberReserved) {
			return ErrStaleGeneration
		}
	}
	a.State = AdmissionCommitted
	r.Admissions[operation.String()] = a
	if a.Kind == AdmissionStart {
		r.ReadyAt = now
		if r.Intent.Group {
			r.Stage = StageActive
		} else {
			r.Stage = StageRinging
			r.ExpiresAt = now.Add(30 * time.Second)
		}
	}
	if a.Kind == AdmissionAccept {
		r.Stage = StageActive
	}
	keys := sortedGenerationKeys(a.Generations)
	for _, key := range keys {
		member := r.Members[key]
		if r.Stage == StageRinging {
			member.State = MemberReserved
		} else {
			member.State = MemberJoined
			member.JoinedAt = now
		}
		r.Members[key] = member
	}
	if a.Kind == AdmissionStart && !r.Intent.Group {
		r.emit(EventIncoming, r.Intent.Callee.ProfileID, a.ActorID, "", now)
	} else {
		if a.Kind == AdmissionStart || a.Kind == AdmissionAccept {
			r.emit(EventAccepted, a.ActorID, a.ActorID, "", now)
		}
		if a.Kind == AdmissionStart {
			r.emit(EventCallStarted, a.ActorID, a.ActorID, "", now)
		}
		for _, key := range keys {
			r.emit(EventParticipantJoined, r.Members[key].ProfileID, a.ActorID, "", now)
		}
	}
	r.Version++
	return nil
}

func (r *Room) AbortAdmission(operation uuid.UUID, now time.Time) error {
	a, ok := r.Admissions[operation.String()]
	if !ok {
		return ErrConflict
	}
	if a.State == AdmissionCommitted || a.State == AdmissionAborted {
		// A lost response/replayed failed check cannot compensate a commit.
		return nil
	}
	a.State = AdmissionAborted
	r.Admissions[operation.String()] = a
	if a.Kind == AdmissionStart {
		r.Stage = StageAborting
		r.EndedAt = now
		r.requireClose(now)
	}
	if a.Kind == AdmissionJoin {
		for key, generation := range a.Generations {
			member, exists := r.Members[key]
			if exists && member.Generation == generation && member.State == MemberPending {
				member.State = MemberAborted
				r.Members[key] = member
				r.Effects = append(r.Effects, Effect{ID: uuid.New(), Kind: EffectReleaseFence,
					OperationID: member.FenceOperationID, ProfileID: member.ProfileID,
					Generation: member.Generation, CreatedAt: now})
			}
		}
	}
	r.Version++
	return nil
}

func (r *Room) CanIssueToken(profile uuid.UUID) bool {
	member, ok := r.Members[profile.String()]
	return r.Stage == StageActive && ok && member.State == MemberJoined
}

func (r *Room) CommittedProfileIDs() []uuid.UUID {
	result := []uuid.UUID{}
	for _, member := range r.Members {
		if member.State == MemberJoined {
			result = append(result, member.ProfileID)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func (r *Room) DecideEnd(actor uuid.UUID, reason string, now time.Time) error {
	if !r.authorizedTerminalActor(actor) {
		return ErrNotMember
	}
	return r.terminate(actor, reason, now)
}

func (r *Room) DecideDecline(actor uuid.UUID, now time.Time) error {
	if r.Intent.Group || r.Intent.Callee == nil || actor != r.Intent.Callee.ProfileID {
		return ErrNotMember
	}
	if r.TerminalReason == "declined" {
		return nil
	}
	if r.Stage != StageRinging {
		return ErrNotRinging
	}
	r.emit(EventDeclined, actor, actor, "declined", now)
	return r.terminate(actor, "declined", now)
}

func (r *Room) DecideTimeout(now time.Time) error {
	if r.TerminalReason == "missed" {
		return nil
	}
	if r.Stage != StageRinging {
		return ErrNotRinging
	}
	if now.Before(r.ExpiresAt) {
		return ErrConflict
	}
	r.emit(EventMissed, r.Intent.Callee.ProfileID, uuid.Nil, "missed", now)
	return r.terminate(uuid.Nil, "missed", now)
}

func (r *Room) authorizedTerminalActor(actor uuid.UUID) bool {
	if !r.Intent.Group {
		return actor == r.Intent.Initiator.ProfileID || r.Intent.Callee != nil && actor == r.Intent.Callee.ProfileID
	}
	if r.Stage == StagePreparing {
		return actor == r.Intent.Initiator.ProfileID
	}
	if r.Stage == StageActive {
		member, exists := r.Members[actor.String()]
		return exists && member.State == MemberJoined
	}
	if actor == r.TerminalActor && actor != uuid.Nil {
		return true
	}
	for _, event := range r.Events {
		if event.Kind != EventEnded {
			continue
		}
		for _, profile := range event.AudienceIDs {
			if profile == actor {
				return true
			}
		}
	}
	return false
}

func (r *Room) terminate(actor uuid.UUID, reason string, now time.Time) error {
	if r.Stage == StageClosing || r.Stage == StageAborting || r.Stage == StageClosed {
		return nil
	}
	wasPreparing := r.Stage == StagePreparing
	audience := r.audience()
	for _, profile := range r.CommittedProfileIDs() {
		member := r.Members[profile.String()]
		member.State = MemberLeaving
		member.LeftAt = now
		r.Members[profile.String()] = member
		r.emit(EventParticipantLeft, profile, actor, reason, now)
	}
	for key, admission := range r.Admissions {
		if admission.State == AdmissionPending {
			admission.State = AdmissionAborted
			r.Admissions[key] = admission
		}
	}
	r.TerminalActor, r.TerminalReason, r.EndedAt = actor, reason, now
	r.Stage = StageClosing
	if wasPreparing {
		r.Stage = StageAborting
	} else {
		r.emit(EventEnded, actor, actor, reason, now)
		r.Events[len(r.Events)-1].AudienceIDs = audience
	}
	r.requireClose(now)
	r.Version++
	return nil
}

func (r *Room) DecideLeave(profile uuid.UUID, now time.Time) error {
	if !r.Intent.Group {
		return r.DecideEnd(profile, "hangup", now)
	}
	member, exists := r.Members[profile.String()]
	if !exists {
		return ErrNotMember
	}
	if member.State == MemberLeaving || member.State == MemberLeft {
		return nil
	}
	if member.State != MemberJoined || r.Stage != StageActive {
		return ErrNotMember
	}
	audience := r.audience()
	member.State, member.LeftAt = MemberLeaving, now
	r.Members[profile.String()] = member
	r.emit(EventParticipantLeft, profile, profile, "leave", now)
	if len(r.CommittedProfileIDs()) == 0 {
		if err := r.terminate(profile, "hangup", now); err != nil {
			return err
		}
		r.Events[len(r.Events)-1].AudienceIDs = audience
		return nil
	}
	r.Effects = append(r.Effects, Effect{ID: uuid.New(), Kind: EffectRemoveMember,
		OperationID: member.FenceOperationID, ProfileID: profile,
		Generation: member.Generation, Participant: member.MediaIdentity, CreatedAt: now})
	r.Version++
	return nil
}

func (r *Room) CompleteMemberCleanup(profile, generation uuid.UUID, now time.Time) error {
	member, exists := r.Members[profile.String()]
	if !exists || member.Generation != generation {
		return ErrStaleGeneration
	}
	if member.State == MemberLeft || member.State == MemberAborted {
		return nil
	}
	if member.State != MemberLeaving || r.Stage != StageActive {
		return ErrCleanupPending
	}
	member.State, member.LeftAt = MemberLeft, now
	r.Members[profile.String()] = member
	for index := range r.Effects {
		if r.Effects[index].Kind == EffectRemoveMember && r.Effects[index].Generation == generation {
			r.Effects[index].Applied = true
		}
	}
	r.Version++
	return nil
}

func (r *Room) HasPendingClose() bool {
	for _, effect := range r.Effects {
		if effect.Kind == EffectClose && !effect.Applied {
			return true
		}
	}
	return false
}

func (r *Room) PendingClose() (Effect, bool) {
	for _, effect := range r.Effects {
		if effect.Kind == EffectClose && !effect.Applied {
			return effect, true
		}
	}
	return Effect{}, false
}

// CompleteRoomCleanup records one exact observed SDK delete. A later observed
// create increments the close revision, so an older delete acknowledgement
// cannot release the fences as though that newer create had also been removed.
func (r *Room) CompleteRoomCleanup(effectID uuid.UUID, revision uint64, now time.Time) error {
	if r.Stage != StageClosing && r.Stage != StageAborting && r.Stage != StageClosed {
		return ErrCleanupPending
	}
	index := -1
	for i, effect := range r.Effects {
		if effect.Kind == EffectClose && effect.ID == effectID {
			index = i
			break
		}
	}
	if index < 0 || r.Effects[index].Revision != revision {
		return ErrStaleGeneration
	}
	if r.Effects[index].Applied {
		return nil
	}
	r.Effects[index].Applied = true
	for key, member := range r.Members {
		member.State = MemberLeft
		if member.LeftAt.IsZero() {
			member.LeftAt = now
		}
		r.Members[key] = member
	}
	r.Stage = StageClosed
	r.Version++
	return nil
}

func (r *Room) requireClose(now time.Time) {
	for index := range r.Effects {
		if r.Effects[index].Kind == EffectClose {
			r.Effects[index].Applied = false
			r.Effects[index].Revision++
			return
		}
	}
	r.Effects = append(r.Effects, Effect{ID: uuid.New(), Kind: EffectClose,
		OperationID: r.Intent.OperationID, CreatedAt: now, Revision: 1})
}

func (r *Room) emit(kind EventKind, profile, actor uuid.UUID, reason string, now time.Time) {
	var rosterVersion uint64
	if kind == EventParticipantJoined || kind == EventParticipantLeft {
		r.RosterVersion++
		rosterVersion = r.RosterVersion
	}
	r.Events = append(r.Events, Event{ID: uuid.New(), Kind: kind, ProfileID: profile,
		ActorID: actor, Reason: reason, OccurredAt: now, AudienceIDs: r.audience(), RosterVersion: rosterVersion})
}

func (r *Room) audience() []uuid.UUID {
	if r.Intent.Callee != nil {
		return []uuid.UUID{r.Intent.Initiator.ProfileID, r.Intent.Callee.ProfileID}
	}
	return r.CommittedProfileIDs()
}

func sortedGenerationKeys(values map[string]uuid.UUID) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
