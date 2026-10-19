package store

import (
	"bytes"
	"context"
	"slices"
)

// This read is historical coordinator input, never current authorization. Its
// transaction shares one read-only snapshot with the exact family tag lookup.
// Native Prepare/FIRST Publish still perform all current fenced checks.
func (s *SpaceStore) loadReadyLegacyJoinDecisionInput(ctx context.Context, sha []byte) (MemberAdmissionInput, error) {
	if s == nil || s.tx == nil {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	saved, err := s.loadLegacyJoinDecision(ctx, sha, false)
	if err != nil {
		return MemberAdmissionInput{}, err
	}
	c := saved.basis.Context
	if c.Kind == JoinLegacyPending {
		return MemberAdmissionInput{}, ErrJoinAttemptPending
	}
	if c.Kind != JoinLegacyReady && c.Kind != JoinApproveLegacyReady {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, c.AttemptID))
	if err != nil || a.Input.SpaceID != c.SpaceID || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	if c.Kind == JoinLegacyReady {
		if c.ActorAccountID != a.Input.AccountID || c.ActorProfileID != a.Input.ProfileID || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil {
			return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
		}
	} else {
		if a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != c.ActorAccountID || *a.approvedProfile != c.ActorProfileID || *a.approvalOperation != c.OperationID {
			return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
		}
		parent, err := s.loadLegacyJoinDecision(ctx, c.PendingBasisSHA256, false)
		if err != nil || !legacyOriginalParentMatches(c, parent) || parent.basis.Context.ActorAccountID != a.Input.AccountID || parent.basis.Context.ActorProfileID != a.Input.ProfileID {
			return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
		}
	}
	in := MemberAdmissionInput{SpaceID: a.Input.SpaceID, AccountID: a.Input.AccountID, ProfileID: a.Input.ProfileID, Guest: a.Input.Guest, JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: slices.Clone(c.ReadyEvidenceSHA256)}, DecisionBasisSHA256: slices.Clone(sha)}
	if a.Input.InviteID != nil {
		invite := *a.Input.InviteID
		in.InviteID = &invite
	}
	if saved.publication == nil {
		if a.publication != nil {
			return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
		}
		if a.State != "READY" {
			return MemberAdmissionInput{}, ErrJoinAttemptDenied
		}
	} else {
		if a.publication == nil || *a.publication != *saved.publication || saved.publicationState == nil || (*saved.publicationState != "PREPARING" && *saved.publicationState != "PUBLISHED") || ((*saved.publicationState == "PUBLISHED") != (a.State == "PUBLISHED")) {
			return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
		}
		in.OperationID = *saved.publication
	}
	return in, nil
}
