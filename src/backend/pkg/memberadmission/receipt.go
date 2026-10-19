// Package memberadmission owns the versioned admission receipt fingerprints
// shared by the Role producer and Space publication consumer.
package memberadmission

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	"github.com/google/uuid"
)

const ProtocolVersion uint32 = 1

var ErrInvalidBinding = errors.New("invalid member admission binding")

type EnsureBinding struct {
	ProtocolVersion                          uint32
	SpaceID, ProfileID, AdmissionOperationID string
	MembershipGeneration                     uint64
	AdmissionBindingSHA256                   []byte
}

type RemoveBinding struct {
	ProtocolVersion                        uint32
	SpaceID, ProfileID, RemovalOperationID string
	RemovalGenerationFloor                 uint64
	RemovalBindingSHA256                   []byte
}

func ValidateEnsure(binding EnsureBinding) error {
	return validate(binding.ProtocolVersion, binding.SpaceID, binding.ProfileID, binding.AdmissionOperationID, binding.MembershipGeneration, binding.AdmissionBindingSHA256)
}
func ValidateRemove(binding RemoveBinding) error {
	return validate(binding.ProtocolVersion, binding.SpaceID, binding.ProfileID, binding.RemovalOperationID, binding.RemovalGenerationFloor, binding.RemovalBindingSHA256)
}

func validate(version uint32, space, profile, operation string, generation uint64, digest []byte) error {
	if version != ProtocolVersion || generation == 0 || generation > math.MaxInt64 || len(digest) != sha256.Size {
		return ErrInvalidBinding
	}
	for _, value := range []string{space, profile, operation} {
		if !canonicalID(value) {
			return ErrInvalidBinding
		}
	}
	return nil
}
func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

// EnsureReceiptFingerprint fixes every admission field and the role chosen by
// Role. It is a receipt identity, not a request signature or live permission.
func EnsureReceiptFingerprint(binding EnsureBinding, roleID string) ([32]byte, error) {
	if err := ValidateEnsure(binding); err != nil {
		return [32]byte{}, err
	}
	if !canonicalID(roleID) {
		return [32]byte{}, ErrInvalidBinding
	}
	data := appendBinding([]byte("voice.member-admission.ensure-receipt.v1\x00"), binding.ProtocolVersion, binding.SpaceID, binding.ProfileID, binding.AdmissionOperationID, binding.MembershipGeneration, binding.AdmissionBindingSHA256)
	id, _ := uuid.Parse(roleID)
	data = append(data, id[:]...)
	return sha256.Sum256(data), nil
}

// RemoveReceiptFingerprint identifies the original floor request. A historical
// receipt cannot prove that the requested incarnation remains a live member.
func RemoveReceiptFingerprint(binding RemoveBinding) ([32]byte, error) {
	if err := ValidateRemove(binding); err != nil {
		return [32]byte{}, err
	}
	data := appendBinding([]byte("voice.member-admission.remove-receipt.v1\x00"), binding.ProtocolVersion, binding.SpaceID, binding.ProfileID, binding.RemovalOperationID, binding.RemovalGenerationFloor, binding.RemovalBindingSHA256)
	return sha256.Sum256(data), nil
}

// Framing is versionBE32, SpaceUUID16, profileUUID16, generationBE64,
// operationUUID16, original binding SHA25632. All values have fixed width.
func appendBinding(data []byte, version uint32, space, profile, operation string, generation uint64, digest []byte) []byte {
	data = binary.BigEndian.AppendUint32(data, version)
	for _, value := range []string{space, profile} {
		id, _ := uuid.Parse(value)
		data = append(data, id[:]...)
	}
	data = binary.BigEndian.AppendUint64(data, generation)
	id, _ := uuid.Parse(operation)
	data = append(data, id[:]...)
	return append(data, digest...)
}
