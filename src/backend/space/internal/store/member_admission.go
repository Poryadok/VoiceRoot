package store

import (
	"context"

	"github.com/google/uuid"
	"voice/backend/pkg/memberadmission"
)

// MemberAdmissionInput identifies one server-owned admission attempt. Remote
// proof verification precedes this API; unsupported interactive policy is denied.
type MemberAdmissionInput struct {
	OperationID, SpaceID, ProfileID, AccountID uuid.UUID
	InviteID                                   *uuid.UUID
	Guest                                      bool
}

type MemberAdmissionIntent struct {
	Input         MemberAdmissionInput
	Generation    uint64
	BindingSHA256 []byte
	PolicyVersion int64
	State         string
}

func (i *MemberAdmissionIntent) EnsureBinding() memberadmission.EnsureBinding {
	return memberadmission.EnsureBinding{ProtocolVersion: 1, SpaceID: i.Input.SpaceID.String(),
		ProfileID: i.Input.ProfileID.String(), AdmissionOperationID: i.Input.OperationID.String(),
		MembershipGeneration: i.Generation, AdmissionBindingSHA256: append([]byte(nil), i.BindingSHA256...)}
}

// MemberAdmissionFinalGuard may access only the provided scoped Space store.
// Callers collect external authority before entering publication; network I/O
// under this callback would violate the protocol's lock ordering.
type MemberAdmissionFinalGuard func(context.Context, *SpaceStore, *MemberAdmissionIntent) error
