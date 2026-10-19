package store

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
)

const (
	JoinLegacyPending        uint8 = 4
	JoinLegacyReady          uint8 = 5
	JoinApproveLegacyReady   uint8 = 6
	JoinOperationClientKey   uint8 = 1
	JoinOperationServerNoKey uint8 = 2
)

// JoinLegacyDecisionContext records the actual protected legacy method. It is
// separate from the frozen v1 codec and never establishes a current permission.
type JoinLegacyDecisionContext struct {
	Kind                 uint8     `json:"kind"`
	OperationOrigin      uint8     `json:"operation_origin"`
	SpaceID              uuid.UUID `json:"space_id"`
	AttemptID            uuid.UUID `json:"attempt_id"`
	AttemptBindingSHA256 []byte    `json:"attempt_binding_sha256"`
	SubmissionSHA256     []byte    `json:"submission_sha256"`
	ReadyEvidenceSHA256  []byte    `json:"ready_evidence_sha256"`
	ActorAccountID       uuid.UUID `json:"actor_account_id"`
	ActorProfileID       uuid.UUID `json:"actor_profile_id"`
	OperationID          uuid.UUID `json:"operation_id"`
	RequestMethod        string    `json:"request_method"`
	RequestSHA256        []byte    `json:"request_sha256"`
	PendingBasisSHA256   []byte    `json:"pending_basis_sha256"`
}

type JoinLegacyDecisionBasis struct {
	Context                    JoinLegacyDecisionContext `json:"context"`
	ApplicantObservationSHA256 []byte                    `json:"applicant_observation_sha256"`
	ReviewerObservationSHA256  []byte                    `json:"reviewer_observation_sha256"`
}

func (c JoinLegacyDecisionContext) canonicalBytes() ([]byte, error) {
	if c.SpaceID == uuid.Nil || c.AttemptID == uuid.Nil || c.ActorAccountID == uuid.Nil || c.ActorProfileID == uuid.Nil || c.OperationID == uuid.Nil || !validJoinDecisionDigest(c.AttemptBindingSHA256) || !validJoinDecisionDigest(c.SubmissionSHA256) || !validJoinDecisionDigest(c.RequestSHA256) {
		return nil, ErrJoinDecisionInvalid
	}
	if c.OperationOrigin != JoinOperationClientKey && c.OperationOrigin != JoinOperationServerNoKey {
		return nil, ErrJoinDecisionInvalid
	}
	switch c.Kind {
	case JoinLegacyPending, JoinLegacyReady:
		if c.RequestMethod != spacev1.SpaceService_JoinSpace_FullMethodName && c.RequestMethod != spacev1.SpaceService_JoinByInvite_FullMethodName {
			return nil, ErrJoinDecisionInvalid
		}
		if c.PendingBasisSHA256 != nil || (c.Kind == JoinLegacyPending && c.ReadyEvidenceSHA256 != nil) || (c.Kind == JoinLegacyReady && !validJoinDecisionDigest(c.ReadyEvidenceSHA256)) {
			return nil, ErrJoinDecisionInvalid
		}
	case JoinApproveLegacyReady:
		if c.RequestMethod != JoinOperationApprove || c.OperationOrigin != JoinOperationClientKey || !validJoinDecisionDigest(c.ReadyEvidenceSHA256) || !validJoinDecisionDigest(c.PendingBasisSHA256) {
			return nil, ErrJoinDecisionInvalid
		}
	default:
		return nil, ErrJoinDecisionInvalid
	}
	out := []byte{c.Kind, c.OperationOrigin}
	out = append(out, c.SpaceID[:]...)
	out = append(out, c.AttemptID[:]...)
	out = append(out, c.AttemptBindingSHA256...)
	out = append(out, c.SubmissionSHA256...)
	out = appendJoinOptionalDigest(out, c.ReadyEvidenceSHA256)
	out = append(out, c.ActorAccountID[:]...)
	out = append(out, c.ActorProfileID[:]...)
	out = append(out, c.OperationID[:]...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(c.RequestMethod)))
	out = append(out, c.RequestMethod...)
	out = append(out, c.RequestSHA256...)
	out = appendJoinOptionalDigest(out, c.PendingBasisSHA256)
	return out, nil
}

func (c JoinLegacyDecisionContext) SHA256() ([32]byte, error) {
	raw, err := c.canonicalBytes()
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(append([]byte("voice.space.join.decision-context.v2\x00"), raw...)), nil
}
func (b JoinLegacyDecisionBasis) SHA256() ([32]byte, error) {
	raw, err := b.Context.canonicalBytes()
	if err != nil || !validJoinDecisionDigest(b.ApplicantObservationSHA256) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	if b.Context.Kind == JoinApproveLegacyReady {
		if !validJoinDecisionDigest(b.ReviewerObservationSHA256) {
			return [32]byte{}, ErrJoinDecisionInvalid
		}
	} else if b.ReviewerObservationSHA256 != nil {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	out := append([]byte("voice.space.join.decision-basis.v2\x00"), raw...)
	out = append(out, b.ApplicantObservationSHA256...)
	out = appendJoinOptionalDigest(out, b.ReviewerObservationSHA256)
	return sha256.Sum256(out), nil
}
