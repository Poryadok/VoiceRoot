package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
)

// This is an owner-request binding, never admission evidence or a new basis.
// The caller must first validate Q/A and the original versioned winner chain.
func joinApprovalAliasContextSHA256(op JoinOperationInput, request, attempt uuid.UUID, originalBasis []byte) ([32]byte, error) {
	if !validJoinOperation(op) || op.Method != JoinOperationApprove || request == uuid.Nil || attempt == uuid.Nil || len(originalBasis) != sha256.Size || bytes.Equal(originalBasis, make([]byte, sha256.Size)) {
		return [32]byte{}, ErrJoinAttemptUnavailable
	}
	frame := []byte("voice.space.join.approval-alias-context.v1\x00")
	frame = binary.BigEndian.AppendUint32(frame, 1)
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(op.Method)))
	frame = append(frame, op.Method...)
	for _, id := range []uuid.UUID{op.ActorAccountID, op.ActorProfileID, op.SpaceID, op.OperationID} {
		frame = append(frame, id[:]...)
	}
	frame = append(frame, op.RequestSHA256...)
	frame = append(frame, request[:]...)
	frame = append(frame, attempt[:]...)
	frame = append(frame, originalBasis...)
	return sha256.Sum256(frame), nil
}
