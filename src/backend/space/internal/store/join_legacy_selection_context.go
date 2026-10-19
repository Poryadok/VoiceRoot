package store

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
)

// JoinLegacySelectionContext fixes pre-allocation request data. It is neither
// a legacy decision basis nor a current owner/admission capability.
type JoinLegacySelectionContext struct {
	Operation     JoinOperationInput `json:"operation"`
	Origin        uint8              `json:"origin"`
	InviteID      *uuid.UUID         `json:"invite_id"`
	PolicyVersion int64              `json:"policy_version"`
	PolicySHA256  []byte             `json:"policy_sha256"`
}

func (c JoinLegacySelectionContext) SHA256() ([32]byte, error) {
	o := c.Operation
	if !validLegacyJoinOperation(o) || (c.Origin != JoinOperationClientKey && c.Origin != JoinOperationServerNoKey) || c.PolicyVersion <= 0 || !validJoinDecisionDigest(c.PolicySHA256) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	invite := o.Method == spacev1.SpaceService_JoinByInvite_FullMethodName
	if (invite && (c.InviteID == nil || *c.InviteID == uuid.Nil)) || (!invite && c.InviteID != nil) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	out := []byte("voice.space.join.legacy-selection-context.v1\x00")
	out = binary.BigEndian.AppendUint32(out, 1)
	out = binary.BigEndian.AppendUint32(out, uint32(len(o.Method)))
	out = append(out, o.Method...)
	out = append(out, o.RequestSHA256...)
	out = append(out, o.ActorAccountID[:]...)
	out = append(out, o.ActorProfileID[:]...)
	out = append(out, o.OperationID[:]...)
	out = append(out, o.SpaceID[:]...)
	out = append(out, c.Origin)
	if c.InviteID == nil {
		out = append(out, 0)
	} else {
		out = append(out, 1)
		out = append(out, c.InviteID[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(c.PolicyVersion))
	out = append(out, c.PolicySHA256...)
	out = append(out, 0) // A is explicitly absent at pre-allocation selection.
	return sha256.Sum256(out), nil
}

// Keep legacy methods out of the generic v1 operation constructor.
func validLegacyJoinOperation(o JoinOperationInput) bool {
	return o.SpaceID != uuid.Nil && o.ActorAccountID != uuid.Nil && o.ActorProfileID != uuid.Nil && o.OperationID != uuid.Nil && validJoinDecisionDigest(o.RequestSHA256) && (o.Method == spacev1.SpaceService_JoinSpace_FullMethodName || o.Method == spacev1.SpaceService_JoinByInvite_FullMethodName)
}
