package spaceaudit

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var ErrInvalidEvent = errors.New("invalid audit event")
var ErrProducerAction = errors.New("audit producer/action mismatch")

const FingerprintVersion = 1

const replayDomain = "voice.space.audit.append.v1"

type ReplayEvent struct {
	AuditEventID, SpaceID, ActorProfileID, Action, TargetType, TargetID, DetailsJSON string
	OccurredAt                                                                       *timestamppb.Timestamp
}

// ValidateEvent must run after authenticating the exact service principal and
// rejecting unknown fields in the outer request. The actor is trusted body data,
// not an actor claim on a service principal.
func ValidateEvent(producer string, event ReplayEvent) error {
	spec, ok := LookupAction(event.Action)
	if !ok || spec.Producer != producer {
		return ErrProducerAction
	}
	return validateBody(event, spec)
}

func validateBody(event ReplayEvent, spec ActionSpec) error {
	ids := []string{event.AuditEventID, event.SpaceID, event.ActorProfileID, event.TargetID}
	parsed := make([]uuid.UUID, len(ids))
	for i, value := range ids {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return ErrInvalidEvent
		}
		parsed[i] = id
	}
	if event.TargetType != spec.TargetType || (spec.TargetsSpace && parsed[1] != parsed[3]) {
		return ErrInvalidEvent
	}
	if event.OccurredAt == nil || event.OccurredAt.CheckValid() != nil || len(event.OccurredAt.ProtoReflect().GetUnknown()) != 0 {
		return ErrInvalidEvent
	}
	return ValidateDetails(event.Action, event.DetailsJSON)
}

// ReplayFingerprint is version 1's durable body identity, NOT the protobuf
// authentication hash. Producer identity is authenticated and compared separately
// by the receipt store. Original accepted UUID spellings are never normalized.
func ReplayFingerprint(event ReplayEvent) ([32]byte, error) {
	var result [32]byte
	spec, ok := LookupAction(event.Action)
	if !ok {
		return result, ErrInvalidEvent
	}
	if err := validateBody(event, spec); err != nil {
		return result, err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(replayDomain))
	for _, value := range []string{event.AuditEventID, event.SpaceID, event.ActorProfileID, event.Action, event.TargetType, event.TargetID, event.DetailsJSON} {
		if uint64(len(value)) > math.MaxUint32 {
			return result, ErrInvalidEvent
		}
		// Length is bounded above before converting to the frozen uint32 frame.
		_ = binary.Write(hash, binary.BigEndian, uint32(len(value)))
		_, _ = hash.Write([]byte(value))
	}
	// binary.Write preserves signed int64 two's-complement without UnixNano's
	// overflow for valid year-0001/year-9999 protobuf instants.
	_ = binary.Write(hash, binary.BigEndian, event.OccurredAt.Seconds)
	_ = binary.Write(hash, binary.BigEndian, uint32(event.OccurredAt.Nanos))
	copy(result[:], hash.Sum(nil))
	return result, nil
}
