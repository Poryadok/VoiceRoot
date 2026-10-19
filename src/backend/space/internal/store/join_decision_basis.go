package store

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	"github.com/google/uuid"
)

var ErrJoinDecisionInvalid = errors.New("invalid join decision basis")

const (
	JoinDecisionSubmitPending uint8 = 1
	JoinDecisionSubmitReady   uint8 = 2
	JoinDecisionApproveReady  uint8 = 3
)

// JoinDecisionContext is historical binding data, never a current permission.
// Its binary encoding is frozen by SPACE-JOIN-DECISION-BASIS-v1.
type JoinDecisionContext struct {
	Kind                 uint8     `json:"kind"`
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
	SubmitBasisSHA256    []byte    `json:"submit_basis_sha256"`
}

type JoinDecisionBasis struct {
	Context                    JoinDecisionContext `json:"context"`
	ApplicantObservationSHA256 []byte              `json:"applicant_observation_sha256"`
	ReviewerObservationSHA256  []byte              `json:"reviewer_observation_sha256"`
}

func validJoinDecisionDigest(value []byte) bool {
	if len(value) != sha256.Size {
		return false
	}
	var nonzero byte
	for _, part := range value {
		nonzero |= part
	}
	return nonzero != 0
}

func appendJoinOptionalDigest(out, value []byte) []byte {
	if value == nil {
		return append(out, 0)
	}
	out = append(out, 1)
	return append(out, value...)
}

func (c JoinDecisionContext) canonicalBytes() ([]byte, error) {
	if c.SpaceID == uuid.Nil || c.AttemptID == uuid.Nil || c.ActorAccountID == uuid.Nil || c.ActorProfileID == uuid.Nil || c.OperationID == uuid.Nil || !validJoinDecisionDigest(c.AttemptBindingSHA256) || !validJoinDecisionDigest(c.SubmissionSHA256) || !validJoinDecisionDigest(c.RequestSHA256) {
		return nil, ErrJoinDecisionInvalid
	}
	switch c.Kind {
	case JoinDecisionSubmitPending:
		if c.RequestMethod != JoinOperationSubmit || c.ReadyEvidenceSHA256 != nil || c.SubmitBasisSHA256 != nil {
			return nil, ErrJoinDecisionInvalid
		}
	case JoinDecisionSubmitReady:
		if c.RequestMethod != JoinOperationSubmit || !validJoinDecisionDigest(c.ReadyEvidenceSHA256) || c.SubmitBasisSHA256 != nil {
			return nil, ErrJoinDecisionInvalid
		}
	case JoinDecisionApproveReady:
		if c.RequestMethod != JoinOperationApprove || !validJoinDecisionDigest(c.ReadyEvidenceSHA256) || !validJoinDecisionDigest(c.SubmitBasisSHA256) {
			return nil, ErrJoinDecisionInvalid
		}
	default:
		return nil, ErrJoinDecisionInvalid
	}
	out := []byte{c.Kind}
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
	out = appendJoinOptionalDigest(out, c.SubmitBasisSHA256)
	return out, nil
}

func (c JoinDecisionContext) SHA256() ([32]byte, error) {
	raw, err := c.canonicalBytes()
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(append([]byte("voice.space.join.decision-context.v1\x00"), raw...)), nil
}

func (b JoinDecisionBasis) SHA256() ([32]byte, error) {
	raw, err := b.Context.canonicalBytes()
	if err != nil || !validJoinDecisionDigest(b.ApplicantObservationSHA256) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	if b.Context.Kind == JoinDecisionApproveReady {
		if !validJoinDecisionDigest(b.ReviewerObservationSHA256) {
			return [32]byte{}, ErrJoinDecisionInvalid
		}
	} else if b.ReviewerObservationSHA256 != nil {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	out := append([]byte("voice.space.join.decision-basis.v1\x00"), raw...)
	out = append(out, b.ApplicantObservationSHA256...)
	out = appendJoinOptionalDigest(out, b.ReviewerObservationSHA256)
	return sha256.Sum256(out), nil
}

// JoinFirstPublishContextSHA256 binds a fresh observation to one exact durable
// native intent. Computing this value does not establish that intent's authority.
func JoinFirstPublishContextSHA256(basis []byte, publication uuid.UUID, generation uint64, nativeBinding []byte) ([32]byte, error) {
	if !validJoinDecisionDigest(basis) || publication == uuid.Nil || generation == 0 || generation > math.MaxInt64 || !validJoinDecisionDigest(nativeBinding) {
		return [32]byte{}, ErrJoinDecisionInvalid
	}
	out := append([]byte("voice.space.join.first-publish-context.v1\x00"), basis...)
	out = append(out, publication[:]...)
	out = binary.BigEndian.AppendUint64(out, generation)
	out = append(out, nativeBinding...)
	return sha256.Sum256(out), nil
}
