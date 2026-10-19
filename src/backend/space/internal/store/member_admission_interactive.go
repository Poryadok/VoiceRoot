package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Implemented by the Space36 owner. Missing source is unavailable, never a
// weaker historical proof or a caller-provided success callback.
type joinDecisionBasisVerifier interface {
	VerifyJoinDecisionBasis(context.Context, JoinAttemptInput, JoinAttemptReference, []byte, int64, uuid.UUID) error
}

func nonzeroAdmissionDigest(value []byte) bool {
	return len(value) == sha256.Size && !bytes.Equal(value, make([]byte, sha256.Size))
}

func validateMemberAdmissionReference(in MemberAdmissionInput) error {
	if in.JoinAttempt == nil {
		if len(in.DecisionBasisSHA256) != 0 {
			return ErrMemberAdmissionConflict
		}
		return nil
	}
	if in.JoinAttempt.AttemptID == uuid.Nil || !nonzeroAdmissionDigest(in.JoinAttempt.EvidenceSHA256) || !nonzeroAdmissionDigest(in.DecisionBasisSHA256) {
		return ErrMemberAdmissionConflict
	}
	return nil
}

func cloneMemberAdmissionInput(in MemberAdmissionInput) MemberAdmissionInput {
	if in.InviteID != nil {
		invite := *in.InviteID
		in.InviteID = &invite
	}
	if in.JoinAttempt != nil {
		in.JoinAttempt = &JoinAttemptReference{AttemptID: in.JoinAttempt.AttemptID, EvidenceSHA256: append([]byte(nil), in.JoinAttempt.EvidenceSHA256...)}
	}
	in.DecisionBasisSHA256 = append([]byte(nil), in.DecisionBasisSHA256...)
	return in
}

func (s *SpaceStore) verifyMemberAdmissionReference(ctx context.Context, intent *MemberAdmissionIntent) error {
	in := intent.Input
	if err := validateMemberAdmissionReference(in); err != nil {
		return err
	}
	if in.JoinAttempt == nil {
		return s.requireNonInteractiveAdmission(ctx, in.SpaceID)
	}
	verifier, ok := any(s).(joinDecisionBasisVerifier)
	if !ok || s.tx == nil {
		return ErrJoinAttemptUnavailable
	}
	selected := JoinAttemptInput{SpaceID: in.SpaceID, AccountID: in.AccountID, ProfileID: in.ProfileID, InviteID: in.InviteID, Guest: in.Guest}
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return err
	}
	if version == 40 || version == 41 || version == 42 || version == 44 || version == 45 || (version == 46 || version == 47) {
		if err := s.VerifyVersionedJoinDecisionBasis(ctx, selected, *in.JoinAttempt, in.DecisionBasisSHA256, intent.PolicyVersion, in.OperationID); err != nil {
			return err
		}
		return s.VerifyJoinAttemptAdmission(ctx, selected, *in.JoinAttempt, intent.PolicyVersion, in.OperationID)
	}
	if err := verifier.VerifyJoinDecisionBasis(ctx, selected, *in.JoinAttempt, in.DecisionBasisSHA256, intent.PolicyVersion, in.OperationID); err != nil {
		return err
	}
	return s.VerifyJoinAttemptAdmission(ctx, selected, *in.JoinAttempt, intent.PolicyVersion, in.OperationID)
}

// FirstPublicationContextSHA256 is a request binding, not an authority grant.
// FIRST_PUBLISH owner observations must use it instead of a saved SUBMIT or
// APPROVE context, and must be freshly checked after local SQL waits.
func (i *MemberAdmissionIntent) FirstPublicationContextSHA256() ([32]byte, error) {
	if i == nil || i.Input.JoinAttempt == nil || validateMemberAdmissionReference(i.Input) != nil || i.Input.OperationID == uuid.Nil || i.Generation == 0 || i.Generation > math.MaxInt64 || !nonzeroAdmissionDigest(i.BindingSHA256) {
		return [32]byte{}, ErrMemberAdmissionConflict
	}
	framed := []byte("voice.space.join.first-publish-context.v1\x00")
	framed = append(framed, i.Input.DecisionBasisSHA256...)
	framed = append(framed, i.Input.OperationID[:]...)
	framed = binary.BigEndian.AppendUint64(framed, i.Generation)
	framed = append(framed, i.BindingSHA256...)
	return sha256.Sum256(framed), nil
}

// AssertJoinAttemptPublicationRetired is scoped local evidence for cancelling
// a CLAIMED A. State alone is insufficient: the exact A/evidence/P and permanent
// ordinary floor must prove that this candidate can never publish again.
func (s *SpaceStore) AssertJoinAttemptPublicationRetired(ctx context.Context, ref JoinAttemptReference, publication uuid.UUID) error {
	if s == nil || s.tx == nil || publication == uuid.Nil || ref.AttemptID == uuid.Nil || !nonzeroAdmissionDigest(ref.EvidenceSHA256) {
		return ErrJoinAttemptUnavailable
	}
	i, savedRequest, err := scanMemberAdmission(s.db().QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents WHERE operation_id=$1`, publication))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrJoinAttemptUnavailable
	}
	if err != nil {
		return err
	}
	request := admissionRequestHash(i.Input)
	if i.State != "RETIRED" || i.Input.JoinAttempt == nil || validateMemberAdmissionReference(i.Input) != nil || i.Input.JoinAttempt.AttemptID != ref.AttemptID || !bytes.Equal(i.Input.JoinAttempt.EvidenceSHA256, ref.EvidenceSHA256) || !bytes.Equal(savedRequest, request[:]) {
		return ErrJoinAttemptUnavailable
	}
	var retired bool
	err = s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 AND retired_through>=$3)`, i.Input.SpaceID, i.Input.ProfileID, i.Generation).Scan(&retired)
	if err != nil {
		return err
	}
	if !retired {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
