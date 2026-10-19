package store

import (
	"bytes"
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PrepareJoinDecisionAdmission resumes the one exact persisted decision/P.
// It is internal coordinator data, not caller authentication or membership.
func (s *SpaceStore) PrepareJoinDecisionAdmission(ctx context.Context, basisSHA []byte) (*MemberAdmissionIntent, error) {
	in, err := s.loadReadyJoinDecisionInput(ctx, basisSHA)
	if err != nil {
		return nil, err
	}
	if in.OperationID != uuid.Nil {
		return s.lookupJoinDecisionAdmission(ctx, in)
	}
	// P is server-owned, independent of the public operation namespace. Native
	// Prepare atomically pins35+36 and inserts its head/intent in the same TX.
	in.OperationID = uuid.New()
	intent, err := s.PrepareMemberAdmission(ctx, in)
	if !errors.Is(err, ErrMemberAdmissionConflict) && !errors.Is(err, ErrJoinAttemptConflict) {
		return intent, err
	}
	// Another exact caller may have won the same A while this candidate waited
	// for the Space fence. Reload this basis's saved P; never choose another A,
	// invite, latest operation, or steal a PREPARING candidate.
	winner, lookupErr := s.loadReadyJoinDecisionInput(ctx, basisSHA)
	if lookupErr != nil || winner.OperationID == uuid.Nil {
		return nil, err
	}
	return s.lookupJoinDecisionAdmission(ctx, winner)
}

func (s *SpaceStore) lookupJoinDecisionAdmission(ctx context.Context, expected MemberAdmissionInput) (*MemberAdmissionIntent, error) {
	i, err := s.LookupMemberAdmission(ctx, expected.OperationID)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if admissionRequestHash(i.Input) != admissionRequestHash(expected) {
		return nil, ErrMemberAdmissionConflict
	}
	return i, nil
}

// This immutable historical read grants nothing. Only native Prepare performs
// current fenced local validation; FIRST publication still requires its sealed
// owner/Role continuation. Published replay keeps its original exact P.
func (s *SpaceStore) loadReadyJoinDecisionInput(ctx context.Context, sha []byte) (MemberAdmissionInput, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || !validJoinDecisionDigest(sha) {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MemberAdmissionInput{}, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	local := *s
	local.tx = tx
	var version int16
	// Old v1 rows predate the additive tag. The owning row, never a caller,
	// chooses the decoder; a v2 body is never reinterpreted as v1.
	if err = tx.QueryRow(ctx, `SELECT COALESCE((to_jsonb(space_join_operations)->>'basis_version')::smallint,1) FROM space_join_operations WHERE decision_basis_sha256=$1`, sha).Scan(&version); err != nil {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	if version == 2 {
		in, err := local.loadReadyLegacyJoinDecisionInput(ctx, sha)
		if err != nil {
			return MemberAdmissionInput{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return MemberAdmissionInput{}, err
		}
		return in, nil
	}
	if version != 1 {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	saved, err := local.loadJoinDecision(ctx, sha, false)
	if err != nil {
		return MemberAdmissionInput{}, err
	}
	c := saved.basis.Context
	if c.Kind == JoinDecisionSubmitPending {
		return MemberAdmissionInput{}, ErrJoinAttemptPending
	}
	if c.Kind != JoinDecisionSubmitReady && c.Kind != JoinDecisionApproveReady {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, c.AttemptID))
	if err != nil {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	if a.Input.SpaceID != c.SpaceID || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) {
		return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
	}
	if c.Kind == JoinDecisionSubmitReady {
		if c.ActorAccountID != a.Input.AccountID || c.ActorProfileID != a.Input.ProfileID || a.approvedAccount != nil || a.approvedProfile != nil || a.approvalOperation != nil {
			return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
		}
	} else {
		if a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || *a.approvedAccount != c.ActorAccountID || *a.approvedProfile != c.ActorProfileID || *a.approvalOperation != c.OperationID {
			return MemberAdmissionInput{}, ErrJoinAttemptUnavailable
		}
		parent, err := local.loadJoinDecision(ctx, c.SubmitBasisSHA256, false)
		if err != nil {
			return MemberAdmissionInput{}, err
		}
		p := parent.basis.Context
		if parent.publication != nil || p.Kind != JoinDecisionSubmitPending || p.SpaceID != c.SpaceID || p.AttemptID != c.AttemptID || p.ActorAccountID != a.Input.AccountID || p.ActorProfileID != a.Input.ProfileID || !bytes.Equal(p.AttemptBindingSHA256, c.AttemptBindingSHA256) || !bytes.Equal(p.SubmissionSHA256, c.SubmissionSHA256) {
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
	if err = tx.Commit(ctx); err != nil {
		return MemberAdmissionInput{}, err
	}
	return in, nil
}
