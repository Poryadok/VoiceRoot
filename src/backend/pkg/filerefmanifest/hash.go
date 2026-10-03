// Package filerefmanifest implements the documented File producer tuple digest.
// File retains authority over its private aggregate root and reference ledger.
package filerefmanifest

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
)

// Hash consumes deterministic protobuf encodings in File's tuple sort order.
func Hash(deletion, space uuid.UUID, generation uint64, producer uint32, ordered [][]byte) ([]byte, error) {
	if deletion == uuid.Nil || space == uuid.Nil || generation == 0 || producer < 1 || producer > 3 {
		return nil, errors.New("invalid File producer binding")
	}
	h := sha256.New()
	h.Write([]byte("voice.file.v1.SpaceDeletionReferenceProducer\x00"))
	h.Write(deletion[:])
	h.Write(space[:])
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], generation)
	h.Write(integer[:])
	binary.BigEndian.PutUint32(integer[:4], producer)
	h.Write(integer[:4])
	for _, wire := range ordered {
		if len(wire) == 0 {
			return nil, errors.New("empty File reference tuple")
		}
		binary.BigEndian.PutUint32(integer[:4], uint32(len(wire)))
		h.Write(integer[:4])
		h.Write(wire)
	}
	return h.Sum(nil), nil
}
