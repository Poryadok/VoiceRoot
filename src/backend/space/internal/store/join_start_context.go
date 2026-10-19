package store

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
)

// JoinStartSelectionContext fixes the exact pre-allocation START selection.
// It is historical request data, not current account or admission authority.
type JoinStartSelectionContext struct {
	Operation             JoinOperationInput `json:"operation"`
	Kind                  uint8              `json:"kind"`
	InviteID              *uuid.UUID         `json:"invite_id"`
	PolicyVersion         int64              `json:"policy_version"`
	PolicySHA256          []byte             `json:"policy_sha256"`
	SelectedAttemptID     *uuid.UUID         `json:"selected_attempt_id"`
	SelectedBindingSHA256 []byte             `json:"selected_binding_sha256"`
}

func (c JoinStartSelectionContext) SHA256() ([32]byte, error) {
	o := c.Operation
	if !validJoinOperation(o) || o.Method != JoinOperationStart || !validJoinDecisionDigest(o.RequestSHA256) || c.PolicyVersion <= 0 || !validJoinDecisionDigest(c.PolicySHA256) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	if (c.Kind != 1 && c.Kind != 2) || (c.Kind == 1 && c.InviteID != nil) || (c.Kind == 2 && (c.InviteID == nil || *c.InviteID == uuid.Nil)) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	if c.SelectedAttemptID == nil {
		if c.SelectedBindingSHA256 != nil {
			return [32]byte{}, ErrJoinDecisionInvalid
		}
	} else if *c.SelectedAttemptID == uuid.Nil || !validJoinDecisionDigest(c.SelectedBindingSHA256) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	out := []byte("voice.space.join.start-selection-context.v1\x00")
	out = binary.BigEndian.AppendUint32(out, 1)
	out = binary.BigEndian.AppendUint32(out, uint32(len(o.Method)))
	out = append(out, o.Method...)
	out = append(out, o.RequestSHA256...)
	out = append(out, o.ActorAccountID[:]...)
	out = append(out, o.ActorProfileID[:]...)
	out = append(out, o.OperationID[:]...)
	out = append(out, o.SpaceID[:]...)
	out = append(out, c.Kind)
	if c.InviteID == nil {
		out = append(out, 0)
	} else {
		out = append(out, 1)
		out = append(out, c.InviteID[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(c.PolicyVersion))
	out = append(out, c.PolicySHA256...)
	if c.SelectedAttemptID == nil {
		out = append(out, 0)
	} else {
		out = append(out, 1)
		out = append(out, c.SelectedAttemptID[:]...)
		out = append(out, c.SelectedBindingSHA256...)
	}
	return sha256.Sum256(out), nil
}
