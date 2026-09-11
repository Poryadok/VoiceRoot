package roomlifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

var (
	ErrOperationConflict       = errors.New("voice lifecycle operation conflict")
	ErrSubjectTransitionActive = errors.New("voice lifecycle subject transition active")
	ErrStaleLease              = errors.New("voice lifecycle stale lease")
	ErrEffectNotReady          = errors.New("voice lifecycle effect not ready")
	ErrOperationNotCompletable = errors.New("voice lifecycle operation not completable")
	ErrMembershipConflict      = errors.New("voice lifecycle membership conflict")
	ErrInvariant               = errors.New("voice lifecycle invariant violation")
	ErrUnavailable             = errors.New("voice lifecycle storage unavailable")
)

type LifecycleMethod uint8

const (
	LifecycleMethodJoin LifecycleMethod = iota + 1
	LifecycleMethodLeave
	LifecycleMethodSelfMove
	LifecycleMethodModeratorMove
)

type LifecycleOutcome uint8

const (
	LifecycleOutcomeJoined LifecycleOutcome = iota + 1
	LifecycleOutcomeLeft
	LifecycleOutcomeMoved
	LifecycleOutcomeNoOp
)

type LifecycleOperationState uint8

const (
	LifecycleOperationDecided LifecycleOperationState = iota + 1
	LifecycleOperationCompleted
	LifecycleOperationQuarantined
)

type LifecycleEffectKind uint8

const (
	LifecycleEffectEnsureRoom LifecycleEffectKind = iota + 1
	LifecycleEffectEjectParticipant
)

type LifecycleEffectState uint8

const (
	LifecycleEffectStateReady LifecycleEffectState = iota + 1
	LifecycleEffectStateApplied
	LifecycleEffectStateQuarantined
)

type LifecycleOutboxState uint8

const (
	LifecycleOutboxStateReady LifecycleOutboxState = iota + 1
	LifecycleOutboxStateDelivered
	LifecycleOutboxStateQuarantined
)

type LifecycleDigest [32]byte

type LifecycleGrants struct {
	CanJoin               bool
	CanPublishAudio       bool
	CanPublishVideo       bool
	CanPublishScreenShare bool
	CanSubscribe          bool
	CanMuteOthers         bool
	CanDeafenOthers       bool
	CanMoveOthers         bool
	CanUsePTT             bool
	PrioritySpeaker       bool
}

type LifecycleAuthority struct {
	SpaceAccessEpoch                int64
	SubjectRolePolicyEpoch          int64
	ActorSourceRolePolicyEpoch      *int64
	ActorDestinationRolePolicyEpoch *int64
	AuthorizationDigest             LifecycleDigest
	SubjectGrants                   LifecycleGrants
	ActorCanMoveSource              *bool
	ActorCanMoveDestination         *bool
}

type LifecycleEffectPlan struct {
	EffectID      uuid.UUID
	Ordinal       int16
	Kind          LifecycleEffectKind
	SchemaVersion int16
}

type LifecycleDecision struct {
	ActorAccountID         uuid.UUID
	ActorProfileID         uuid.UUID
	OperationID            uuid.UUID
	SubjectProfileID       uuid.UUID
	SpaceID                uuid.UUID
	Method                 LifecycleMethod
	SourceVoiceRoomID      *uuid.UUID
	DestinationVoiceRoomID *uuid.UUID
	Authority              LifecycleAuthority
	RedisOwnerToken        LifecycleDigest
	Effects                []LifecycleEffectPlan
	AcceptedClockSkew      time.Duration
	DecidedAt              time.Time
}

type LifecycleReceipt struct {
	OperationID              uuid.UUID
	ActorProfileID           uuid.UUID
	SubjectProfileID         uuid.UUID
	SpaceID                  uuid.UUID
	Method                   LifecycleMethod
	Outcome                  LifecycleOutcome
	SourceVoiceRoomID        *uuid.UUID
	DestinationVoiceRoomID   *uuid.UUID
	RoomID                   *uuid.UUID
	SourceRosterVersion      *int64
	DestinationRosterVersion *int64
	MediaEpoch               *uuid.UUID
	SpaceAccessEpoch         *int64
	RolePolicyEpoch          *int64
	AuthorizationDigest      *LifecycleDigest
}

type LifecycleNoOpDecision struct {
	Decision    LifecycleDecision
	CompletedAt time.Time
	ReplayUntil time.Time
}

type LifecycleOutboxRecord struct {
	EventID          uuid.UUID
	Ordinal          int16
	Subject          string
	SchemaVersion    int16
	PayloadBytes     []byte
	SubjectProfileID uuid.UUID
	RoomID           uuid.UUID
	VoiceRoomID      uuid.UUID
	SpaceID          uuid.UUID
	RosterVersion    int64
	NextAttemptAt    time.Time
}

type LifecycleCompletion struct {
	ActorProfileID uuid.UUID
	OperationID    uuid.UUID
	WorkerID       uuid.UUID
	ExpectedFence  int64
	Receipt        LifecycleReceipt
	Outbox         []LifecycleOutboxRecord
	CompletedAt    time.Time
	ReplayUntil    time.Time
}

type LifecycleOperation struct {
	ActorAccountID         uuid.UUID
	ActorProfileID         uuid.UUID
	OperationID            uuid.UUID
	SubjectProfileID       uuid.UUID
	SpaceID                uuid.UUID
	Method                 LifecycleMethod
	Fingerprint            LifecycleDigest
	BindingBytes           []byte
	SourceVoiceRoomID      *uuid.UUID
	DestinationVoiceRoomID *uuid.UUID
	SourceRoomID           *uuid.UUID
	DestinationRoomID      *uuid.UUID
	SourceMediaEpoch       *uuid.UUID
	DestinationMediaEpoch  *uuid.UUID
	Authority              LifecycleAuthority
	RedisOwnerToken        LifecycleDigest
	State                  LifecycleOperationState
	LeaseOwner             *uuid.UUID
	LeaseUntil             *time.Time
	LeaseFence             int64
	Receipt                *LifecycleReceipt
	ReceiptBytes           []byte
	ReceiptHash            *LifecycleDigest
	CompletedAt            *time.Time
	ReplayUntil            *time.Time
	QuarantineClass        *string
	QuarantineDetail       *string
	QuarantinedAt          *time.Time
	DecidedAt              time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type LifecycleMembership struct {
	ProfileID            uuid.UUID
	RoomID               uuid.UUID
	MediaEpoch           uuid.UUID
	SpaceAccessEpoch     int64
	RolePolicyEpoch      int64
	AuthorizationDigest  LifecycleDigest
	Grants               LifecycleGrants
	LatestGrantExpiresAt *time.Time
	JoinedAt             time.Time
	UpdatedAt            time.Time
}

type LifecycleRoomSnapshot struct {
	RoomID          uuid.UUID
	SpaceID         uuid.UUID
	VoiceRoomID     uuid.UUID
	LiveKitRoomName string
	RosterVersion   int64
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ClosedAt        *time.Time
}

type LifecycleOperationClaim struct {
	Operation  LifecycleOperation
	WorkerID   uuid.UUID
	Fence      int64
	LeaseUntil time.Time
}

type LifecycleOperationLease struct {
	ActorProfileID uuid.UUID
	OperationID    uuid.UUID
	WorkerID       uuid.UUID
	ExpectedFence  int64
	LeaseUntil     time.Time
}

type LifecycleOperationQuarantine struct {
	ActorProfileID uuid.UUID
	OperationID    uuid.UUID
	WorkerID       uuid.UUID
	ExpectedFence  int64
	Class          string
	Detail         string
	At             time.Time
}

type LifecycleEffect struct {
	EffectID            uuid.UUID
	ActorProfileID      uuid.UUID
	OperationID         uuid.UUID
	Ordinal             int16
	Kind                LifecycleEffectKind
	SchemaVersion       int16
	VoiceRoomID         uuid.UUID
	LiveKitRoomName     string
	TargetProfileID     *uuid.UUID
	ParticipantIdentity *string
	MediaEpoch          *uuid.UUID
	RequestBytes        []byte
	RequestDigest       LifecycleDigest
	State               LifecycleEffectState
	AttemptCount        int64
	LastErrorClass      *string
	LastErrorAt         *time.Time
	NextAttemptAt       time.Time
	LeaseOwner          *uuid.UUID
	LeaseUntil          *time.Time
	LeaseFence          int64
	AppliedAt           *time.Time
	QuarantineClass     *string
	QuarantineDetail    *string
	QuarantinedAt       *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type LifecycleEffectClaim struct {
	Effect     LifecycleEffect
	WorkerID   uuid.UUID
	Fence      int64
	LeaseUntil time.Time
}

type LifecycleEffectLease struct {
	EffectID      uuid.UUID
	WorkerID      uuid.UUID
	ExpectedFence int64
	LeaseUntil    time.Time
}

type LifecycleEffectApplied struct {
	EffectID      uuid.UUID
	WorkerID      uuid.UUID
	ExpectedFence int64
	Observed      LifecycleEffect
	AppliedAt     time.Time
}

type LifecycleRetry struct {
	WorkerID      uuid.UUID
	ExpectedFence int64
	ErrorClass    string
	ErrorAt       time.Time
	NextAttemptAt time.Time
}

type LifecycleEffectRetry struct {
	EffectID uuid.UUID
	Retry    LifecycleRetry
}

type LifecycleEffectQuarantine struct {
	EffectID      uuid.UUID
	WorkerID      uuid.UUID
	ExpectedFence int64
	Class         string
	Detail        string
	At            time.Time
}

type LifecycleGrantEligibility struct {
	Membership LifecycleMembership
	Eligible   bool
}

type LifecycleMediaEpochDenial struct {
	MediaEpoch          uuid.UUID
	ProfileID           uuid.UUID
	RoomID              uuid.UUID
	LiveKitRoomName     string
	ParticipantIdentity string
	GrantExpiresAt      time.Time
	AcceptedClockSkew   time.Duration
	DenyUntil           time.Time
	Reason              string
	ActorProfileID      *uuid.UUID
	OperationID         *uuid.UUID
	AbsenceObservedAt   *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type LifecycleOutbox struct {
	EventID          uuid.UUID
	ActorProfileID   uuid.UUID
	OperationID      uuid.UUID
	Ordinal          int16
	Subject          string
	SchemaVersion    int16
	PayloadBytes     []byte
	PayloadHash      LifecycleDigest
	SubjectProfileID uuid.UUID
	RoomID           uuid.UUID
	VoiceRoomID      uuid.UUID
	SpaceID          uuid.UUID
	RosterVersion    int64
	State            LifecycleOutboxState
	AttemptCount     int64
	LastErrorClass   *string
	LastErrorAt      *time.Time
	NextAttemptAt    time.Time
	LeaseOwner       *uuid.UUID
	LeaseUntil       *time.Time
	LeaseFence       int64
	DeliveredAt      *time.Time
	QuarantineClass  *string
	QuarantineDetail *string
	QuarantinedAt    *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type LifecycleOutboxClaim struct {
	Outbox     LifecycleOutbox
	WorkerID   uuid.UUID
	Fence      int64
	LeaseUntil time.Time
}

type LifecycleOutboxLease struct {
	EventID       uuid.UUID
	WorkerID      uuid.UUID
	ExpectedFence int64
	LeaseUntil    time.Time
}

type LifecycleOutboxDelivered struct {
	EventID       uuid.UUID
	WorkerID      uuid.UUID
	ExpectedFence int64
	DeliveredAt   time.Time
}

type LifecycleOutboxRetry struct {
	EventID uuid.UUID
	Retry   LifecycleRetry
}

type LifecycleOutboxQuarantine struct {
	EventID       uuid.UUID
	WorkerID      uuid.UUID
	ExpectedFence int64
	Class         string
	Detail        string
	At            time.Time
}

type LifecycleAdvisoryLockKey struct {
	Namespace int32
	Key       int32
}

type LifecycleLogicalRoomLock struct {
	VoiceRoomID uuid.UUID
	Key         LifecycleAdvisoryLockKey
}

type LifecycleStore interface {
	CheckSchema(context.Context) error
	LoadOperation(context.Context, uuid.UUID, uuid.UUID) (LifecycleOperation, bool, error)
	DecideOperation(context.Context, LifecycleDecision) (LifecycleOperation, error)
	CompleteNoOp(context.Context, LifecycleNoOpDecision) (LifecycleOperation, error)
	ClaimNextOperation(context.Context, uuid.UUID, time.Time, time.Duration) (LifecycleOperationClaim, bool, error)
	RenewOperationLease(context.Context, LifecycleOperationLease) error
	QuarantineOperation(context.Context, LifecycleOperationQuarantine) error
	ListOperationEffects(context.Context, uuid.UUID, uuid.UUID) ([]LifecycleEffect, error)
	ClaimNextEffect(context.Context, uuid.UUID, time.Time, time.Duration) (LifecycleEffectClaim, bool, error)
	RenewEffectLease(context.Context, LifecycleEffectLease) error
	MarkEffectApplied(context.Context, LifecycleEffectApplied) error
	MarkEffectRetry(context.Context, LifecycleEffectRetry) error
	QuarantineEffect(context.Context, LifecycleEffectQuarantine) error
	CompleteOperation(context.Context, LifecycleCompletion) (LifecycleOperation, error)
	LoadMembership(context.Context, uuid.UUID) (LifecycleMembership, bool, error)
	LoadRoomSnapshot(context.Context, uuid.UUID) (LifecycleRoomSnapshot, bool, error)
	CheckGrantEligibility(context.Context, uuid.UUID, uuid.UUID) (LifecycleGrantEligibility, error)
	AdvanceLatestGrantExpiry(context.Context, uuid.UUID, uuid.UUID, time.Time) (LifecycleMembership, error)
	LookupMediaEpochDenial(context.Context, uuid.UUID) (LifecycleMediaEpochDenial, bool, error)
	MarkDenialAbsenceObserved(context.Context, uuid.UUID, time.Time) error
	DeleteExpiredObservedDenials(context.Context, time.Time, int) (int64, error)
	ClaimNextOutbox(context.Context, uuid.UUID, time.Time, time.Duration) (LifecycleOutboxClaim, bool, error)
	RenewOutboxLease(context.Context, LifecycleOutboxLease) error
	MarkOutboxDelivered(context.Context, LifecycleOutboxDelivered) error
	MarkOutboxRetry(context.Context, LifecycleOutboxRetry) error
	QuarantineOutbox(context.Context, LifecycleOutboxQuarantine) error
}

func EncodeLifecycleReceipt(receipt LifecycleReceipt) ([]byte, LifecycleDigest, error) {
	if err := validateReceipt(receipt); err != nil {
		return nil, LifecycleDigest{}, err
	}
	message := &callsv1.VoiceRoomLifecycleReceipt{
		OperationId: receipt.OperationID.String(), ActorProfileId: receipt.ActorProfileID.String(),
		SubjectProfileId: receipt.SubjectProfileID.String(), Space: &spacev1.SpaceRef{Id: receipt.SpaceID.String()},
		Method: callsv1.VoiceRoomLifecycleMethod(receipt.Method), Outcome: callsv1.VoiceRoomLifecycleOutcome(receipt.Outcome),
	}
	message.SourceVoiceRoomId = uuidStringPointer(receipt.SourceVoiceRoomID)
	message.DestinationVoiceRoomId = uuidStringPointer(receipt.DestinationVoiceRoomID)
	message.RoomId = uuidStringPointer(receipt.RoomID)
	message.MediaEpoch = uuidStringPointer(receipt.MediaEpoch)
	message.SourceRosterVersion = int64UintPointer(receipt.SourceRosterVersion)
	message.DestinationRosterVersion = int64UintPointer(receipt.DestinationRosterVersion)
	message.SpaceAccessEpoch = int64UintPointer(receipt.SpaceAccessEpoch)
	message.RolePolicyEpoch = int64UintPointer(receipt.RolePolicyEpoch)
	if receipt.AuthorizationDigest != nil {
		message.AuthorizationDigest = append([]byte(nil), receipt.AuthorizationDigest[:]...)
	}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, LifecycleDigest{}, ErrInvariant
	}
	return encoded, LifecycleDigest(sha256.Sum256(encoded)), nil
}

func validateReceipt(receipt LifecycleReceipt) error {
	if anyNilUUID(receipt.OperationID, receipt.ActorProfileID, receipt.SubjectProfileID, receipt.SpaceID) {
		return ErrInvariant
	}
	positive := func(value *int64) bool { return value != nil && *value > 0 }
	nonnegative := func(value *int64) bool { return value != nil && *value >= 0 }
	nonNilUUID := func(value *uuid.UUID) bool { return value != nil && *value != uuid.Nil }
	switch {
	case receipt.Method == LifecycleMethodJoin && (receipt.Outcome == LifecycleOutcomeJoined || receipt.Outcome == LifecycleOutcomeNoOp):
		if receipt.SourceVoiceRoomID != nil || receipt.SourceRosterVersion != nil ||
			!nonNilUUID(receipt.DestinationVoiceRoomID) || !nonNilUUID(receipt.RoomID) || !nonnegative(receipt.DestinationRosterVersion) ||
			!nonNilUUID(receipt.MediaEpoch) || !positive(receipt.SpaceAccessEpoch) || !positive(receipt.RolePolicyEpoch) || receipt.AuthorizationDigest == nil {
			return ErrInvariant
		}
	case receipt.Method == LifecycleMethodLeave && receipt.Outcome == LifecycleOutcomeLeft:
		if !nonNilUUID(receipt.SourceVoiceRoomID) || !nonNilUUID(receipt.RoomID) || !nonnegative(receipt.SourceRosterVersion) ||
			receipt.DestinationVoiceRoomID != nil || receipt.DestinationRosterVersion != nil || receipt.MediaEpoch != nil ||
			receipt.SpaceAccessEpoch != nil || receipt.RolePolicyEpoch != nil || receipt.AuthorizationDigest != nil {
			return ErrInvariant
		}
	case receipt.Method == LifecycleMethodLeave && receipt.Outcome == LifecycleOutcomeNoOp:
		if receipt.SourceVoiceRoomID != nil || receipt.DestinationVoiceRoomID != nil || receipt.RoomID != nil ||
			receipt.SourceRosterVersion != nil || receipt.DestinationRosterVersion != nil || receipt.MediaEpoch != nil ||
			receipt.SpaceAccessEpoch != nil || receipt.RolePolicyEpoch != nil || receipt.AuthorizationDigest != nil {
			return ErrInvariant
		}
	case (receipt.Method == LifecycleMethodSelfMove || receipt.Method == LifecycleMethodModeratorMove) && receipt.Outcome == LifecycleOutcomeMoved:
		if !nonNilUUID(receipt.SourceVoiceRoomID) || !nonNilUUID(receipt.DestinationVoiceRoomID) || *receipt.SourceVoiceRoomID == *receipt.DestinationVoiceRoomID ||
			!nonNilUUID(receipt.RoomID) || !nonnegative(receipt.SourceRosterVersion) || !nonnegative(receipt.DestinationRosterVersion) ||
			!nonNilUUID(receipt.MediaEpoch) || !positive(receipt.SpaceAccessEpoch) || !positive(receipt.RolePolicyEpoch) || receipt.AuthorizationDigest == nil {
			return ErrInvariant
		}
	default:
		return ErrInvariant
	}
	return nil
}

func uuidStringPointer(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

func int64UintPointer(value *int64) *uint64 {
	if value == nil || *value < 0 {
		return nil
	}
	converted := uint64(*value)
	return &converted
}

func LifecycleOperationAdvisoryKey(actorProfileID, operationID uuid.UUID) LifecycleAdvisoryLockKey {
	preimage := append([]byte("voice.lifecycle.lock.operation.v1\x00"), actorProfileID[:]...)
	preimage = append(preimage, operationID[:]...)
	return advisoryKey(0x564f5031, preimage)
}

func LifecycleSubjectAdvisoryKey(subjectProfileID uuid.UUID) LifecycleAdvisoryLockKey {
	preimage := append([]byte("voice.lifecycle.lock.subject.v1\x00"), subjectProfileID[:]...)
	return advisoryKey(0x56535531, preimage)
}

func LifecycleLogicalRoomAdvisoryKey(voiceRoomID uuid.UUID) LifecycleAdvisoryLockKey {
	preimage := append([]byte("voice.lifecycle.lock.logical-room.v1\x00"), voiceRoomID[:]...)
	return advisoryKey(0x564c5231, preimage)
}

func advisoryKey(namespace int32, preimage []byte) LifecycleAdvisoryLockKey {
	digest := sha256.Sum256(preimage)
	return LifecycleAdvisoryLockKey{Namespace: namespace, Key: int32(binary.BigEndian.Uint32(digest[:4]))}
}

func SortLifecycleLogicalRoomLocks(ids []uuid.UUID) []LifecycleLogicalRoomLock {
	locks := make([]LifecycleLogicalRoomLock, 0, len(ids))
	for _, id := range ids {
		if id != uuid.Nil {
			locks = append(locks, LifecycleLogicalRoomLock{VoiceRoomID: id, Key: LifecycleLogicalRoomAdvisoryKey(id)})
		}
	}
	sort.Slice(locks, func(i, j int) bool {
		if locks[i].Key.Key != locks[j].Key.Key {
			return locks[i].Key.Key < locks[j].Key.Key
		}
		return bytes.Compare(locks[i].VoiceRoomID[:], locks[j].VoiceRoomID[:]) < 0
	})
	result := locks[:0]
	for _, lock := range locks {
		if len(result) == 0 || result[len(result)-1].Key != lock.Key {
			result = append(result, lock)
		}
	}
	return result
}

func encodeLifecycleDecision(decision LifecycleDecision) ([]byte, LifecycleDigest, error) {
	if err := validateLifecycleDecision(decision); err != nil {
		return nil, LifecycleDigest{}, err
	}
	var buffer bytes.Buffer
	buffer.WriteString("voice.lifecycle.binding.v1\x00")
	for _, id := range []uuid.UUID{decision.ActorAccountID, decision.ActorProfileID, decision.OperationID, decision.SubjectProfileID, decision.SpaceID} {
		buffer.Write(id[:])
	}
	buffer.WriteByte(byte(decision.Method))
	writeOptionalUUID(&buffer, decision.SourceVoiceRoomID)
	writeOptionalUUID(&buffer, decision.DestinationVoiceRoomID)
	writeAuthority(&buffer, decision.Authority)
	buffer.Write(decision.RedisOwnerToken[:])
	_ = binary.Write(&buffer, binary.BigEndian, int64(decision.AcceptedClockSkew))
	_ = binary.Write(&buffer, binary.BigEndian, decision.DecidedAt.UTC().UnixNano())
	_ = binary.Write(&buffer, binary.BigEndian, uint16(len(decision.Effects)))
	for _, effect := range decision.Effects {
		buffer.Write(effect.EffectID[:])
		_ = binary.Write(&buffer, binary.BigEndian, effect.Ordinal)
		buffer.WriteByte(byte(effect.Kind))
		_ = binary.Write(&buffer, binary.BigEndian, effect.SchemaVersion)
	}
	encoded := buffer.Bytes()
	return append([]byte(nil), encoded...), LifecycleDigest(sha256.Sum256(encoded)), nil
}

func validateLifecycleDecision(decision LifecycleDecision) error {
	if anyNilUUID(decision.ActorAccountID, decision.ActorProfileID, decision.OperationID, decision.SubjectProfileID, decision.SpaceID) ||
		decision.DecidedAt.IsZero() || decision.AcceptedClockSkew < 0 {
		return ErrInvariant
	}
	validAuthority := decision.Authority.SpaceAccessEpoch > 0 && decision.Authority.SubjectRolePolicyEpoch > 0
	for index, effect := range decision.Effects {
		if effect.EffectID == uuid.Nil || effect.Ordinal != int16(index) || effect.SchemaVersion <= 0 {
			return ErrInvariant
		}
	}
	switch decision.Method {
	case LifecycleMethodJoin:
		if decision.SourceVoiceRoomID != nil || !validUUIDPointer(decision.DestinationVoiceRoomID) || !validAuthority ||
			len(decision.Effects) != 1 || decision.Effects[0].Kind != LifecycleEffectEnsureRoom {
			return ErrInvariant
		}
	case LifecycleMethodLeave:
		if !validUUIDPointer(decision.SourceVoiceRoomID) || decision.DestinationVoiceRoomID != nil ||
			len(decision.Effects) != 1 || decision.Effects[0].Kind != LifecycleEffectEjectParticipant {
			return ErrInvariant
		}
	case LifecycleMethodSelfMove, LifecycleMethodModeratorMove:
		if !validUUIDPointer(decision.SourceVoiceRoomID) || !validUUIDPointer(decision.DestinationVoiceRoomID) ||
			*decision.SourceVoiceRoomID == *decision.DestinationVoiceRoomID || !validAuthority || len(decision.Effects) != 2 ||
			decision.Effects[0].Kind != LifecycleEffectEjectParticipant || decision.Effects[1].Kind != LifecycleEffectEnsureRoom {
			return ErrInvariant
		}
		if decision.Method == LifecycleMethodModeratorMove &&
			(decision.Authority.ActorSourceRolePolicyEpoch == nil || decision.Authority.ActorDestinationRolePolicyEpoch == nil ||
				decision.Authority.ActorCanMoveSource == nil || decision.Authority.ActorCanMoveDestination == nil) {
			return ErrInvariant
		}
	default:
		return ErrInvariant
	}
	return nil
}

func encodeEffectRequest(kind LifecycleEffectKind, voiceRoomID uuid.UUID, liveKitRoomName string, targetProfileID *uuid.UUID, participantIdentity *string, mediaEpoch *uuid.UUID) ([]byte, LifecycleDigest) {
	var buffer bytes.Buffer
	buffer.WriteString("voice.lifecycle.effect.v1\x00")
	buffer.WriteByte(byte(kind))
	buffer.Write(voiceRoomID[:])
	writeString(&buffer, liveKitRoomName)
	writeOptionalUUID(&buffer, targetProfileID)
	if participantIdentity == nil {
		buffer.WriteByte(0)
	} else {
		buffer.WriteByte(1)
		writeString(&buffer, *participantIdentity)
	}
	writeOptionalUUID(&buffer, mediaEpoch)
	encoded := buffer.Bytes()
	return append([]byte(nil), encoded...), LifecycleDigest(sha256.Sum256(encoded))
}

func writeAuthority(buffer *bytes.Buffer, authority LifecycleAuthority) {
	_ = binary.Write(buffer, binary.BigEndian, authority.SpaceAccessEpoch)
	_ = binary.Write(buffer, binary.BigEndian, authority.SubjectRolePolicyEpoch)
	writeOptionalInt64(buffer, authority.ActorSourceRolePolicyEpoch)
	writeOptionalInt64(buffer, authority.ActorDestinationRolePolicyEpoch)
	buffer.Write(authority.AuthorizationDigest[:])
	for _, value := range []bool{
		authority.SubjectGrants.CanJoin, authority.SubjectGrants.CanPublishAudio, authority.SubjectGrants.CanPublishVideo,
		authority.SubjectGrants.CanPublishScreenShare, authority.SubjectGrants.CanSubscribe, authority.SubjectGrants.CanMuteOthers,
		authority.SubjectGrants.CanDeafenOthers, authority.SubjectGrants.CanMoveOthers, authority.SubjectGrants.CanUsePTT,
		authority.SubjectGrants.PrioritySpeaker,
	} {
		if value {
			buffer.WriteByte(1)
		} else {
			buffer.WriteByte(0)
		}
	}
	writeOptionalBool(buffer, authority.ActorCanMoveSource)
	writeOptionalBool(buffer, authority.ActorCanMoveDestination)
}

func writeOptionalUUID(buffer *bytes.Buffer, value *uuid.UUID) {
	if value == nil {
		buffer.WriteByte(0)
		return
	}
	buffer.WriteByte(1)
	buffer.Write(value[:])
}

func writeOptionalInt64(buffer *bytes.Buffer, value *int64) {
	if value == nil {
		buffer.WriteByte(0)
		return
	}
	buffer.WriteByte(1)
	_ = binary.Write(buffer, binary.BigEndian, *value)
}

func writeOptionalBool(buffer *bytes.Buffer, value *bool) {
	if value == nil {
		buffer.WriteByte(0)
		return
	}
	if *value {
		buffer.Write([]byte{1, 1})
	} else {
		buffer.Write([]byte{1, 0})
	}
}

func writeString(buffer *bytes.Buffer, value string) {
	_ = binary.Write(buffer, binary.BigEndian, uint32(len(value)))
	buffer.WriteString(value)
}

func validUUIDPointer(value *uuid.UUID) bool { return value != nil && *value != uuid.Nil }

func anyNilUUID(values ...uuid.UUID) bool {
	for _, value := range values {
		if value == uuid.Nil {
			return true
		}
	}
	return false
}

func nonblank(value string) bool { return strings.TrimSpace(value) != "" }
