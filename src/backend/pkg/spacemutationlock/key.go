// Package spacemutationlock defines the PostgreSQL advisory-lock key shared by
// services that serialize mutations of one Space.
package spacemutationlock

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
)

const namespace = "voice.space.mutation.v1\x00"

// Key returns the stable signed 64-bit key for a Space UUID. Keep this
// derivation identical in every service that participates in Space mutation
// serialization.
func Key(spaceID uuid.UUID) int64 {
	input := make([]byte, len(namespace)+len(spaceID))
	copy(input, namespace)
	copy(input[len(namespace):], spaceID[:])
	sum := sha256.Sum256(input)
	return int64(binary.BigEndian.Uint64(sum[:8]))
}
