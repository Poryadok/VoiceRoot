package spacelifecycle

import (
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
)

var (
	ErrInvalidRequest = errors.New("invalid Space lifecycle request")
	ErrConflict       = errors.New("Space lifecycle request conflicts with durable state")
	ErrUnavailable    = errors.New("Space lifecycle store unavailable")
	ErrSpaceFrozen    = errors.New("Space voice admission is fenced")
)

func validateFenceRequest(req *commonv1.SpaceLifecycleFenceRequest) error {
	if req == nil || req.GetProtocolVersion() != 1 || !canonicalUUID(req.GetSpaceId()) ||
		!canonicalUUID(req.GetDeletionOperationId()) || req.GetGeneration() == 0 || req.GetGeneration() > uint64(^uint64(0)>>1) {
		return ErrInvalidRequest
	}
	if req.ProtoReflect().GetUnknown() != nil && len(req.ProtoReflect().GetUnknown()) != 0 {
		return ErrInvalidRequest
	}
	switch req.GetDesiredState() {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE,
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
	default:
		return ErrInvalidRequest
	}
	manifest := req.GetManifest()
	if manifest == nil || !canonicalUUID(manifest.GetManifestId()) || len(manifest.GetManifestSha256()) != 32 {
		return ErrInvalidRequest
	}
	if len(manifest.ProtoReflect().GetUnknown()) != 0 {
		return ErrInvalidRequest
	}
	return nil
}

func validatePurgeRequest(req *commonv1.SpacePurgeRequest) error {
	if req == nil || req.GetProtocolVersion() != 1 || !canonicalUUID(req.GetSpaceId()) ||
		!canonicalUUID(req.GetDeletionOperationId()) || req.GetGeneration() == 0 || req.GetGeneration() > uint64(^uint64(0)>>1) ||
		req.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_VOICE || req.GetPurgeDecidedAt() == nil || req.GetPurgeDecidedAt().CheckValid() != nil {
		return ErrInvalidRequest
	}
	if len(req.ProtoReflect().GetUnknown()) != 0 {
		return ErrInvalidRequest
	}
	manifest := req.GetManifest()
	if manifest == nil || !canonicalUUID(manifest.GetManifestId()) || len(manifest.GetManifestSha256()) != 32 || len(manifest.ProtoReflect().GetUnknown()) != 0 {
		return ErrInvalidRequest
	}
	return nil
}

func canonicalUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func canonicalProto(message proto.Message) ([]byte, []byte, error) {
	if message == nil {
		return nil, nil, ErrInvalidRequest
	}
	// Receipts bind the actual participant RPC, independently of the transport
	// credential's plain deterministic protobuf request hash.
	switch req := message.(type) {
	case *commonv1.SpaceLifecycleFenceRequest:
		message = &callsv1.ApplySpaceLifecycleFenceRequest{Fence: req}
	case *commonv1.SpacePurgeRequest:
		message = &callsv1.PurgeSpaceRequest{Purge: req}
	default:
		return nil, nil, ErrInvalidRequest
	}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	if err != nil || len(encoded) == 0 {
		return nil, nil, ErrInvalidRequest
	}
	domain := string(message.ProtoReflect().Descriptor().FullName()) + "\x00"
	digest := sha256.New()
	_, _ = digest.Write([]byte(domain))
	_, _ = digest.Write(encoded)
	return encoded, digest.Sum(nil), nil
}
